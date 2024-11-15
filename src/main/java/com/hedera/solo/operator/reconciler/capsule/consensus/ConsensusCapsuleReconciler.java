// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus;

import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE_KEY;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE_VALUE;

import com.hedera.solo.operator.library.generic.Compare;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsulePod;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleRoleBinding;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleServiceAccount;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.ConsensusCapsuleGossipKeys;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.ConsensusCapsuleGrpcTlsKeys;
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
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import io.quarkus.qute.Engine;
import jakarta.inject.Inject;
import java.util.Map;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import org.jboss.logging.Logger;

@ControllerConfiguration(name = "consensus-capsule")
public class ConsensusCapsuleReconciler implements EventSourceInitializer<ConsensusCapsule>, Reconciler<ConsensusCapsule>, ErrorStatusHandler<ConsensusCapsule> {

    public static final Predicate<HasMetadata> HAS_LABEL_SELECTOR_CONSENSUS_NODE =
            hasLabel(LABEL_SELECTOR_CONSENSUS_NODE_KEY, LABEL_SELECTOR_CONSENSUS_NODE_VALUE);
    public static final String CONSENSUS_CONTAINER_NAME = "consensus-node";
    private final Logger log;
    final CapsuleServiceAccount capsuleServiceAccount = new CapsuleServiceAccount();
    final CapsuleRoleBinding capsuleRoleBinding = new CapsuleRoleBinding();
    final CapsuleService capsuleService = new CapsuleService();
    final CapsuleLogsPvc capsuleLogsPvc = new CapsuleLogsPvc();
    final CapsuleStatsPvc capsuleStatsPvc = new CapsuleStatsPvc();
    final CapsuleStatesPvc capsuleStatesPvc = new CapsuleStatesPvc();
    final CapsuleUpgradePvc capsuleUpgradePvc = new CapsuleUpgradePvc();
    final CapsuleRecordStreamPvc capsuleRecordStreamPvc = new CapsuleRecordStreamPvc();
    final CapsuleEventsStreamPvc capsuleEventsStreamPvc = new CapsuleEventsStreamPvc();
    final CapsulePod capsulePod;
    final ConsensusCapsuleGossipKeys consensusCapsuleGossipKeys = new ConsensusCapsuleGossipKeys();
    final ConsensusCapsuleGrpcTlsKeys consensusCapsuleGrpcTlsKeys = new ConsensusCapsuleGrpcTlsKeys();
    private final Workflow<ConsensusCapsule> workflow;

    @Inject
    public ConsensusCapsuleReconciler(final Logger log, final Engine engine) {
        this.log = log;
        this.capsulePod = new CapsulePod(engine);
        workflow = new WorkflowBuilder<ConsensusCapsule>()
                .addDependentResource(capsuleServiceAccount)
                .addDependentResource(capsuleRoleBinding)
                .addDependentResource(capsuleService)
                .addDependentResource(capsuleLogsPvc).withReconcilePrecondition(new CapsuleLogsPvc.Ready())
                .addDependentResource(capsuleStatsPvc).withReconcilePrecondition(new CapsuleStatsPvc.Ready())
                .addDependentResource(capsuleStatesPvc).withReconcilePrecondition(new CapsuleStatesPvc.Ready())
                .addDependentResource(capsuleUpgradePvc).withReconcilePrecondition(new CapsuleUpgradePvc.Ready())
                .addDependentResource(capsuleRecordStreamPvc).withReconcilePrecondition(new CapsuleRecordStreamPvc.Ready())
                .addDependentResource(capsuleEventsStreamPvc).withReconcilePrecondition(new CapsuleEventsStreamPvc.Ready())
                .addDependentResource(capsulePod).withReconcilePrecondition(ConsensusCapsuleReconciler::isEligibleForDeployment)
                .addDependentResource(consensusCapsuleGossipKeys).withReconcilePrecondition(ConsensusCapsuleReconciler::isEligibleForDeployment)
                .addDependentResource(consensusCapsuleGrpcTlsKeys).withReconcilePrecondition(ConsensusCapsuleReconciler::isEligibleForDeployment)
                .build();
    }

    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context) throws Exception {
        try {
            log.infof(
                    "Consensus Capsule - Starting Reconciliation [ name = '%s' ]",
                    capsule.getMetadata().getName());

            workflow.reconcile(capsule, context);

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

    private static boolean isEligibleForDeployment(DependentResource<?, ?> resource, HasMetadata primary, Context<?> context) {
        return primary instanceof ConsensusCapsule consensusCapsule && consensusCapsule.getSpec().getEligibleForDeployment();
    }

    @Override
    public Map<String, EventSource> prepareEventSources(EventSourceContext<ConsensusCapsule> context) {
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
