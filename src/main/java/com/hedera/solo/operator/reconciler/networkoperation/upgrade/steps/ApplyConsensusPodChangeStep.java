// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.render.NowFrozenConsensusCapsules;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.api.model.ListOptionsBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.Listable;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.HashSet;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.function.Consumer;
import java.util.stream.Collectors;

/**
 * ApplyConsensusPodChangeStep applies the changeset to the consensus pods
 *
 * <p>It makes a single atomic patch to consensus-capsule CR using the following changesets. It will be checked by
 * {@link NetworkCheckProcessor} when deferred.
 *
 * <ol>
 *   <li>Apply consensus-node changeset (image source, nodeProperties and logging).
 *   <li>Apply streamUploader changesets (e.g., recordStreamUploader, backUploader, etc.).
 * </ol>
 *
 * <p>If this step is re-invoked, the changes in consensus-capsule CR should be idempotent.
 */
public class ApplyConsensusPodChangeStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        if (status == null) {
            status = new NetworkOperationStatus();
        }
        Set<Long> excludeNodes = new HashSet<>(resource.getStatus().getUpgradedNodes());
        Set<Long> includeNodes = resource.getSpec().getNetworkUpgrade().getAddressBook().stream()
                .map(AddressBookEntry::getNodeId)
                .collect(Collectors.toSet());
        try {
            KubernetesClient client = context.getClient();
            String namespace = resource.getMetadata().getNamespace();
            NowFrozenConsensusCapsules nowFrozen = new NowFrozenConsensusCapsules(resource, status);
            forEach(
                    client.resources(ConsensusCapsule.class).inNamespace(namespace),
                    cc -> update(client, nowFrozen, cc, includeNodes, excludeNodes));
            return status;
        } finally {
            List<Long> upgradedNodes = excludeNodes.stream().sorted().toList();
            resource.getStatus().setUpgradedNodes(upgradedNodes);
            context.getClient().resource(resource).patchStatus();
            status.setUpgradedNodes(upgradedNodes);
        }
    }

    <T extends HasMetadata> void forEach(Listable<KubernetesResourceList<T>> listable, Consumer<T> consumer) {
        KubernetesResourceList<T> list =
                listable.list(new ListOptionsBuilder().withLimit(10L).build());
        list.getItems().forEach(consumer);
        while (0
                < Optional.ofNullable(list.getMetadata().getRemainingItemCount())
                        .orElse(0L)) {
            list = listable.list(new ListOptionsBuilder()
                    .withLimit(10L)
                    .withContinue(list.getMetadata().getContinue())
                    .build());
            list.getItems().forEach(consumer);
        }
    }

    void update(
            final KubernetesClient client,
            final NowFrozenConsensusCapsules nowFrozen,
            final ConsensusCapsule capsule,
            final Set<Long> includeNodes,
            final Set<Long> excludeNodes) {
        long nodeId = capsule.getSpec().getNodeId();
        if (includeNodes.contains(nodeId) && !excludeNodes.contains(nodeId)) {
            client.resource(nowFrozen.update(capsule)).update();
            excludeNodes.add(nodeId);
        }
    }
}
