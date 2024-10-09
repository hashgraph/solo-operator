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
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.lang.invoke.MethodHandles;
import java.time.ZonedDateTime;
import java.util.Collection;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.jboss.logging.Logger;
import org.slf4j.LoggerFactory;

// can maybe handle secrets inside consensus capsule after eligible for deployment is set?

// allow only one NI to be running
// DR mutation can only occur when NI is running or when first created and status is null and no other NI is running

/**
 * Document this and/or change it because its basically an anti-pattern for a reconciler to sequence primary resources
 *
 * An alternative approach to achieve similar outcome would be for reconciler to add primary to a list and have second reconciler
 * that maps primary to secondary as first entry in list. This assumes we cqn update status in the secondary.
 *
 * Only downside of alternative is we need to assume uniqueness of list in namespace since we are apparently required not to reference it from
 * orbit but instead reference orbit from it. Or in other words we need to select the list for our orbit, just name list with orbit name.
 *
 * primary to secondary mapper either needs to rely on cache (maybe not up to date?) or yse client (latency?) or scoped-value (preview).
 * maybe ok if not called often?
 *
 * another alternative is to use bulk DR, but this is likely not a great either.
 * */

@ControllerConfiguration(name = "network-initialization"/*, labelSelector = "phase notin (Failed, Succeeded)"*/)
public class NetworkInitializationReconciler implements
//        ErrorStatusHandler<NetworkInitialization>,
        Reconciler<NetworkInitialization>, EventSourceInitializer<NetworkInitialization>,
        Cleaner<NetworkInitialization>
{
    private final AtomicLong sequenceNumber = new AtomicLong();
    private final RolloverCounter counter = new RolloverCounter(63);
    private static final org.slf4j.Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    private static final Logger logger = Logger.getLogger(MethodHandles.lookup().lookupClass());
//    private final KubernetesClient client;
//    private static final Set<NetworkInitializationStatus.Phase> terminalPhases =
//            Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded);

    private EventSourceContext<NetworkInitialization> eventSourceContext;
    private final NetworkInitializationClusterConfig clusterConfig = new NetworkInitializationClusterConfig();
    private final NetworkInitializationConsensusCapsule consensusCapsule = new NetworkInitializationConsensusCapsule(this);
    private final NetworkInitializationOrbit orbit = new NetworkInitializationOrbit(this);
    private final NetworkInitializationTrigger trigger = new NetworkInitializationTrigger(this);
    private final Workflow<NetworkInitialization> workflow = new WorkflowBuilder<NetworkInitialization>()
            .addDependentResource(consensusCapsule)
            .addDependentResource(orbit)
            .addDependentResource(trigger)
            .addDependentResource(clusterConfig).dependsOn(consensusCapsule, orbit)
            .build();

//    @Inject
//    public NetworkInitializationReconciler(final KubernetesClient client) {
////        this.client = client;
//    }

    public EventSourceContext<NetworkInitialization> eventSourceContext() {
        return eventSourceContext;
    }

//    static final long msb = 1L << 63;
//    static final long mask = ~msb;
//    static final long half = msb >>> 1;
//
////    static final long half = Long.MIN_VALUE >>> 1;
//    int compare(long a, long b) {
//        long diff = a - b & mask;
//        return (diff | half) == half ? 0 : diff < half ? 1 : -1;
//    }

    public Predicate<NetworkInitialization> active() {
        return x ->
                null == x.getMetadata().getDeletionTimestamp() &&
                null != x.getStatus() && Set.of(NetworkInitializationStatus.Phase.Pending, NetworkInitializationStatus.Phase.Running).contains(x.getStatus().getPhase());
    }

    public Comparator<NetworkInitialization> prioritize() {
        return Comparator.<NetworkInitialization, ZonedDateTime>comparing(x -> ZonedDateTime.parse(x.getMetadata().getCreationTimestamp())).thenComparing(x -> x.getStatus().getSequenceNumber(), counter);
    }

//    void trigger(final NetworkInitialization resource, final Context<NetworkInitialization> context) {
//        context.getClient().resource(edit(new ConfigMap())
//                .with(builder -> builder
//                        .editMetadata()
//                        .withName(trigger.NAME)
//                        .withNamespace(resource.getMetadata().getNamespace())
//                        .addToLabels("app", "consensus")
//                        .addToLabels("type", "trigger"))
//                .with(builder -> builder
//                        .addToData("count", String.valueOf(counter.rollover(sequenceNumber.getAndIncrement()))))
//                .build()).createOr(Updatable::update);
////        try {
////            context.getClient().configMaps().inNamespace(resource.getMetadata().getNamespace()).withName(state.NAME).create();
////        } catch (KubernetesClientException ignored) {
////            log.info("error", ignored);
////        }
////        context.getClient().configMaps().inNamespace(resource.getMetadata().getNamespace()).withName(state.NAME).edit(x -> edit(x)
//////                .with(builder -> builder
//////                        .editMetadata()
//////                        .withName(state.NAME)
//////                        .withNamespace(resource.getMetadata().getNamespace()))
////                .with(builder -> builder
////                        .addToData("count", String.valueOf(counter.rollover(sequenceNumber.getAndIncrement()))))
////                .build());
//////                .build()).createOr(Updatable::update);
//    }

    @Override
    public UpdateControl<NetworkInitialization> reconcile(
            final NetworkInitialization resource, final Context<NetworkInitialization> context) throws Exception {
        try {
            logger.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
            if (null == resource.getStatus()) {
                log.info("adding status for {}", resource.getMetadata().getName());
//                context.getSecondaryResources(ConsensusCapsule.class);
//                trigger(resource, context);
                NetworkInitialization current = context.getClient().resource(resource).get();
                if (null != current.getStatus()) {
                    return UpdateControl.<NetworkInitialization>noUpdate().rescheduleAfter(1, TimeUnit.SECONDS);
                }
                return UpdateControl.patchStatus(edit(context.getClient().resource(resource).get())
                                .with(builder -> builder
                                        .editStatus()
//                                        .withObservedGeneration(resource.getMetadata().getGeneration())
                                        .withPhase(NetworkInitializationStatus.Phase.Pending)
                                        .withSequenceNumber(counter.rollover(sequenceNumber.getAndIncrement())))
//                                        .withConsensusCapsules(consensusCapsule.getSecondaryResources(resource, context).values().stream()
//                                                .map(CustomResource::getMetadata)
//                                                .map(ObjectMeta::getName)
//                                                .sorted()
//                                                .toList()))
                                .build())
                        .rescheduleAfter(0);
            }
            log.info("NetworkInitializationReconciler phase {} {}", resource.getMetadata().getName(), resource.getStatus().getPhase());
            if (Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded).contains(resource.getStatus().getPhase())) {
                log.info("NetworkInitializationReconciler update state {}", resource.getMetadata().getName());
                trigger.next(resource, context);
                return UpdateControl.noUpdate();
//                context.getClient().configMaps().inNamespace(resource.getMetadata().getNamespace()).resource(edit(context.getSecondaryResource(ConfigMap.class, state).orElseThrow())
//                        .with(builder -> builder
//                                .addToData("count", String.valueOf(Math.max(0, Integer.parseInt(builder.getData().getOrDefault("count", "0")) + 1))))
//                        .build()).update();
//                return UpdateControl.patchStatus(edit(resource)
//                        .with(builder -> builder
//                                .editStatus()
//                                .withObservedGeneration(resource.getMetadata().getGeneration()))
//                        .build());
            }
            String name = resource.getMetadata().getName();
            Function<Stream<NetworkInitialization>, List<String>> function = stream -> stream
                    .filter(active())
                    .sorted(prioritize())
                    .map(CustomResource::getMetadata)
                    .map(ObjectMeta::getName)
                    .toList();
            List<String> list = function.apply(context.getPrimaryCache().list(resource.getMetadata().getNamespace()));
            log.info("from cache list {}", list);
            if (!list.isEmpty() && name.equals(list.getFirst())) {
                list = function.apply(context.getClient().resources(NetworkInitialization.class).inNamespace(resource.getMetadata().getNamespace()).list().getItems().stream());
                log.info("from client list {}", list);
//                if (!list.contains(name)) {
//                    // cache out of sync
//                    return UpdateControl.<NetworkInitialization>noUpdate().rescheduleAfter(0);
//                }
            }
            if (list.isEmpty() || !name.equals(list.getFirst())) {
                UpdateControl<NetworkInitialization> updateControl = UpdateControl.noUpdate();
//                long index = list.indexOf(name);
//                return -1 != index ? updateControl.rescheduleAfter(3 * (index + 1), TimeUnit.SECONDS) : updateControl;
//                log.info("reconciliation list {} {}", resource.getMetadata().getName(), list);
//                log.info("reconciliation 1 done for {} active {} inlist {} phase {}", resource.getMetadata().getName(), active().test(resource), list.contains(name), resource.getStatus().getPhase());
                if (active().test(resource) && !list.contains(name)) {
                    // cache is out of sync
                    log.info("cache is out of sync {}", resource.getMetadata().getName());
                    trigger.next(resource, context);
                    //updateControl.rescheduleAfter(1, TimeUnit.SECONDS);
                }
//                log.info("reconciliation 2 done for {} active {} inlist {} phase {}", resource.getMetadata().getName(), active().test(resource), list.contains(name), resource.getStatus().getPhase());
                return updateControl;
            }
            if (resource.getStatus().getConsensusCapsules() == null) {
                NetworkInitialization current = context.getClient().resource(resource).get();
                if (null != current.getStatus().getConsensusCapsules()) {
                    log.info("already have list of capsules!");
                    return UpdateControl.<NetworkInitialization>noUpdate().rescheduleAfter(1, TimeUnit.SECONDS);
                }
                return UpdateControl.patchStatus(edit(resource)
                                .with(builder -> builder
                                        .editStatus()
//                                        .withObservedGeneration(resource.getMetadata().getGeneration())
//                                        .withPhase(NetworkInitializationStatus.Phase.Pending)
//                                        .withSequenceNumber(counter.rollover(sequenceNumber.getAndIncrement()))
                                        .withConsensusCapsules(consensusCapsule.getSecondaryResources(resource, context).values().stream()
                                                .map(CustomResource::getMetadata)
                                                .map(ObjectMeta::getName)
                                                .sorted()
                                                .toList()))
                                .build())
                        .rescheduleAfter(0);
            }
            log.info("reconciling for {}", resource.getMetadata().getName());
//            logger.info("NetworkInitializationReconciler capsules: "
//                    + context.getSecondaryResources(ConsensusCapsule.class).size());
//            logger.info("NetworkInitializationReconciler capsules: "
//                    + context.getSecondaryResourcesAsStream(ConsensusCapsule.class)
//                    .map(CustomResource::getMetadata)
//                    .map(ObjectMeta::getName)
//                    .toList());
//            logger.info("NetworkInitializationReconciler orbit: " + context.getSecondaryResources(Orbit.class).size());
//            logger.info("NetworkInitializationReconciler orbit: "
//                    + context.getSecondaryResourcesAsStream(Orbit.class)
//                    .map(CustomResource::getMetadata)
//                    .map(ObjectMeta::getName)
//                    .toList());

//            List<String> running = context.getPrimaryCache().list(resource.getMetadata().getNamespace())
//                    .filter(r -> null != r.getStatus() && NetworkInitializationStatus.Phase.Running.equals(r.getStatus().getPhase()))
//                    .map(r -> r.getMetadata().getName())
//                    .toList();
//            if (!running.isEmpty() && !running.equals(List.of(resource.getMetadata().getName()))) {
//                return UpdateControl.patchStatus(edit(resource)
//                        .with(builder -> builder
//                                .editStatus()
//                                .withObservedGeneration(resource.getMetadata().getGeneration())
//                                .withPhase(NetworkInitializationStatus.Phase.Failed))
//                        .build());
//            }

//            final boolean seen = resource.getStatus() != null
//                    && resource.getStatus().getObservedGeneration() != null
//                    && Objects.equals(
//                    resource.getStatus().getObservedGeneration(),
//                    resource.getMetadata().getGeneration());
//
//            if (seen && !active(resource)) {
//                return UpdateControl.noUpdate();
//            }

//            logger.info("net-init perform reconciliation " + resource.getMetadata().getName());
            workflow.reconcile(resource, context);


//            final var consensusCapsuleClient = client.resources(ConsensusCapsule.class)
//                    .inNamespace(resource.getMetadata().getNamespace());
//
//            if (!seen) {
//                // process new generation by marking all consensus capsules eligible for deployment
//                consensusCapsuleClient
//                        .resources()
//                        .forEach(rc -> rc.edit(cc -> cc.edit()
//                                .editSpec()
//                                .withEligibleForDeployment(true)
//                                .endSpec()
//                                .build()));
//            }

//            NetworkInitializationStatus.Phase phase = getPhase(resource, context);
//            NetworkInitialization updated = edit(resource)
//                    .with(builder -> builder
//                            .editMetadata()
//                            .addToLabels("phase", phase.name()))
//                    .with(builder -> builder
//                            .editStatus()
//                            .withObservedGeneration(resource.getMetadata().getGeneration())
//                            .withPhase(phase))
//                    .build();
            NetworkInitializationStatus.Phase phase = getPhase(resource, context);
            if (Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded).contains(phase)) {
                trigger.next(resource, context);
            }
            UpdateControl<NetworkInitialization> updateControl = UpdateControl.patchStatus(resource.edit()
                    .editStatus()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withPhase(phase)
                    .endStatus()
                    .build());
//            if (Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded).contains(phase)) {
//                updateControl.rescheduleAfter(0);
//            }
            return updateControl;
//                    .rescheduleAfter(Duration.ofSeconds(30));
//        } catch (UpdateInactiveException e) {
//            logger.info("Network Initialization - caught UpdateInactiveException " + resource.getMetadata().getName());
//            if (null == resource.getStatus()) {
//                return UpdateControl.patchStatus(edit(resource)
//                        .with(builder -> builder
//                                .editStatus()
//                                .withPhase(NetworkInitializationStatus.Phase.Failed))
//                        .build());
//            }
//            throw e;
        } catch (Exception e) {
            log.info("exception:", e);
            throw e;
        } finally {
            logger.infof(
                    "Network Initialization - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

//    public static boolean active(NetworkInitialization resource) {
//        return null == resource.getStatus() || NetworkInitializationStatus.Phase.Running.equals(resource.getStatus().getPhase());
//    }

    // collect consensus capsule status in status until any fail or all succeed.
    private NetworkInitializationStatus.Phase getPhase(final NetworkInitialization resource, final Context<NetworkInitialization> context) {
        Collection<ConsensusCapsule> consensusCapsules = consensusCapsule.getSecondaryResources(resource, context).values();
        Set<ConsensusCapsuleStatus.Phase> phases = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .filter(Objects::nonNull)
                .map(ConsensusCapsuleStatus::getPhase)
                .filter(Objects::nonNull)
                .collect(Collectors.toSet());

        NetworkInitializationStatus.Phase phase = NetworkInitializationStatus.Phase.Failed;
        if (Set.of(ConsensusCapsuleStatus.Phase.Running, ConsensusCapsuleStatus.Phase.Succeeded).containsAll(phases)) {
            phase = NetworkInitializationStatus.Phase.Succeeded;
        }
        else if (Set.of(ConsensusCapsuleStatus.Phase.Pending, ConsensusCapsuleStatus.Phase.Running, ConsensusCapsuleStatus.Phase.Unknown).containsAll(phases)) {
//            List<String> running = context.getPrimaryCache().list(resource.getMetadata().getNamespace())
//                    .filter(r -> null != r.getStatus() && NetworkInitializationStatus.Phase.Running.equals(r.getStatus().getPhase()))
//                    .map(r -> r.getMetadata().getName())
//                    .toList();
//            if (running.isEmpty() || running.equals(List.of(resource.getMetadata().getName()))) {
            phase = NetworkInitializationStatus.Phase.Running;
//            }
        }
        logger.info("set to " + phase + " " + resource.getMetadata().getName());
        if (NetworkInitializationStatus.Phase.Failed.equals(phase)) {
            log.info("phases: {}", phases);
        }
        return phase;
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitialization> context) {
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }

//    @Override
//    public ErrorStatusUpdateControl<NetworkInitialization> updateErrorStatus(NetworkInitialization resource, Context<NetworkInitialization> context, Exception e) {
////        if (null == resource.getStatus() && e instanceof UpdateInactiveException) {
////            return ErrorStatusUpdateControl.updateStatus(edit(resource)
////                    .with(builder -> builder
////                            .editStatus()
////                            .withPhase(NetworkInitializationStatus.Phase.Failed))
////                    .build());
////        }
//        return ErrorStatusUpdateControl.<NetworkInitialization>noStatusUpdate().rescheduleAfter(new Random().nextInt() & 0x1fff, TimeUnit.MILLISECONDS);
//    }

    @Override
    public DeleteControl cleanup(NetworkInitialization resource, Context<NetworkInitialization> context) {
        log.info("cleanup: {}", resource.getMetadata().getName());
        try {
            trigger.next(resource, context);
        } catch (KubernetesClientException ignored) {}
        return DeleteControl.defaultDelete();
    }
}
