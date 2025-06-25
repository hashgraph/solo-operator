// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

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
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
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

/** CheckConsensusNodeRunningStep checks if the consensus node is ACTIVE and processing transactions. */
public class CheckConsensusNodeReadyStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("VerifyReady", "ConsensusCapsule");
    public static final String SUCCESS_STATUS_MESSAGE = "Consensus nodes are ready: %s";
    public static final String FAIL_STATUS_MESSAGE = "Consensus nodes are not ready: %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    /**
     * Constructor for CheckConsensusNodeReadyStep that initializes the CustomResourceManager.
     *
     * @param customResourceManager The CustomResourceManager instance used to manage Kubernetes resources for consensus
     *     capsules.
     */
    public CheckConsensusNodeReadyStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

            final String timestamp = timestamp();
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
                        .withType(CONDITION_TYPE.type())
                        .withReason(CONDITION_TYPE.reason())
                        .withStatus(ConditionStatus.True)
                        .withMessage(SUCCESS_STATUS_MESSAGE.formatted(phaseMap))
                        .withLastTransitionTime(timestamp)
                        .build());
                log.infof(SUCCESS_STATUS_MESSAGE, phaseMap);
            } else {
                resultingPhase = NetworkOperationStatus.Phase.Failed;
                conditions.add(new ConditionBuilder()
                        .withObservedGeneration(resource.getMetadata().getGeneration())
                        .withType(CONDITION_TYPE.type())
                        .withReason(CONDITION_TYPE.reason())
                        .withStatus(ConditionStatus.False)
                        .withMessage(FAIL_STATUS_MESSAGE.formatted(phaseMap))
                        .withLastTransitionTime(timestamp)
                        .build());
                log.warnf(
                        "%s [mode: %s]",
                        FAIL_STATUS_MESSAGE.formatted(phaseMap),
                        resource.getSpec().getMode());
            }

            return NetworkOperationStepUtil.prepareStatus(status, conditions, resultingPhase);
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    /**
     * Converts a collection of ConsensusCapsule objects to a set of their phases.
     *
     * @param capsules The collection of ConsensusCapsule objects.
     * @return A set of ConsensusCapsuleStatus.Phase representing the phases of the consensus capsules
     */
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

    /**
     * Converts a collection of ConsensusCapsule objects to a map with their names as keys and phases as values.
     *
     * @param capsules The collection of ConsensusCapsule objects.
     * @return A map where the key is the name of the consensus capsule and the value is its phase
     */
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

    /**
     * Checks if the NetworkOperationStatus indicates that the consensus nodes are ready.
     *
     * @param status The current status of the NetworkOperation.
     * @return True if the status indicates readiness; otherwise, false
     */
    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
