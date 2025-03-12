// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static java.util.Objects.nonNull;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import org.jboss.logging.Logger;

/** CheckConsensusNodeRunningStep checks if the consensus node is ACTIVE and processing transactions */
public class CheckConsensusNodeReadyStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ConsensusCapsuleReady";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public CheckConsensusNodeReadyStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            return status;
        }

        final String timestamp = timestamp();
        try {
            // retrieve consensus capsule list and phases
            final String namespace = resource.getMetadata().getNamespace();
            final Collection<ConsensusCapsule> consensusCapsules = customResourceManager.consensusCapsules(
                    namespace, Map.of(Constants.LABEL_K8S_COMPONENT, Constants.COMPONENT_CONSENSUS_NODE));
            final Set<ConsensusCapsuleStatus.Phase> phaseSet = toPhaseSet(consensusCapsules);
            final Map<String, ConsensusCapsuleStatus.Phase> phaseMap = toPhaseMap(consensusCapsules);

            // determine phase and conditions based on the consensus capsule status
            final NetworkOperationStatus.Phase resultingPhase;

            List<Condition> conditions = new ArrayList<>();
            if (Set.of(ConsensusCapsuleStatus.Phase.Running).equals(phaseSet)) {
                resultingPhase = NetworkOperationStatus.Phase.Succeeded;
                conditions.add(new ConditionBuilder()
                        .withObservedGeneration(resource.getMetadata().getGeneration())
                        .withType(CONDITION_TYPE)
                        .withStatus(ConditionStatus.True)
                        .withMessage("Successful with capsule phases: %s".formatted(phaseMap))
                        .withLastTransitionTime(timestamp)
                        .build());
            } else if (Set.of(
                            ConsensusCapsuleStatus.Phase.Pending,
                            ConsensusCapsuleStatus.Phase.Running,
                            ConsensusCapsuleStatus.Phase.Unknown)
                    .containsAll(phaseSet)) {
                resultingPhase = NetworkOperationStatus.Phase.Running;
            } else {
                resultingPhase = NetworkOperationStatus.Phase.Failed;
                conditions.add(new ConditionBuilder()
                        .withObservedGeneration(resource.getMetadata().getGeneration())
                        .withType(CONDITION_TYPE)
                        .withStatus(ConditionStatus.False)
                        .withMessage("Failed with capsule phases: %s".formatted(phaseMap))
                        .withLastTransitionTime(timestamp)
                        .build());
            }

            // return a new status with the phase and conditions
            log.infof(
                    "Consensus capsules status: %s [ %s/%s status: %s ]",
                    phaseMap,
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resultingPhase);
            return status.edit()
                    .withPhase(resultingPhase)
                    .addAllToConditions(conditions)
                    .build();
        } catch (final Exception ex) {
            log.error("Failed to check consensus-capsule readiness", ex);
            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withObservedGeneration(
                                            resource.getMetadata().getGeneration())
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(timestamp)
                                    .withMessage("Failed to check consensus-capsule readiness: %s"
                                            .formatted(ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }
    }

    private Set<ConsensusCapsuleStatus.Phase> toPhaseSet(Collection<ConsensusCapsule> capsules) {
        Set<ConsensusCapsuleStatus.Phase> phases = new HashSet<>();
        for (ConsensusCapsule capsule : capsules) {
            if (Objects.nonNull(capsule.getStatus())
                    && Objects.nonNull(capsule.getStatus().getPhase())) {
                phases.add(capsule.getStatus().getPhase());
            }
        }
        return phases;
    }

    private Map<String, ConsensusCapsuleStatus.Phase> toPhaseMap(Collection<ConsensusCapsule> capsules) {
        Map<String, ConsensusCapsuleStatus.Phase> phaseMap = new LinkedHashMap<>();
        List<ConsensusCapsule> sortedCapsules = new ArrayList<>(capsules);

        // sort the capsules by name for consistent output and make it easier to debug
        sortedCapsules.sort(Comparator.comparing(c -> c.getMetadata().getName()));

        for (ConsensusCapsule capsule : sortedCapsules) {
            if (nonNull(capsule.getStatus()) && nonNull(capsule.getStatus().getPhase())) {
                phaseMap.put(
                        capsule.getMetadata().getName(), capsule.getStatus().getPhase());
            }
        }

        return phaseMap;
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
