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
import jakarta.annotation.PostConstruct;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.jboss.logging.Logger;

@ControllerConfiguration(name = "network-initialization")
public class NetworkInitializationReconciler implements Reconciler<NetworkInitialization> {
    private final Logger log;
    private final KubernetesClient client;
    private final static Set<NetworkInitializationStatus.Phase> terminalPhases = Set.of(NetworkInitializationStatus.Phase.Failed, NetworkInitializationStatus.Phase.Succeeded);

    // use 1 second interval in testing, 30 second interval otherwise
    @ConfigProperty(name = "quarkus.profile")
    String profile;
    Duration refreshInterval;
    @PostConstruct
    void init() {
        refreshInterval = Duration.ofSeconds("test".equals(profile) ? 1 : 30);
    }

    @Inject
    public NetworkInitializationReconciler(final Logger log, final KubernetesClient client) {
        this.log = log;
        this.client = client;
    }

    /*
    * process new generation by marking all consensus capsules eligible for deployment
    * collect consensus capsule status in status until any fail or all succeed.
    * */

    @Override
    public UpdateControl<NetworkInitialization> reconcile(NetworkInitialization resource, Context<NetworkInitialization> context) throws Exception {
        try {
            log.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());

            boolean seen =
                    resource.getStatus() != null &&
                    resource.getStatus().getObservedGeneration() != null &&
                    Objects.equals(resource.getStatus().getObservedGeneration(), resource.getMetadata().getGeneration());

            if (seen && terminalPhases.contains(resource.getStatus().getPhase())) {
                return UpdateControl.noUpdate();
            }

            var consensusCapsuleClient = client.resources(ConsensusCapsule.class)
                    .inNamespace(resource.getMetadata().getNamespace());

            if (!seen) {
                consensusCapsuleClient.resources().forEach(rc -> rc.edit(cc -> cc.edit()
                        .editSpec().withEligibleForDeployment(true).endSpec().build()));
            }

            return UpdateControl.patchStatus(resource.edit()
                    .editStatus()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withPhase(getPhase(consensusCapsuleClient.list().getItems()))
                    .endStatus()
                    .build()).rescheduleAfter(refreshInterval);
        } finally {
            log.infof(
                    "Network Initialization - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

    private static NetworkInitializationStatus.Phase getPhase(List<ConsensusCapsule> consensusCapsules) {
        Set<ConsensusCapsuleStatus.Phase> phases = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .map(ConsensusCapsuleStatus::getPhase)
                .collect(Collectors.toSet());

        NetworkInitializationStatus.Phase phase;
        if (phases.contains(ConsensusCapsuleStatus.Phase.Failed)) {
            phase = NetworkInitializationStatus.Phase.Failed;
        }
        else if (phases.contains(ConsensusCapsuleStatus.Phase.Running)) {
            phase = NetworkInitializationStatus.Phase.Running;
        }
        else if (phases.contains(ConsensusCapsuleStatus.Phase.Pending)) {
            phase = NetworkInitializationStatus.Phase.Pending;
        }
        else if (phases.equals(Set.of(ConsensusCapsuleStatus.Phase.Succeeded))) {
            phase = NetworkInitializationStatus.Phase.Succeeded;
        }
        else {
            phase = NetworkInitializationStatus.Phase.Unknown;
        }
        return phase;
    }
}
