// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkinitialization;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;
import org.jboss.logging.Logger;

@ControllerConfiguration(name = "network-initialization")
public class NetworkInitializationReconciler implements Reconciler<NetworkInitialization> {
    private final Logger log;
    private final KubernetesClient client;
    private static final Set<NetworkInitializationStatus.Phase> terminalPhases =
            Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded);

    @Inject
    public NetworkInitializationReconciler(final Logger log, final KubernetesClient client) {
        this.log = log;
        this.client = client;
    }

    @Override
    public UpdateControl<NetworkInitialization> reconcile(
            final NetworkInitialization resource, final Context<NetworkInitialization> context) throws Exception {
        try {
            log.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());

            final boolean seen = resource.getStatus() != null
                    && resource.getStatus().getObservedGeneration() != null
                    && Objects.equals(
                            resource.getStatus().getObservedGeneration(),
                            resource.getMetadata().getGeneration());

            if (seen && terminalPhases.contains(resource.getStatus().getPhase())) {
                return UpdateControl.noUpdate();
            }

            final var consensusCapsuleClient = client.resources(ConsensusCapsule.class)
                    .inNamespace(resource.getMetadata().getNamespace());

            if (!seen) {
                // process new generation by marking all consensus capsules eligible for deployment
                consensusCapsuleClient
                        .resources()
                        .forEach(rc -> rc.edit(cc -> cc.edit()
                                .editSpec()
                                .withEligibleForDeployment(true)
                                .endSpec()
                                .build()));
            }

            return UpdateControl.patchStatus(resource.edit()
                            .editStatus()
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withPhase(getPhase(consensusCapsuleClient.list().getItems()))
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
    private NetworkInitializationStatus.Phase getPhase(final List<ConsensusCapsule> consensusCapsules) {
        Set<ConsensusCapsuleStatus.Phase> phases = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .map(ConsensusCapsuleStatus::getPhase)
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
}
