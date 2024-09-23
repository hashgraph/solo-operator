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
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationClusterConfig;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationConsensusCapsule;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationOrbit;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Collection;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Random;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;
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

@ControllerConfiguration(name = "network-initialization")
public class NetworkInitializationReconciler implements
        ErrorStatusHandler<NetworkInitialization>,
        Reconciler<NetworkInitialization>, EventSourceInitializer<NetworkInitialization>
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
    private final Workflow<NetworkInitialization> workflow = new WorkflowBuilder<NetworkInitialization>()
            .addDependentResource(consensusCapsule)
            .addDependentResource(orbit)
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
                return UpdateControl.patchStatus(edit(resource)
                                .with(builder -> builder
                                        .editStatus()
//                                        .withObservedGeneration(resource.getMetadata().getGeneration())
                                        .withPhase(NetworkInitializationStatus.Phase.Pending)
                                        .withSequenceNumber(counter.rollover(sequenceNumber.getAndIncrement()))
                                        .withConsensusCapsules(consensusCapsule.getSecondaryResources(resource, context).values().stream()
                                                .map(CustomResource::getMetadata)
                                                .map(ObjectMeta::getName)
                                                .sorted()
                                                .toList()))
                                .build())
                        // todo: add capsules later, don't need to sleep here if using client not cache later
                        .rescheduleAfter(1, TimeUnit.SECONDS);
            }
            List<String> queue = context.getPrimaryCache().list(resource.getMetadata().getNamespace()).toList().stream()
                    .filter(x -> null != x.getStatus())
                    .filter(x -> Set.of(NetworkInitializationStatus.Phase.Pending, NetworkInitializationStatus.Phase.Running).contains(x.getStatus().getPhase()))
                    // todo: first compare by creation time (since this sequence number will reset if operator restarts)
                    .sorted(Comparator.comparing(x -> x.getStatus().getSequenceNumber(), counter))
                    .map(CustomResource::getMetadata)
                    .map(ObjectMeta::getName)
                    .toList();
            String name = resource.getMetadata().getName();
            if (queue.isEmpty() || !name.equals(queue.getFirst())) {
                UpdateControl<NetworkInitialization> updateControl = UpdateControl.noUpdate();
                int index = queue.indexOf(name);
                log.info("not first, maybe reschedule {}", resource.getMetadata().getName());
                // todo: use randomized "backoff" delay based on position in queue
                // backoff = min(cap, base * 2 ** attempt)
                // full_jitter = random_between(0, min(cap, base * 2 ** attempt))
                // decor_jitter = min(cap, random_between(base, sleep * 3))
                // todo: full_jitter looks like the best option
                // todo: don't need delay here if can check queue and reschedule when first NI completes
                return -1 != index ? updateControl.rescheduleAfter(index + 1, TimeUnit.SECONDS) : updateControl;
            }
            // todo: if capsules are not in status at this point add them (using client and reschedule)
            // todo: perform final queue based check using client
            log.info("reconciling for {}", resource.getMetadata().getName());
            logger.info("NetworkInitializationReconciler capsules: "
                    + context.getSecondaryResources(ConsensusCapsule.class).size());
            logger.info("NetworkInitializationReconciler capsules: "
                    + context.getSecondaryResourcesAsStream(ConsensusCapsule.class)
                    .map(CustomResource::getMetadata)
                    .map(ObjectMeta::getName)
                    .toList());
            logger.info("NetworkInitializationReconciler orbit: " + context.getSecondaryResources(Orbit.class).size());
            logger.info("NetworkInitializationReconciler orbit: "
                    + context.getSecondaryResourcesAsStream(Orbit.class)
                    .map(CustomResource::getMetadata)
                    .map(ObjectMeta::getName)
                    .toList());

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

            logger.info("net-init perform reconciliation " + resource.getMetadata().getName());
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

            return UpdateControl.patchStatus(resource.edit()
                            .editStatus()
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withPhase(getPhase(resource, context))
                            .endStatus()
                            .build())
                    .rescheduleAfter(Duration.ofSeconds(30));
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
        if (Set.of(ConsensusCapsuleStatus.Phase.Pending, ConsensusCapsuleStatus.Phase.Unknown).containsAll(phases)) {
//            List<String> running = context.getPrimaryCache().list(resource.getMetadata().getNamespace())
//                    .filter(r -> null != r.getStatus() && NetworkInitializationStatus.Phase.Running.equals(r.getStatus().getPhase()))
//                    .map(r -> r.getMetadata().getName())
//                    .toList();
//            if (running.isEmpty() || running.equals(List.of(resource.getMetadata().getName()))) {
            phase = NetworkInitializationStatus.Phase.Running;
//            }
        } else if (Set.of(ConsensusCapsuleStatus.Phase.Running, ConsensusCapsuleStatus.Phase.Succeeded).containsAll(phases)) {
            phase = NetworkInitializationStatus.Phase.Succeeded;
        }
        logger.info("set to " + phase + " " + resource.getMetadata().getName());
        return phase;
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitialization> context) {
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }

    @Override
    public ErrorStatusUpdateControl<NetworkInitialization> updateErrorStatus(NetworkInitialization resource, Context<NetworkInitialization> context, Exception e) {
//        if (null == resource.getStatus() && e instanceof UpdateInactiveException) {
//            return ErrorStatusUpdateControl.updateStatus(edit(resource)
//                    .with(builder -> builder
//                            .editStatus()
//                            .withPhase(NetworkInitializationStatus.Phase.Failed))
//                    .build());
//        }
        return ErrorStatusUpdateControl.<NetworkInitialization>noStatusUpdate().rescheduleAfter(new Random().nextInt() & 0x1fff, TimeUnit.MILLISECONDS);
    }
}
