// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

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

public abstract class VerifyLayerHashesAbstract<Spec, Status, Repository> {
    protected abstract ImageCoordinates coordinates(final Repository repository);

    protected abstract List<Repository> repositories();

    protected abstract List<Spec> specs();

    protected abstract Status exception(final Repository repository, final Exception e);

    protected abstract Status invalid(final Repository repository, final ManifestResponse response);

    protected abstract Status invalid(final Repository repository, final String digest);

    protected abstract Status statusCode(final Repository repository, final int statusCode);

    protected abstract Status valid(final Repository repository, final String digest);

    protected abstract boolean match(final Spec spec, final OCIImageIndex.OCIImageIndexManifest manifest);

    protected abstract boolean match(final Spec spec, final OCIImageManifest manifest);

    /**
     * Generate status list
     *
     * @return status list
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
     * Update list of conditions with new condition
     *
     * @param resource with conditions
     * @param conditions list of current conditions
     * @param newCondition the new condition to include in the list
     * @return updated list of conditions
     */
    public List<Condition> conditions(
            final HasMetadata resource, final List<Condition> conditions, final Condition newCondition) {
        List<Condition> list = new ArrayList<>(conditions == null ? emptyList() : conditions);
        int index = IntStream.range(0, list.size())
                .filter(i -> Objects.equals(list.get(i).getType(), newCondition.getType()))
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
                        list.add(valid(repository, response.digest));
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
