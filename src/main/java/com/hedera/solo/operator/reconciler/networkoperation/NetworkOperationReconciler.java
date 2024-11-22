// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static java.util.Objects.nonNull;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.library.numeric.RolloverComparator;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.resources.NetworkOperationClusterConfig;
import com.hedera.solo.operator.reconciler.networkoperation.resources.NetworkOperationConsensusCapsule;
import com.hedera.solo.operator.reconciler.networkoperation.resources.NetworkOperationOrbit;
import com.hedera.solo.operator.reconciler.networkoperation.resources.NetworkOperationTrigger;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClientException;
import io.javaoperatorsdk.operator.api.reconciler.Cleaner;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.time.Instant;
import java.time.ZonedDateTime;
import java.time.temporal.ChronoUnit;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Queue;
import java.util.Set;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.jboss.logging.Logger;

/** {@code NetworkOperationReconciler} operates on a single CR per namespace at any given time. */
@ControllerConfiguration(name = "network-operation")
public class NetworkOperationReconciler
        implements Cleaner<NetworkOperation>, EventSourceInitializer<NetworkOperation>, Reconciler<NetworkOperation> {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private static final Set<NetworkOperationStatus.Phase> DONE =
            Set.of(NetworkOperationStatus.Phase.Failed, NetworkOperationStatus.Phase.Succeeded);
    private static final Set<ConsensusCapsuleStatus.Phase> RECOGNIZED_CAPSULE_PHASES = Set.of(
            ConsensusCapsuleStatus.Phase.Failed,
            ConsensusCapsuleStatus.Phase.Pending,
            ConsensusCapsuleStatus.Phase.Running,
            ConsensusCapsuleStatus.Phase.Succeeded,
            ConsensusCapsuleStatus.Phase.Unknown);
    private static final Duration DELAY = Duration.of(100, ChronoUnit.MILLIS);
    private EventSourceContext<NetworkOperation> eventSourceContext;
    private final AtomicLong sequenceNumber = new AtomicLong();
    private final RolloverComparator rolloverComparator = new RolloverComparator(63);
    private final NetworkOperationClusterConfig clusterConfig = new NetworkOperationClusterConfig(this);
    private final NetworkOperationConsensusCapsule consensusCapsule = new NetworkOperationConsensusCapsule(this);
    private final NetworkOperationOrbit orbit = new NetworkOperationOrbit(this);
    private final NetworkOperationTrigger trigger = new NetworkOperationTrigger(this);
    private final Workflow<NetworkOperation> workflow = new WorkflowBuilder<NetworkOperation>()
            .addDependentResource(clusterConfig)
            .addDependentResource(consensusCapsule)
            .dependsOn(clusterConfig)
            .addDependentResource(orbit)
            .addDependentResource(trigger)
            .build();
    public final Comparator<NetworkOperation> prioritize = Comparator.<NetworkOperation, ZonedDateTime>comparing(
                    resource -> ZonedDateTime.parse(resource.getMetadata().getCreationTimestamp()))
            .thenComparing(resource -> resource.getStatus().getSequenceNumber(), rolloverComparator);

    @Override
    public UpdateControl<NetworkOperation> reconcile(
            final NetworkOperation resource, final Context<NetworkOperation> context) throws Exception {
        try {
            log.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
            // has the resource status been initialized?
            if (null == resource.getStatus()) {
                if (null != context.getClient().resource(resource).get().getStatus()) {
                    // cache is old
                    return UpdateControl.<NetworkOperation>noUpdate().rescheduleAfter(DELAY);
                }
                // initialize resource status
                return UpdateControl.patchStatus(edit(resource)
                                .with(status -> status.editStatus()
                                        .withPhase(NetworkOperationStatus.Phase.Pending)
                                        .withSequenceNumber(
                                                rolloverComparator.rollover(sequenceNumber.getAndIncrement())))
                                .build())
                        .rescheduleAfter(DELAY);
            }
            // is this resource done?
            if (DONE.contains(resource.getStatus().getPhase())) {
                trigger.next(resource, context);
                return UpdateControl.noUpdate();
            }
            // is this resource next in the queue?
            String name = resource.getMetadata().getName();
            Queue<String> queue =
                    queue(context.getPrimaryCache().list(resource.getMetadata().getNamespace()));
            if (name.equals(queue.peek())) {
                // it appears to be first based on the cache, so verify with client
                queue = queue(context
                        .getClient()
                        .resources(NetworkOperation.class)
                        .inNamespace(resource.getMetadata().getNamespace())
                        .list()
                        .getItems()
                        .stream());
            }
            if (!name.equals(queue.peek())) {
                // this resource is not first in the queue
                if (candidate(resource) && !queue.contains(name)) {
                    // cache is old
                    trigger.next(resource, context);
                }
                return UpdateControl.noUpdate();
            }
            // have the capsules been selected?
            if (resource.getStatus().getConsensusCapsules() == null) {
                NetworkOperation current =
                        context.getClient().resource(resource).get();
                if (null != current.getStatus().getConsensusCapsules()) {
                    // cache is old
                    return UpdateControl.<NetworkOperation>noUpdate().rescheduleAfter(DELAY);
                }
                // select capsules
                return UpdateControl.patchStatus(edit(resource)
                                .with(status -> status.editStatus()
                                        .withConsensusCapsules(
                                                consensusCapsule
                                                        .getSecondaryResources(resource, context)
                                                        .values()
                                                        .stream()
                                                        .map(CustomResource::getMetadata)
                                                        .map(ObjectMeta::getName)
                                                        .sorted()
                                                        .toList()))
                                .build())
                        .rescheduleAfter(DELAY);
            }

            // reconcile dependent resources through workflow
            workflow.reconcile(resource, context);

            // determine phase
            NetworkOperation patchStatus = edit(resource)
                    .with(status -> status.editStatus()
                            .withObservedGeneration(resource.getMetadata().getGeneration()))
                    .with(determinePhase(resource, context))
                    .build();
            if (DONE.contains(patchStatus.getStatus().getPhase())) {
                trigger.next(resource, context);
            }
            return UpdateControl.patchStatus(patchStatus);
        } finally {
            log.infof(
                    "Network Initialization - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

    // collect consensus capsule status in status until any fail or all are running.
    private BuilderFunction<NetworkOperationBuilder> determinePhase(
            final NetworkOperation resource, final Context<NetworkOperation> context) {
        Collection<ConsensusCapsule> consensusCapsules =
                consensusCapsules(resource, context).values();
        List<ConsensusCapsuleStatus.Phase> phaseList = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .filter(Objects::nonNull)
                .map(ConsensusCapsuleStatus::getPhase)
                .filter(Objects::nonNull)
                .toList();
        Set<ConsensusCapsuleStatus.Phase> phases = new HashSet<>(phaseList);
        NetworkOperationStatus.Phase phase;
        List<Condition> conditions = new ArrayList<>();
        if (Set.of(ConsensusCapsuleStatus.Phase.Running).equals(phases)
                && consensusCapsules.size() == phaseList.size()) {
            phase = NetworkOperationStatus.Phase.Succeeded;
            conditions.add(new ConditionBuilder()
                    .withType("Ready")
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(Instant.now().toString())
                    .build());
        } else if (Set.of(
                        ConsensusCapsuleStatus.Phase.Pending,
                        ConsensusCapsuleStatus.Phase.Running,
                        ConsensusCapsuleStatus.Phase.Unknown)
                .containsAll(phases)) {
            phase = NetworkOperationStatus.Phase.Running;
        } else {
            phase = NetworkOperationStatus.Phase.Failed;
            Map<String, ConsensusCapsuleStatus.Phase> map = consensusCapsules.stream()
                    .filter(c -> nonNull(c.getStatus()) && nonNull(c.getStatus().getPhase()))
                    .sorted(Comparator.comparing(c -> c.getMetadata().getName()))
                    .collect(
                            LinkedHashMap::new,
                            (m, c) -> m.put(
                                    c.getMetadata().getName(), c.getStatus().getPhase()),
                            Map::putAll);
            conditions.add(new ConditionBuilder()
                    .withType("Ready")
                    .withStatus(ConditionStatus.False)
                    .withReason("CapsulePhase")
                    .withMessage("Capsule phases: " + map)
                    .withLastTransitionTime(Instant.now().toString())
                    .build());
            log.warnf(
                    "Network Initialization - Failed [ name = '%s', capsules = '%s' ]",
                    resource.getMetadata().getName(), map);
        }
        return builder -> builder.editStatus().withPhase(phase).addAllToConditions(conditions);
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkOperation> context) {
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }

    public EventSourceContext<NetworkOperation> eventSourceContext() {
        return eventSourceContext;
    }

    public boolean candidate(final NetworkOperation resource) {
        return null == resource.getMetadata().getDeletionTimestamp()
                && null != resource.getStatus()
                && Set.of(NetworkOperationStatus.Phase.Pending, NetworkOperationStatus.Phase.Running)
                        .contains(resource.getStatus().getPhase());
    }

    Queue<String> queue(final Stream<NetworkOperation> stream) {
        return stream.filter(this::candidate)
                .sorted(prioritize)
                .map(CustomResource::getMetadata)
                .map(ObjectMeta::getName)
                .collect(Collectors.toCollection(ArrayDeque::new));
    }

    @Override
    public DeleteControl cleanup(final NetworkOperation resource, final Context<NetworkOperation> context) {
        try {
            trigger.next(resource, context);
        } catch (KubernetesClientException ignored) {
        }
        return DeleteControl.defaultDelete();
    }

    public Set<ResourceID> toPrimaryResourceIDs(final String namespace) {
        return eventSourceContext()
                .getPrimaryCache()
                .list(namespace)
                .filter(this::candidate)
                .min(prioritize)
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElseGet(Collections::emptySet);
    }

    public Map<String, ConsensusCapsule> consensusCapsules(
            final NetworkOperation primary, final Context<NetworkOperation> context) {
        return consensusCapsule.getSecondaryResources(primary, context);
    }
}
