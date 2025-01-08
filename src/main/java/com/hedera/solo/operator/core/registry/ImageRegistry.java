// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

import com.hedera.solo.operator.library.container.DistributionRegistryClient;
import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import edu.umd.cs.findbugs.annotations.NonNull;
import java.net.http.HttpResponse;
import java.util.Objects;
import java.util.Optional;

public class ImageRegistry extends DistributionRegistryClient {

    /**
     * Fetch image index corresponding to image coordinates.
     *
     * @param coordinates to image
     */
    public Optional<OCIImageIndex> fetchImageIndex(@NonNull final ImageCoordinates coordinates) {
        Objects.requireNonNull(coordinates, "coordinates cannot be null");
        final HttpResponse<byte[]> resp =
                fetch(coordinates, "manifests/" + coordinates.reference, OCIImageIndex.mediaTypes());
        final String mediaType = resp.headers().firstValue("Content-Type").orElse(null);
        return Optional.ofNullable((OCIImageIndex) OCIImageIndex.from(mediaType, resp.body()));
    }

    /**
     * Fetch image manifest corresponding to image coordinates, os and architecture.
     *
     * @param coordinates to image
     * @param os os os type.
     * @param architecture architecture type.
     */
    public Optional<OCIImageManifest> fetchImageManifest(
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
     */
    public Optional<OCIImageManifest> fetchImageManifest(
            @NonNull final ImageCoordinates coordinates, @NonNull final OCIImageIndex.OCIImageIndexPlatform platform) {
        Optional<OCIImageIndex> resp = fetchImageIndex(coordinates);
        if (resp.isEmpty()) {
            return Optional.empty();
        }

        return fetchImageManifest(resp.get(), coordinates, platform);
    }

    /**
     * Fetch image manifest using image index, image coordinates and platform.
     *
     * @param index OCI image index
     * @param coordinates image coordinates
     * @param platform platform info including os and architecture
     */
    public Optional<OCIImageManifest> fetchImageManifest(
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
                final ImageCoordinates refCoordinates = coordinates.reference(manifest.digest);
                final HttpResponse<byte[]> resp =
                        fetch(refCoordinates, "manifests/" + refCoordinates.reference, OCIImageManifest.mediaTypes());
                final String mediaType =
                        resp.headers().firstValue("Content-Type").orElse(null);
                return Optional.ofNullable((OCIImageManifest) OCIImageManifest.from(mediaType, resp.body()));
            }
        }

        return Optional.empty();
    }

    public HttpResponse<byte[]> fetch(
            final ImageCoordinates coordinates, final String endpoint, final String... mediaTypes) {
        try {
            return get(coordinates, endpoint, mediaTypes);
        } catch (Exception e) {
            throw new ImageRegistryException("Failed to fetch from image registry", e);
        }
    }
}
