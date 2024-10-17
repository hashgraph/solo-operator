/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.reconciler.networkinitialization;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.numeric.RolloverCounter;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationClusterConfig;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationConsensusCapsule;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationOrbit;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationTrigger;
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
import java.time.ZonedDateTime;
import java.time.temporal.ChronoUnit;
import java.util.ArrayDeque;
import java.util.Collection;
import java.util.Collections;
import java.util.Comparator;
import java.util.Map;
import java.util.Objects;
import java.util.Queue;
import java.util.Set;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.jboss.logging.Logger;

/** {@code NetworkInitializationReconciler} operates on a single CR per namespace at any given time. */
@ControllerConfiguration(name = "network-initialization")
public class NetworkInitializationReconciler
        implements Cleaner<NetworkInitialization>,
                EventSourceInitializer<NetworkInitialization>,
                Reconciler<NetworkInitialization> {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private static final Set<NetworkInitializationStatus.Phase> DONE =
            Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded);
    private static final Duration DELAY = Duration.of(100, ChronoUnit.MILLIS);
    private EventSourceContext<NetworkInitialization> eventSourceContext;
    private final AtomicLong sequenceNumber = new AtomicLong();
    private final RolloverCounter counter = new RolloverCounter(63);
    private final NetworkInitializationClusterConfig clusterConfig = new NetworkInitializationClusterConfig();
    private final NetworkInitializationConsensusCapsule consensusCapsule =
            new NetworkInitializationConsensusCapsule(this);
    private final NetworkInitializationOrbit orbit = new NetworkInitializationOrbit(this);
    private final NetworkInitializationTrigger trigger = new NetworkInitializationTrigger(this);
    private final Workflow<NetworkInitialization> workflow = new WorkflowBuilder<NetworkInitialization>()
            .addDependentResource(clusterConfig)
            .addDependentResource(consensusCapsule)
            .dependsOn(clusterConfig)
            .addDependentResource(orbit)
            .addDependentResource(trigger)
            .build();
    public final Comparator<NetworkInitialization> prioritize =
            Comparator.<NetworkInitialization, ZonedDateTime>comparing(resource ->
                            ZonedDateTime.parse(resource.getMetadata().getCreationTimestamp()))
                    .thenComparing(resource -> resource.getStatus().getSequenceNumber(), counter);

    @Override
    public UpdateControl<NetworkInitialization> reconcile(
            final NetworkInitialization resource, final Context<NetworkInitialization> context) throws Exception {
        try {
            log.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
            // has the resource status been initialized?
            if (null == resource.getStatus()) {
                if (null != context.getClient().resource(resource).get().getStatus()) {
                    // cache is old
                    return UpdateControl.<NetworkInitialization>noUpdate().rescheduleAfter(DELAY);
                }
                // initialize resource status
                return UpdateControl.patchStatus(edit(resource)
                                .with(status -> status.editStatus()
                                        .withPhase(NetworkInitializationStatus.Phase.Pending)
                                        .withSequenceNumber(counter.rollover(sequenceNumber.getAndIncrement())))
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
                        .resources(NetworkInitialization.class)
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
                NetworkInitialization current =
                        context.getClient().resource(resource).get();
                if (null != current.getStatus().getConsensusCapsules()) {
                    // cache is old
                    return UpdateControl.<NetworkInitialization>noUpdate().rescheduleAfter(DELAY);
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
            NetworkInitializationStatus.Phase phase = getPhase(resource, context);
            if (DONE.contains(phase)) {
                trigger.next(resource, context);
            }
            return UpdateControl.patchStatus(edit(resource)
                    .with(status -> status.editStatus()
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withPhase(phase))
                    .build());
        } finally {
            log.infof(
                    "Network Initialization - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

    // collect consensus capsule status in status until any fail or all are running.
    private NetworkInitializationStatus.Phase getPhase(
            final NetworkInitialization resource, final Context<NetworkInitialization> context) {
        Collection<ConsensusCapsule> consensusCapsules =
                consensusCapsule.getSecondaryResources(resource, context).values();
        Set<ConsensusCapsuleStatus.Phase> phases = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .filter(Objects::nonNull)
                .map(ConsensusCapsuleStatus::getPhase)
                .filter(Objects::nonNull)
                .collect(Collectors.toSet());

        NetworkInitializationStatus.Phase phase = NetworkInitializationStatus.Phase.Failed;
        if (Set.of(ConsensusCapsuleStatus.Phase.Running).equals(phases)) {
            phase = NetworkInitializationStatus.Phase.Succeeded;
        } else if (Set.of(
                        ConsensusCapsuleStatus.Phase.Pending,
                        ConsensusCapsuleStatus.Phase.Running,
                        ConsensusCapsuleStatus.Phase.Unknown)
                .containsAll(phases)) {
            phase = NetworkInitializationStatus.Phase.Running;
        }
        return phase;
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitialization> context) {
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }

    public EventSourceContext<NetworkInitialization> eventSourceContext() {
        return eventSourceContext;
    }

    public boolean candidate(final NetworkInitialization resource) {
        return null == resource.getMetadata().getDeletionTimestamp()
                && null != resource.getStatus()
                && Set.of(NetworkInitializationStatus.Phase.Pending, NetworkInitializationStatus.Phase.Running)
                        .contains(resource.getStatus().getPhase());
    }

    Queue<String> queue(final Stream<NetworkInitialization> stream) {
        return stream.filter(this::candidate)
                .sorted(prioritize)
                .map(CustomResource::getMetadata)
                .map(ObjectMeta::getName)
                .collect(Collectors.toCollection(ArrayDeque::new));
    }

    @Override
    public DeleteControl cleanup(final NetworkInitialization resource, final Context<NetworkInitialization> context) {
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
}
