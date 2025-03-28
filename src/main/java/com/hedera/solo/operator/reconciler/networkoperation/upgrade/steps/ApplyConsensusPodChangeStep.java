// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.render.NowFrozenConsensusCapsules;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.api.model.ListOptionsBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.Listable;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
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
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully updated consensus capsules";
    public static final String FAIL_STATUS_MESSAGE = "Failed to update consensus capsules:'%s'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            final NetworkOperation resource,
            final NetworkOperationStatus status,
            final Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation is required");
        Objects.requireNonNull(resource.getSpec(), "NetworkOperationSpec is required");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "NetworkUpgrade is required");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade().getAddressBook(), "AddressBook is required");
        Objects.requireNonNull(status, "NetworkOperationStatus is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> is required");

        Set<Long> excludeNodes = new HashSet<>(resource.getStatus().getUpgradedNodes());
        Set<Long> includeNodes = resource.getSpec().getNetworkUpgrade().getAddressBook().stream()
                .map(AddressBookEntry::getNodeId)
                .collect(Collectors.toSet());

        try {
            log.infof(
                    "Applying consensus node update for network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            KubernetesClient client = context.getClient();
            String namespace = resource.getMetadata().getNamespace();
            NowFrozenConsensusCapsules nowFrozen = new NowFrozenConsensusCapsules(resource, status);
            forEach(
                    client.resources(ConsensusCapsule.class).inNamespace(namespace),
                    cc -> update(client, nowFrozen, cc, includeNodes, excludeNodes));

            final List<Long> upgradedNodes = excludeNodes.stream().sorted().toList();
            if (upgradedNodes.size() != includeNodes.size()) {
                final NetworkOperationStatus updated = NetworkOperationStepUtil.prepareFailed(
                        resource, status, FAIL_STATUS_MESSAGE.formatted(includeNodes), CONDITION_TYPE);
                return edit(updated)
                        .with(s -> s.withUpgradedNodes(upgradedNodes))
                        .build();
            }

            log.infof(
                    "Successfully upgraded consensus nodes: %s [%s]",
                    upgradedNodes, resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            final Condition condition = new ConditionBuilder()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withType(CONDITION_TYPE)
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(timestamp())
                    .withMessage(SUCCESS_STATUS_MESSAGE)
                    .build();

            NetworkOperationStatus updated = NetworkOperationStepUtil.prepareStatus(status, List.of(condition));
            return edit(updated).with(s -> s.withUpgradedNodes(upgradedNodes)).build();
        } catch (Throwable t) {
            // add upgraded node list so that during retry we can skip the already upgraded nodes
            final List<Long> upgradedNodes = excludeNodes.stream().sorted().toList();
            final NetworkOperationStatus updated =
                    edit(status).with(s -> s.withUpgradedNodes(upgradedNodes)).build();

            // prepare failed status
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, updated, t.getMessage(), CONDITION_TYPE);
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
            log.infof("Updating consensus capsule: %s", capsule.getMetadata().getName());
            client.resource(nowFrozen.update(capsule)).update();
            excludeNodes.add(nodeId);
            log.infof("List of updated consensus capsules: %s", excludeNodes);
        }
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
