// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

import static java.net.HttpURLConnection.HTTP_NOT_FOUND;

import com.hedera.solo.operator.library.container.DistributionRegistryClient;
import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.container.UnexpectedResponseException;
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import com.hedera.solo.operator.library.container.oci.OCIManifest;
import edu.umd.cs.findbugs.annotations.NonNull;
import java.io.IOException;
import java.net.http.HttpResponse;
import java.util.List;
import java.util.Objects;

/**
 * Image registry client to fetch image index and manifest from registry.
 *
 * <p>It extends the capability of {@link DistributionRegistryClient} with more specific methods to fetch image index
 * and manifest
 *
 * <p>Intention of this class is to provide the following functionalities:
 *
 * <ol>
 *   <li>Fetch image index corresponding to image coordinates.
 *   <li>Fetch image manifest corresponding to image coordinates, os and architecture.
 *   <li>Fetch image manifest corresponding to image coordinates and platform.
 *   <li>Fetch image manifest using image index, image coordinates and platform.
 *   <li>Fetch image using image coordinates and endpoint.
 *   <li>Handle exceptions while fetching record from registry.
 *   <li>Provide a functionality to match and verify the desired layers exist in the repository
 * </ol>
 */
public class ImageRegistry extends DistributionRegistryClient {

    /**
     * Fetch image index corresponding to image coordinates.
     *
     * @param coordinates to image
     * @return OCIImageIndex representing the image index for the given coordinates
     */
    public OCIImageIndex fetchImageIndex(@NonNull final ImageCoordinates coordinates) {
        Objects.requireNonNull(coordinates, "coordinates cannot be null");
        final HttpResponse<byte[]> resp =
                fetch(coordinates, "manifests/" + coordinates.reference(), OCIImageIndex.mediaTypes());
        final String mediaType = resp.headers().firstValue("Content-Type").orElse(null);
        OCIManifest manifest = OCIManifest.from(mediaType, resp.body());
        return manifest instanceof OCIImageIndex ? (OCIImageIndex) manifest : null;
    }

    /**
     * Fetch image manifest corresponding to image coordinates, os and architecture.
     *
     * @param coordinates to image
     * @param os os type.
     * @param architecture architecture type.
     * @return OCIImageManifest representing the image manifest for the given coordinates and platform
     */
    public OCIImageManifest fetchImageManifest(
            @NonNull final ImageCoordinates coordinates, @NonNull final String os, @NonNull final String architecture) {
        Objects.requireNonNull(coordinates, "coordinates cannot be null");
        Objects.requireNonNull(os, "os cannot be null");
        Objects.requireNonNull(architecture, "architecture cannot be null");

        OCIImageIndex.OCIImageIndexPlatform platform = new OCIImageIndex.OCIImageIndexPlatform(os, architecture);
        return fetchImageManifest(coordinates, platform);
    }

    /**
     * Fetch image manifest corresponding to image coordinates and platform.
     *
     * @param coordinates to image
     * @param platform platform info including os and architecture
     * @return OCIImageManifest representing the image manifest for the given coordinates and platform
     * @throws RecordNotFoundException if the manifest index is not found in the registry
     */
    public OCIImageManifest fetchImageManifest(
            @NonNull final ImageCoordinates coordinates, @NonNull final OCIImageIndex.OCIImageIndexPlatform platform) {
        OCIImageIndex resp = fetchImageIndex(coordinates);
        if (Objects.isNull(resp)) {
            throw new RecordNotFoundException("manifest index not found in registry");
        }

        return fetchImageManifest(resp, coordinates, platform);
    }

    /**
     * Fetch image manifest using image index, image coordinates and platform.
     *
     * @param index OCI image index
     * @param coordinates image coordinates
     * @param platform platform info including os and architecture
     * @return OCIImageManifest representing the image manifest for the given index, coordinates and platform
     * @throws RecordNotFoundException if the manifest is not found in the index
     */
    public OCIImageManifest fetchImageManifest(
            final OCIImageIndex index,
            final ImageCoordinates coordinates,
            final OCIImageIndex.OCIImageIndexPlatform platform) {
        Objects.requireNonNull(index, "index cannot be null");
        Objects.requireNonNull(coordinates, "coordinates cannot be null");
        Objects.requireNonNull(platform, "platform cannot be null");
        Objects.requireNonNull(platform.os, "os cannot be null");
        Objects.requireNonNull(platform.architecture, "architecture cannot be null");

        for (OCIImageIndex.OCIImageIndexManifest manifest : index.manifests) {
            if (platform.architecture.equals(manifest.platform.architecture)
                    && platform.os.equals(manifest.platform.os)) {
                final ImageCoordinates refCoordinates = coordinates.digest(manifest.digest);
                final HttpResponse<byte[]> resp =
                        fetch(refCoordinates, "manifests/" + refCoordinates.reference(), OCIImageManifest.mediaTypes());
                final String mediaType =
                        resp.headers().firstValue("Content-Type").orElse(null);
                return (OCIImageManifest) OCIImageManifest.from(mediaType, resp.body());
            }
        }

        throw new RecordNotFoundException("image manifest not found in registry");
    }

    /**
     * Fetches a resource from the image registry using the given coordinates, endpoint, and media types.
     *
     * @param coordinates the image coordinates to fetch from
     * @param endpoint the registry endpoint to query
     * @param mediaTypes the accepted media types for the response
     * @return the HTTP response containing the resource as a byte array
     * @throws ImageRegistryException if an I/O or unexpected response error occurs
     * @throws RecordNotFoundException if the resource is not found in the registry
     */
    public HttpResponse<byte[]> fetch(
            final ImageCoordinates coordinates, final String endpoint, final String... mediaTypes) {
        try {
            return get(coordinates, endpoint, mediaTypes);
        } catch (IOException | InterruptedException e) {
            throw new ImageRegistryException("Failed to fetch from image registry", e);
        } catch (UnexpectedResponseException e) {
            if (e.response.statusCode() == HTTP_NOT_FOUND) {
                throw new RecordNotFoundException("image not found in the registry", e);
            }

            throw new ImageRegistryException(
                    "Failed to fetch from image registry because of HTTP error: %s".formatted(e.response.statusCode()),
                    e);
        }
    }

    /**
     * Validate the manifest contains the expected image layer digests.
     *
     * @param coordinates image coordinates
     * @param manifest OCIImageManifest containing a list of layers
     * @param layerHashes list of layer hashes from the CR
     * @return true if the layers match the hashes in the same order and exist in the registry
     * @throws ImageRegistryException if an I/O error occurs while checking layer existence
     */
    public boolean hasValidLayers(ImageCoordinates coordinates, OCIImageManifest manifest, List<String> layerHashes) {
        if (layerHashes.size() != manifest.layers.length) {
            return false;
        }

        for (int i = 0; i < manifest.layers.length; i++) {
            if (!layerHashes.get(i).equals(manifest.layers[i].digest)) {
                return false;
            }
        }

        try {
            for (int i = 0; i < manifest.layers.length; i++) {
                if (!layerExists(coordinates, manifest.layers[i])) {
                    return false;
                }
            }
        } catch (IOException | InterruptedException e) {
            throw new ImageRegistryException("failed to check if layer exists", e);
        }

        return true;
    }
}
