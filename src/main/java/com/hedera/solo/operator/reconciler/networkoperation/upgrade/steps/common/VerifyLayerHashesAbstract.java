// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static java.util.Collections.emptyList;

import com.hedera.solo.operator.library.container.DistributionRegistryClient;
import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.container.ManifestResponse;
import com.hedera.solo.operator.library.container.UnexpectedResponseException;
import com.hedera.solo.operator.library.container.oci.OCIContentDescriptor;
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.HasMetadata;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.stream.IntStream;

/**
 * Abstract class to verify layer hashes for a given repository and specifications.
 *
 * @param <Spec> the type of specifications used for verification
 * @param <Status> the type of status returned after verification
 * @param <Repository> the type of repository containing the image
 */
public abstract class VerifyLayerHashesAbstract<Spec, Status, Repository> {
    /**
     * Get the image coordinates for the specified repository.
     *
     * @param repository the repository containing the image
     * @return ImageCoordinates object representing the image coordinates
     */
    protected abstract ImageCoordinates coordinates(final Repository repository);

    /**
     * Retrieve a list of repositories to check.
     *
     * @return List of repositories
     */
    protected abstract List<Repository> repositories();

    /**
     * Get the specifications for verification.
     *
     * @return List of specifications
     */
    protected abstract List<Spec> specs();

    /**
     * Create a status object with an exception for the specified repository and exception.
     *
     * @param repository the repository where the exception occurred
     * @param e the exception to be included in the status
     * @return Status object representing the exception status
     */
    protected abstract Status exception(final Repository repository, final Exception e);

    /**
     * Create a status object indicating an invalid response for the specified repository and manifest response.
     *
     * @param repository the repository where the invalid response occurred
     * @param response the ManifestResponse containing the invalid response
     * @return Status object representing the invalid response status
     */
    protected abstract Status invalid(final Repository repository, final ManifestResponse response);

    /**
     * Create a status object indicating an invalid response for the specified repository and digest.
     *
     * @param repository the repository where the invalid response occurred
     * @param digest the digest of the image that caused the invalid response
     * @return Status object representing the invalid response status
     */
    protected abstract Status invalid(final Repository repository, final String digest);

    /**
     * Create a status object indicating an error with the specified status code for the repository.
     *
     * @param repository the repository where the status code occurred
     * @param statusCode the HTTP status code that caused the error
     * @return Status object representing the status code error
     */
    protected abstract Status statusCode(final Repository repository, final int statusCode);

    /**
     * Create a status object indicating a valid response for the specified repository and specification.
     *
     * @param repository the repository where the valid response occurred
     * @param spec the specification that matches the image
     * @param digest the digest of the image that matched the specification
     * @return Status object representing the valid response status
     */
    protected abstract Status valid(final Repository repository, final Spec spec, final String digest);

    /**
     * Check if a given specification matches an OCIImageIndexManifest.
     *
     * @param spec the specification to match against the manifest
     * @param manifest the OCIImageIndexManifest to compare with
     * @return true if the specification matches the manifest, false otherwise
     */
    protected abstract boolean match(final Spec spec, final OCIImageIndex.OCIImageIndexManifest manifest);

    /**
     * Check if a given specification matches an OCIImageManifest.
     *
     * @param spec the specification to match against the manifest
     * @param manifest the OCIImageManifest to compare with
     * @return true if the specification matches the manifest, false otherwise
     */
    protected abstract boolean match(final Spec spec, final OCIImageManifest manifest);

    /**
     * Generate a list of status objects for all repositories.
     *
     * @return List of Status objects representing the check results
     */
    public List<Status> statusList() {
        List<Status> list = new ArrayList<>();
        try (DistributionRegistryClient client = new DistributionRegistryClient()) {
            for (Repository repository : repositories()) {
                try {
                    list.addAll(check(new ArrayList<>(), client, repository, coordinates(repository), null));
                } catch (UnexpectedResponseException e) {
                    list.add(statusCode(repository, e.response.statusCode()));
                } catch (Exception e) {
                    list.add(exception(repository, e));
                }
            }
        }
        return list;
    }

    /**
     * Update the list of conditions with a new condition.
     *
     * @param resource the HasMetadata object to update
     * @param conditions the current list of conditions (can be null)
     * @param newCondition the new Condition to add or update in the list
     * @return Updated list of Conditions
     */
    public List<Condition> conditions(
            final HasMetadata resource, final List<Condition> conditions, final Condition newCondition) {
        List<Condition> list = new ArrayList<>(conditions == null ? emptyList() : conditions);
        int index = IntStream.range(0, list.size())
                .filter(i -> Objects.equals(list.get(i).getType(), newCondition.getType())
                        && Objects.equals(list.get(i).getReason(), newCondition.getReason()))
                .findFirst()
                .orElse(-1);
        if (index != -1) {
            Condition oldCondition = list.get(index);
            newCondition.setObservedGeneration(oldCondition.getObservedGeneration());
            newCondition.setLastTransitionTime(oldCondition.getLastTransitionTime());
            if (Objects.equals(oldCondition, newCondition)) {
                oldCondition.setObservedGeneration(resource.getMetadata().getGeneration());
                return list;
            } else {
                list.set(index, newCondition);
            }
        } else {
            list.add(newCondition);
        }
        newCondition.setObservedGeneration(resource.getMetadata().getGeneration());
        newCondition.setLastTransitionTime(timestamp());
        return list;
    }

    /**
     * Check the repository for valid or invalid image layers and update the status list accordingly.
     *
     * @param list the current list of Status objects to be updated
     * @param client the DistributionRegistryClient used to fetch manifests
     * @param repository the repository containing the image
     * @param coordinates the ImageCoordinates for the image
     * @param specific the specific specification to match against (can be null)
     * @param mediaTypes the media types to request when fetching the manifest
     * @return Updated list of Status objects
     * @throws IOException if an I/O error occurs while fetching the manifest
     * @throws InterruptedException if the thread is interrupted while waiting for the manifest
     */
    protected List<Status> check(
            final List<Status> list,
            final DistributionRegistryClient client,
            final Repository repository,
            final ImageCoordinates coordinates,
            final Spec specific,
            final String... mediaTypes)
            throws IOException, InterruptedException {
        ManifestResponse response = client.fetchManifest(coordinates, mediaTypes);
        if (response.manifest instanceof OCIImageIndex index) {
            for (OCIImageIndex.OCIImageIndexManifest m : index.manifests) {
                for (Spec spec : specs()) {
                    if (match(spec, m)) {
                        check(list, client, repository, coordinates.digest(m.digest), spec, m.mediaType);
                    }
                }
            }
        } else if (response.manifest instanceof OCIImageManifest manifest) {
            for (Spec spec : specific != null ? List.of(specific) : specs()) {
                if (match(spec, manifest)) {
                    boolean allExist = true;
                    for (OCIContentDescriptor layer : manifest.layers) {
                        allExist = allExist && client.layerExists(coordinates, layer);
                    }
                    if (allExist) {
                        list.add(valid(repository, spec, response.digest));
                        return list;
                    }
                }
            }
            list.add(invalid(repository, response.digest));
        } else {
            list.add(invalid(repository, response));
        }
        return list;
    }
}
