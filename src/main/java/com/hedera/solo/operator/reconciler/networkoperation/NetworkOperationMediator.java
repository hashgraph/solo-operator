// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation;

import com.hedera.solo.operator.library.numeric.RolloverComparator;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.resources.v2.NetworkOperationClusterConfigV2;
import com.hedera.solo.operator.reconciler.networkoperation.resources.v2.NetworkOperationConsensusCapsuleV2;
import com.hedera.solo.operator.reconciler.networkoperation.resources.v2.NetworkOperationOrbitV2;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowReconcileResult;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.time.ZonedDateTime;
import java.util.Collections;
import java.util.Comparator;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

/**
 * {@code NetworkOperationMediator} implements the "Mediator Pattern" in order to facilitate communication among the
 * NetworkOperation reconciler and its dependents.
 */
public class NetworkOperationMediator {
    private EventSourceContext<NetworkOperation> eventSourceContext;
    private static final RolloverComparator rolloverComparator = new RolloverComparator(63);

    // select earliest by creation timestamp, then by sequence number
    public static final Comparator<NetworkOperation> selectEarliest =
            Comparator.<NetworkOperation, ZonedDateTime>comparing(resource ->
                            ZonedDateTime.parse(resource.getMetadata().getCreationTimestamp()))
                    .thenComparing(resource -> resource.getStatus().getSequenceNumber(), rolloverComparator);

    private final NetworkOperationClusterConfigV2 clusterConfig = new NetworkOperationClusterConfigV2(this);
    private final NetworkOperationConsensusCapsuleV2 consensusCapsule = new NetworkOperationConsensusCapsuleV2(this);
    private final NetworkOperationOrbitV2 orbit = new NetworkOperationOrbitV2(this);
    private final Workflow<NetworkOperation> workflow = new WorkflowBuilder<NetworkOperation>()
            .addDependentResource(clusterConfig)
            .withReconcilePrecondition(clusterConfig::isReady)
            .addDependentResource(consensusCapsule)
            .dependsOn(clusterConfig)
            .addDependentResource(orbit)
            .build();

    /** Filter the network operation resources that are in the processing phase */
    public static boolean isProcessing(final NetworkOperation resource) {
        return null == resource.getMetadata().getDeletionTimestamp()
                && null != resource.getStatus()
                && Set.of(NetworkOperationStatus.Phase.Pending, NetworkOperationStatus.Phase.Running)
                        .contains(resource.getStatus().getPhase());
    }

    /** Set the event source context for the network operation resources */
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkOperation> context) {
        Objects.requireNonNull(workflow, "workflow must not be null, call init first");
        Objects.requireNonNull(context, "context must not be null");
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }

    /** Get the event source context */
    public EventSourceContext<NetworkOperation> eventSourceContext() {
        Objects.requireNonNull(
                eventSourceContext, "eventSourceContext must not be null, call prepareEventSources first");
        return eventSourceContext;
    }

    /**
     * Find NetworkOperation resource IDs to be used as primary resource IDs
     *
     * @param namespace name of the namespace
     * @return a set of primary resource IDs
     */
    public Set<ResourceID> primaryResourceIDs(final String namespace) {
        return eventSourceContext()
                .getPrimaryCache()
                .list(namespace)
                .filter(NetworkOperationMediator::isProcessing)
                .min(NetworkOperationMediator.selectEarliest)
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElseGet(Collections::emptySet);
    }

    /** Find consensus capsules associated with the primary network operation resource */
    public Map<String, ConsensusCapsule> findConsensusCapsules(
            final NetworkOperation primary, final Context<NetworkOperation> context) {
        Objects.requireNonNull(consensusCapsule, "consensusCapsule must not be null, call init first");
        return consensusCapsule.getSecondaryResources(primary, context);
    }

    public WorkflowReconcileResult reconcileWorkflow(NetworkOperation resource, Context<NetworkOperation> context) {
        Objects.requireNonNull(workflow, "workflow must not be null, call init first");
        return workflow.reconcile(resource, context);
    }
}
