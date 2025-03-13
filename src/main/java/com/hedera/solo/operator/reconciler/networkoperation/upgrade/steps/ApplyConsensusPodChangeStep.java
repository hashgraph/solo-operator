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
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            log.debugf(
                    "Skipping consensus node update for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status));
            return status;
        }

        log.infof(
                "Applying consensus node update for network operation: %s/%s",
                resource.getMetadata().getNamespace(), resource.getMetadata().getName());

        Set<Long> excludeNodes = new HashSet<>(resource.getStatus().getUpgradedNodes());
        Set<Long> includeNodes = resource.getSpec().getNetworkUpgrade().getAddressBook().stream()
                .map(AddressBookEntry::getNodeId)
                .collect(Collectors.toSet());

        KubernetesClient client = context.getClient();
        String namespace = resource.getMetadata().getNamespace();
        NowFrozenConsensusCapsules nowFrozen = new NowFrozenConsensusCapsules(resource, status);

        forEach(
                client.resources(ConsensusCapsule.class).inNamespace(namespace),
                cc -> update(client, nowFrozen, cc, includeNodes, excludeNodes));

        List<Long> upgradedNodes = excludeNodes.stream().sorted().toList();

        // FIXME need to make this step retryable without running the steps that come after it
        //  Thoughts: rather than saving the upgrade list in the status, we could add labels (e.g. upgraded-by,
        //  upgraded-at) in the consensus capsule to indicate that it has been upgraded by a certain network-operation.
        //  Then we can skip updating it if the labels are present.
        //  So, if not all nodes are upgraded, we can throw exception and retry without running following steps.
        if (upgradedNodes.size() != includeNodes.size()) {
            log.infof(
                    "Failed to upgrade all consensus nodes, reattempting during next reconciliation... [ updated: %s]",
                    upgradedNodes);
            return edit(status).with(s -> s.withUpgradedNodes(upgradedNodes)).build();
        }

        log.infof("Successfully upgrade consensus nodes [updated = %s]", upgradedNodes);
        return edit(status)
                .with(s -> s.addToConditions(new ConditionBuilder()
                                .withObservedGeneration(resource.getMetadata().getGeneration())
                                .withType(CONDITION_TYPE)
                                .withStatus(ConditionStatus.True)
                                .withLastTransitionTime(timestamp())
                                .withMessage(SUCCESS_STATUS_MESSAGE)
                                .build())
                        .withUpgradedNodes(upgradedNodes))
                .build();
    }

    @Override
    public NetworkOperationStatus handleError(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context, Exception ex) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperation status is required");
        Objects.requireNonNull(ex, "Exception is required");

        log.errorf(
                ex,
                "Failed to update consensus nodes for network operation %s/%s: %s"
                        .formatted(
                                resource.getMetadata().getNamespace(),
                                resource.getMetadata().getName(),
                                resource.getStatus().getConsensusCapsules()));

        final String failMessage = FAIL_STATUS_MESSAGE.formatted(ex.getMessage());
        return NetworkOperationStepUtil.prepareFailed(resource, status, failMessage, CONDITION_TYPE);
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
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
