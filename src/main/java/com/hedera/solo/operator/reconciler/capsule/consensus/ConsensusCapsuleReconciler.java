package com.hedera.solo.operator.reconciler.capsule.consensus;

import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE_KEY;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE_VALUE;

import com.hedera.solo.operator.library.generic.Compare;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsulePod;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleRoleBinding;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleServiceAccount;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleEventsStreamPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleLogsPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleRecordStreamPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleStatesPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleStatsPvc;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsuleUpgradePvc;
import io.fabric8.kubernetes.api.model.ContainerState;
import io.fabric8.kubernetes.api.model.ContainerStatus;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import jakarta.inject.Inject;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import org.jboss.logging.Logger;

@ControllerConfiguration(
        name = "consensus-capsule",
        dependents = {
            @Dependent(name = "sa", type = CapsuleServiceAccount.class),
            @Dependent(type = CapsuleRoleBinding.class, dependsOn = "sa"),
            @Dependent(type = CapsuleService.class),
            @Dependent(type = CapsuleLogsPvc.class, reconcilePrecondition = CapsuleLogsPvc.Ready.class),
            @Dependent(type = CapsuleStatsPvc.class, reconcilePrecondition = CapsuleStatsPvc.Ready.class),
            @Dependent(type = CapsuleStatesPvc.class, reconcilePrecondition = CapsuleStatesPvc.Ready.class),
            @Dependent(type = CapsuleUpgradePvc.class, reconcilePrecondition = CapsuleUpgradePvc.Ready.class),
            @Dependent(type = CapsuleRecordStreamPvc.class, reconcilePrecondition = CapsuleRecordStreamPvc.Ready.class),
            @Dependent(type = CapsuleEventsStreamPvc.class, reconcilePrecondition = CapsuleEventsStreamPvc.Ready.class),
            @Dependent(type = CapsulePod.class, reconcilePrecondition = CapsulePod.Ready.class)
        })
public class ConsensusCapsuleReconciler implements Reconciler<ConsensusCapsule>, ErrorStatusHandler<ConsensusCapsule> {

    public static final Predicate<HasMetadata> HAS_LABEL_SELECTOR_CONSENSUS_NODE =
            hasLabel(LABEL_SELECTOR_CONSENSUS_NODE_KEY, LABEL_SELECTOR_CONSENSUS_NODE_VALUE);
    public static final String CONSENSUS_CONTAINER_NAME = "consensus-node";
    private final Logger log;

    @Inject
    public ConsensusCapsuleReconciler(final Logger log){this.log = log;}

    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context) throws Exception {
        try {
            log.infof(
                    "Consensus Capsule - Starting Reconciliation [ name = '%s' ]",
                    capsule.getMetadata().getName());

            ConsensusCapsuleStatus.Phase phase =
                    phase(context.getSecondaryResources(Pod.class).toArray(Pod[]::new));

            ConsensusCapsule desired = capsule.edit()
                    .editStatus()
                    .withObservedGeneration(capsule.getMetadata().getGeneration())
                    .withPhase(phase)
                    .endStatus()
                    .build();

            try {
                if (Compare.allMatch(capsule, desired, o -> o.getStatus().getObservedGeneration(), o -> o.getStatus()
                        .getPhase())) {
                    return UpdateControl.noUpdate();
                }
            } catch (NullPointerException e) {
            }
            return UpdateControl.patchStatus(desired);
        } finally {
            log.infof(
                    "Consensus Capsule - Finished Reconciliation [ name = '%s' ]",
                    capsule.getMetadata().getName());
        }
    }

    public static ConsensusCapsuleStatus.Phase phase(final Pod... pods) {
        ConsensusCapsuleStatus.Phase phase = ConsensusCapsuleStatus.Phase.Unknown;
        if (1 == pods.length) {
            ContainerStatus containerStatus = pods[0].getStatus().getContainerStatuses().stream()
                    .collect(Collectors.toMap(ContainerStatus::getName, Function.identity()))
                    .get(CONSENSUS_CONTAINER_NAME);
            if (containerStatus != null) {
                ContainerState state = containerStatus.getState();
                if (state.getRunning() != null) {
                    phase = containerStatus.getReady()
                            ? ConsensusCapsuleStatus.Phase.Running
                            : ConsensusCapsuleStatus.Phase.Pending;
                }
                if (state.getTerminated() != null) {
                    phase = state.getTerminated().getExitCode() == 0
                            ? ConsensusCapsuleStatus.Phase.Succeeded
                            : ConsensusCapsuleStatus.Phase.Failed;
                }
            }
        }
        return phase;
    }

    @Override
    public ErrorStatusUpdateControl<ConsensusCapsule> updateErrorStatus(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context, final Exception e) {
        return ErrorStatusUpdateControl.noStatusUpdate();
    }
}
