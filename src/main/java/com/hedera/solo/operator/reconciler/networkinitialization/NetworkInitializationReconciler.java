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

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;
import org.jboss.logging.Logger;

@ControllerConfiguration(name = "network-initialization")
public class NetworkInitializationReconciler implements Reconciler<NetworkInitialization>, EventSourceInitializer<NetworkInitialization> {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
//    private final KubernetesClient client;
    private static final Set<NetworkInitializationStatus.Phase> terminalPhases =
            Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded);

    private EventSourceContext<NetworkInitialization> eventSourceContext;
    private final NetworkInitializationConsensusCapsule consensusCapsule = new NetworkInitializationConsensusCapsule(this);
    private final Workflow<NetworkInitialization> workflow = new WorkflowBuilder<NetworkInitialization>()
            .addDependentResource(consensusCapsule)
            .build();

//    @Inject
//    public NetworkInitializationReconciler(final KubernetesClient client) {
////        this.client = client;
//    }

    public EventSourceContext<NetworkInitialization> eventSourceContext() {
        return eventSourceContext;
    }

    @Override
    public UpdateControl<NetworkInitialization> reconcile(
            final NetworkInitialization resource, final Context<NetworkInitialization> context) throws Exception {
        try {
            log.info("NetworkInitializationReconciler capsules: "
                    + context.getSecondaryResources(ConsensusCapsule.class).size());
            log.info("NetworkInitializationReconciler capsules: "
                    + context.getSecondaryResourcesAsStream(ConsensusCapsule.class)
                            .map(CustomResource::getMetadata)
                            .map(ObjectMeta::getName)
                            .toList());
            log.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
            workflow.reconcile(resource, context);

            final boolean seen = resource.getStatus() != null
                    && resource.getStatus().getObservedGeneration() != null
                    && Objects.equals(
                            resource.getStatus().getObservedGeneration(),
                            resource.getMetadata().getGeneration());

            if (seen && terminalPhases.contains(resource.getStatus().getPhase())) {
                return UpdateControl.noUpdate();
            }

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
                            .withPhase(getPhase(consensusCapsule.getSecondaryResources(resource, context).values()))
                            .endStatus()
                            .build())
                    .rescheduleAfter(Duration.ofSeconds(30));
        } finally {
            log.infof(
                    "Network Initialization - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

    // collect consensus capsule status in status until any fail or all succeed.
    private static NetworkInitializationStatus.Phase getPhase(final Collection<ConsensusCapsule> consensusCapsules) {
        Set<ConsensusCapsuleStatus.Phase> phases = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .filter(Objects::nonNull)
                .map(ConsensusCapsuleStatus::getPhase)
                .filter(Objects::nonNull)
                .collect(Collectors.toSet());

        NetworkInitializationStatus.Phase phase;
        if (phases.contains(ConsensusCapsuleStatus.Phase.Failed)) {
            phase = NetworkInitializationStatus.Phase.Failed;
        } else if (phases.contains(ConsensusCapsuleStatus.Phase.Pending)) {
            phase = NetworkInitializationStatus.Phase.Pending;
        } else if (phases.equals(Set.of(ConsensusCapsuleStatus.Phase.Running))
                || phases.equals(Set.of(ConsensusCapsuleStatus.Phase.Succeeded))) {
            phase = NetworkInitializationStatus.Phase.Succeeded;
        } else {
            phase = NetworkInitializationStatus.Phase.Unknown;
        }
        return phase;
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitialization> context) {
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
