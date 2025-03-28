// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.cleanupEvents;
import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.createOrUpdateEvent;
import static com.hedera.solo.operator.library.validate.ManifestValidation.validate;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_NAME;

import com.hedera.solo.operator.library.generic.Compare;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Secrets;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleConfigMap;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleDeployment;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsulePod;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleRoleBinding;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleServiceAccount;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleTelemetryConfig;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleEventsStreamPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleLogsPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleRecordStreamPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleStatesPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleStatsPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleUpgradePvc;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ContainerState;
import io.fabric8.kubernetes.api.model.ContainerStatus;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.reconciler.Cleaner;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Workflow;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import java.time.ZonedDateTime;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;
import java.util.function.Consumer;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import org.jboss.logging.Logger;

@Workflow(
        dependents = {
            @Dependent(name = "sa", type = CapsuleServiceAccount.class),
            @Dependent(type = CapsuleRoleBinding.class, dependsOn = "sa"),
            @Dependent(type = CapsuleService.class),
            @Dependent(type = CapsuleConfigMap.class),
            @Dependent(type = CapsuleTelemetryConfig.class, reconcilePrecondition = CapsuleTelemetryConfig.Ready.class),
            @Dependent(type = CapsuleLogsPvc.class, reconcilePrecondition = CapsuleLogsPvc.Ready.class),
            @Dependent(type = CapsuleStatsPvc.class, reconcilePrecondition = CapsuleStatsPvc.Ready.class),
            @Dependent(type = CapsuleStatesPvc.class, reconcilePrecondition = CapsuleStatesPvc.Ready.class),
            @Dependent(type = CapsuleUpgradePvc.class, reconcilePrecondition = CapsuleUpgradePvc.Ready.class),
            @Dependent(type = CapsuleRecordStreamPvc.class, reconcilePrecondition = CapsuleRecordStreamPvc.Ready.class),
            @Dependent(type = CapsuleEventsStreamPvc.class, reconcilePrecondition = CapsuleEventsStreamPvc.Ready.class),
            @Dependent(type = CapsuleDeployment.class),
            @Dependent(type = CapsulePod.class)
        })
@ControllerConfiguration(name = "consensus-capsule")
public class ConsensusCapsuleReconciler implements Cleaner<ConsensusCapsule>, Reconciler<ConsensusCapsule> {
    public static final Predicate<HasMetadata> HAS_LABEL_SELECTOR_CONSENSUS_NODE =
            hasLabel(Constants.LABEL_APP, Constants.COMPONENT_CONSENSUS_NODE);
    public static final String CONSENSUS_CONTAINER_NAME = "consensus-node";

    private static final Logger log = Logger.getLogger(ConsensusCapsuleReconciler.class);

    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context) throws Exception {
        try {
            log.infof(
                    "Consensus Capsule - Starting Reconciliation [ name = '%s', namespace = '%s' ]",
                    capsule.getMetadata().getName(), capsule.getMetadata().getNamespace());

            final Pod[] pods = context.getSecondaryResources(Pod.class).toArray(Pod[]::new);
            final ConsensusCapsuleStatus.Phase phase =
                    phase(findConsensusPod(capsule, context).orElse(null));
            log.infof(
                    "Consensus Capsule - Phase [ name = '%s', namespace = '%s', phase = '%s' ]",
                    capsule.getMetadata().getName(), capsule.getMetadata().getNamespace(), phase);

            final List<Condition> conditions = Optional.ofNullable(capsule.getStatus())
                    .map(ConsensusCapsuleStatus::getConditions)
                    .orElse(null);

            final ConsensusCapsule desired = edit(new ConsensusCapsule())
                    .with(metadata -> metadata.editMetadata()
                            .withName(capsule.getMetadata().getName())
                            .withNamespace(capsule.getMetadata().getNamespace()))
                    .with(status -> status.editStatus()
                            .withObservedGeneration(capsule.getMetadata().getGeneration())
                            .withPhase(phase)
                            .withConditions(validate(capsule, conditions, validateSecrets(capsule))))
                    .build();

            // if possible, avoid updating capsule
            if (Compare.allMatch(
                    capsule,
                    desired,
                    o -> o.getStatus() != null ? o.getStatus().getObservedGeneration() : null,
                    o -> o.getStatus() != null ? o.getStatus().getPhase() : null)) {
                return UpdateControl.noUpdate();
            }

            return UpdateControl.patchStatus(desired);
        } finally {
            log.infof(
                    "Consensus Capsule - Finished Reconciliation [ name = '%s' namespace = '%s' ]",
                    capsule.getMetadata().getName(), capsule.getMetadata().getNamespace());
        }
    }

    /**
     * Find pod with latest creation timestamp for the consensus capsule
     *
     * @param capsule the consensus capsule
     * @param context the context
     * @return the latest pod for the consensus capsule
     */
    private Optional<Pod> findConsensusPod(ConsensusCapsule capsule, Context<ConsensusCapsule> context) {
        String nodeName = Metadata.prepareNodeName(capsule.getSpec().getNodeId());
        return context.getSecondaryResources(Pod.class).stream()
                .filter(pod -> nodeName.equals(pod.getMetadata().getLabels().get(LABEL_NODE_NAME)))
                .max(Comparator.comparing(
                        pod -> ZonedDateTime.parse(pod.getMetadata().getCreationTimestamp())));
    }

    public static ConsensusCapsuleStatus.Phase phase(final Pod... pods) {
        ConsensusCapsuleStatus.Phase phase = ConsensusCapsuleStatus.Phase.Unknown;

        if (1 == pods.length && pods[0] != null) {
            final Pod pod = pods[0];
            final ContainerStatus containerStatus = pod.getStatus().getContainerStatuses().stream()
                    .collect(Collectors.toMap(ContainerStatus::getName, Function.identity()))
                    .get(CONSENSUS_CONTAINER_NAME);

            if (containerStatus != null) {
                log.infof(
                        "Consensus Capsule - Phase [ name = '%s' containerStatus.phase = '%s ]",
                        pod.getMetadata().getName(), containerStatus.getState().getRunning());
                final ContainerState state = containerStatus.getState();

                // if terminated already, return Succeeded or Failed based on exit code
                if (state.getTerminated() != null) {
                    phase = state.getTerminated().getExitCode() == 0
                            ? ConsensusCapsuleStatus.Phase.Succeeded
                            : ConsensusCapsuleStatus.Phase.Failed;
                    return phase;
                }

                // if it is running, return Running if all containers are ready, otherwise return Pending
                if (state.getRunning() != null) {
                    phase = containerStatus.getReady()
                            ? ConsensusCapsuleStatus.Phase.Running
                            : ConsensusCapsuleStatus.Phase.Pending;

                    // check if all sidecar containers are also ready
                    if (phase.equals(ConsensusCapsuleStatus.Phase.Running)) {
                        log.infof("Consensus container '%s' is ready", containerStatus.getName());
                        for (final ContainerStatus cs : pod.getStatus().getContainerStatuses()) {
                            if (!cs.getName().equals(CONSENSUS_CONTAINER_NAME) && !cs.getReady()) {
                                log.infof("Sidecar container '%s' is not ready yet", cs.getName());
                                phase = ConsensusCapsuleStatus.Phase.Pending;
                                return phase;
                            }
                        }
                    }
                }
            }
        }

        return phase;
    }

    Consumer<Condition> validateSecrets(final ConsensusCapsule capsule) {
        return condition -> {
            Secrets secrets = capsule.getSpec().getSecrets();
            if (secrets != null && secrets.getHostPath() != null && secrets.getSecretResources() != null) {
                condition.setMessage("Consensus Capsule may not define more then one type of secret");
                condition.setReason("SpecSecrets");
                condition.setStatus(ConditionStatus.False);
            }
        };
    }

    @Override
    public ErrorStatusUpdateControl<ConsensusCapsule> updateErrorStatus(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context, final Exception e) {
        log.error(
                "Error in reconciliation '%s/%s': %s"
                        .formatted(
                                capsule.getMetadata().getNamespace(),
                                capsule.getMetadata().getName(),
                                e.getMessage()),
                e);
        createOrUpdateEvent(capsule, context, e);
        return ErrorStatusUpdateControl.noStatusUpdate();
    }

    @Override
    public DeleteControl cleanup(final ConsensusCapsule resource, final Context<ConsensusCapsule> context) {
        log.info("Deleting consensus capsule: %s/%s "
                .formatted(
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName()));
        cleanupEvents(resource, context);
        return DeleteControl.defaultDelete();
    }
}
