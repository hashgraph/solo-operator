// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

/** Utility class providing methods to prepare and check the status of a NetworkOperation. */
public final class NetworkOperationStepUtil {
    private static final Logger log = Logger.getLogger(NetworkOperationStepUtil.class);

    /** Private constructor to prevent instantiation of this utility class. */
    private NetworkOperationStepUtil() {}

    /**
     * Prepare a status with conditions and phase.
     *
     * <p>This method must be used by intermediate steps to return the updated status with Pending status
     *
     * <p>A terminal step that sets "Succeeded" or "Failed" should use prepareStatus or prepareFailed methods
     *
     * @param status network operation status
     * @param conditions list of conditions
     * @return updated status
     */
    public static NetworkOperationStatus prepareStatus(
            final NetworkOperationStatus status, List<Condition> conditions) {
        return prepareStatus(status, conditions, NetworkOperationStatus.Phase.Running);
    }

    /**
     * Prepares failed status.
     *
     * @param resource network operation resource
     * @param status network operation status
     * @param message failed message
     * @param conditionType condition type
     * @return a new status with success condition and phase
     */
    public static NetworkOperationStatus prepareFailed(
            NetworkOperation resource,
            NetworkOperationStatus status,
            final String message,
            final StepConditionType conditionType) {

        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(conditionType, "Condition type is required");

        final Condition condition = new ConditionBuilder()
                .withObservedGeneration(resource.getMetadata().getGeneration())
                .withType(conditionType.type())
                .withReason(conditionType.reason())
                .withStatus(ConditionStatus.False)
                .withLastTransitionTime(timestamp())
                .withMessage(message != null ? message : "Network operation failed")
                .build();

        return prepareStatus(status, List.of(condition), NetworkOperationStatus.Phase.Failed);
    }

    /**
     * Clone and update status with specified condition and phase.
     *
     * @param status network operation status
     * @param conditions list of conditions
     * @param phase phase
     * @return a new status with condition and phase
     */
    public static NetworkOperationStatus prepareStatus(
            final NetworkOperationStatus status,
            final List<Condition> conditions,
            final NetworkOperationStatus.Phase phase) {
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(phase, "NetworkOperationStatus phase is required");

        final List<Condition> finalConditions =
                status.getConditions() != null ? status.getConditions() : new ArrayList<>();
        for (final Condition condition : conditions) {
            boolean found = false;
            for (final Condition c : finalConditions) {
                if (c.getType().equals(condition.getType()) && c.getReason().equals(condition.getReason())) {
                    found = true;

                    log.debugf(
                            "Found existing condition '%s-%s'  [ status: %s ] ",
                            c.getType(), c.getReason(), c.getStatus());

                    boolean isChanged = false;
                    if (Objects.nonNull(condition.getStatus())
                            && !condition.getStatus().equals(c.getStatus())) {
                        c.setStatus(condition.getStatus());
                        isChanged = true;
                    }

                    if (Objects.nonNull(condition.getMessage())
                            && !condition.getMessage().equals(c.getMessage())) {
                        c.setMessage(condition.getMessage());
                        isChanged = true;
                    }

                    if (Objects.nonNull(condition.getObservedGeneration())
                            && !condition.getObservedGeneration().equals(c.getObservedGeneration())) {
                        c.setObservedGeneration(condition.getObservedGeneration());
                        isChanged = true;
                    }

                    if (isChanged) {
                        log.debugf(
                                "Updated condition '%s-%s' [ status: %s ] ", c.getType(), c.getReason(), c.getStatus());
                        if (Objects.nonNull(condition.getLastTransitionTime())) {
                            c.setLastTransitionTime(condition.getLastTransitionTime());
                        } else {
                            c.setLastTransitionTime(timestamp());
                        }
                    }
                }
            }

            if (!found) {
                finalConditions.add(condition);
            }
        }

        return edit(status)
                .with(s -> s.withConditions(finalConditions).withPhase(phase))
                .build();
    }

    /**
     * Checks if the status is successful based on the given condition type.
     *
     * @param status NetworkOperation status
     * @param conditionType Condition type to check for success
     * @return True if the status indicates success, false otherwise
     */
    public static boolean isSuccessful(final NetworkOperationStatus status, final StepConditionType conditionType) {
        if (status == null
                || status.getConditions() == null
                || status.getConditions().isEmpty()
                || status.getPhase() == NetworkOperationStatus.Phase.Failed) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(c -> c.getType().equals(conditionType.type())
                        && c.getReason().equals(conditionType.reason())
                        && c.getStatus().equals(ConditionStatus.True));
    }

    /**
     * Checks if the network upgrade mode is enabled for a given NetworkOperation resource.
     *
     * @param resource NetworkOperation resource to check
     * @return True if the mode is NetworkUpgrade, false otherwise
     */
    public static boolean isNetworkUpgrade(final NetworkOperation resource) {
        return resource != null
                && resource.getSpec() != null
                && resource.getSpec().getMode() != null
                && resource.getSpec().getMode().equals(NetworkOperationSpec.Mode.NetworkUpgrade)
                && resource.getSpec().getNetworkUpgrade() != null;
    }
}
