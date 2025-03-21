// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

public final class NetworkOperationStepUtil {
    private static final Logger log = Logger.getLogger(NetworkOperationStepUtil.class);

    /**
     * Prepare a status with conditions and phase
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
        return prepareStatus(status, conditions, NetworkOperationStatus.Phase.Pending);
    }

    /**
     * Prepare failed status
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
            final String conditionType) {

        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(message, "Failed message is required");
        Objects.requireNonNull(conditionType, "Condition type is required");

        final Condition condition = new ConditionBuilder()
                .withObservedGeneration(resource.getMetadata().getGeneration())
                .withType(conditionType)
                .withStatus(ConditionStatus.False)
                .withLastTransitionTime(timestamp())
                .withMessage(message)
                .build();

        return prepareStatus(status, List.of(condition), NetworkOperationStatus.Phase.Failed);
    }

    /**
     * Clone and update status with specified condition and phase
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
                if (c.getType().equals(condition.getType())) {
                    log.debugf("Condition %s already exists, updating", condition.getType());
                    c.setObservedGeneration(condition.getObservedGeneration());
                    c.setLastTransitionTime(condition.getLastTransitionTime());
                    c.setMessage(condition.getMessage());
                    c.setReason(condition.getReason());
                    c.setStatus(condition.getStatus());
                    c.setType(condition.getType());
                    found = true;
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

    public static boolean isSuccessful(final NetworkOperationStatus status, final String conditionType) {
        if (status == null
                || status.getConditions() == null
                || status.getConditions().isEmpty()
                || status.getPhase() == NetworkOperationStatus.Phase.Failed) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(conditionType) && c.getStatus().equals(ConditionStatus.True));
    }

    /**
     * Return a status with Failed or Succeeded phase based on the last condition
     *
     * @param updated network operation status
     * @return network operation status with correct phase
     */
    public static NetworkOperationStatus checkLastCondition(final NetworkOperationStatus updated) {
        final Condition c = updated.getConditions().getLast();
        if (c != null && !c.getStatus().equals(ConditionStatus.True)) {
            return NetworkOperationStepUtil.prepareStatus(updated, List.of(), NetworkOperationStatus.Phase.Failed);
        }

        return prepareStatus(updated, List.of());
    }
}
