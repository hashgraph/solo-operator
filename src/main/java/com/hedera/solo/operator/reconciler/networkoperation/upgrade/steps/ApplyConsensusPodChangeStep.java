// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.render.NowFrozenConsensusCapsules;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.api.model.ListOptionsBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.Listable;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.time.Instant;
import java.util.HashSet;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.function.Consumer;
import java.util.stream.Collectors;
import org.jboss.logging.Logger;

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
    public static final String CONDITION_TYPE = "ConsensusCapsuleUpdated";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            return status;
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
        } catch (Exception ex) {
            List<Long> upgradedNodes = excludeNodes.stream().sorted().toList();
            log.errorf(
                    "Failed to update consensus capsules for network operation %s/%s: %s"
                            .formatted(
                                    resource.getMetadata().getNamespace(),
                                    resource.getMetadata().getName(),
                                    resource.getStatus().getConsensusCapsules()),
                    ex);

            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(Instant.now().toString())
                                    .withMessage("Failed to update consensus capsules: %s".formatted(ex.getMessage()))
                                    .build())
                            .withUpgradedNodes(upgradedNodes)
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }

        List<Long> upgradedNodes = excludeNodes.stream().sorted().toList();
        log.infof("List of updated node IDs: %s", upgradedNodes);
        return edit(status)
                .with(s -> s.addToConditions(new ConditionBuilder()
                                .withType(CONDITION_TYPE)
                                .withStatus(ConditionStatus.True)
                                .withLastTransitionTime(Instant.now().toString())
                                .withMessage("Successfully updated consensus capsules")
                                .build())
                        .withUpgradedNodes(upgradedNodes))
                .build();
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
            log.infof("Updating consensus capsule: %s", capsule.getMetadata().getName());
            client.resource(nowFrozen.update(capsule)).update();
            excludeNodes.add(nodeId);
            log.infof("List of updated consensus capsules: %s", excludeNodes);
        }
    }

    public static boolean isDone(NetworkOperationStatus status) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(CONDITION_TYPE) && c.getStatus().equals(ConditionStatus.True));
    }
}
