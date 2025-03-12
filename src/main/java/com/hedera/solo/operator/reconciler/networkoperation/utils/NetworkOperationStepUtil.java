// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import org.jboss.logging.Logger;

public final class NetworkOperationStepUtil {
    private static final Logger log = Logger.getLogger(NetworkOperationStepUtil.class);

    /**
     * Prepare status with condition and phase
     *
     * @param status network operation status
     * @param condition condition
     * @param phase phase
     * @return a new status with condition and phase
     */
    public static NetworkOperationStatus prepareStatus(
            final NetworkOperationStatus status, final Condition condition, final NetworkOperationStatus.Phase phase) {
        Condition existing = status.getConditions().stream()
                .filter(c -> c.getType().equals(condition.getType()))
                .findFirst()
                .orElse(null);

        if (existing != null) {
            existing.setObservedGeneration(condition.getObservedGeneration());
            existing.setLastTransitionTime(condition.getLastTransitionTime());
            existing.setMessage(condition.getMessage());
            existing.setReason(condition.getReason());
            existing.setStatus(condition.getStatus());
            existing.setType(condition.getType());
            return edit(status).with(s -> s.withPhase(phase)).build();
        }

        return edit(status)
                .with(s -> s.addToConditions(condition).withPhase(phase))
                .build();
    }

    public static boolean isDone(final NetworkOperationStatus status, final String conditionType) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(conditionType) && c.getStatus().equals(ConditionStatus.True));
    }

    /**
     * Check if the last condition is false and set the phase to failed
     *
     * @param updated updated network operation status
     * @return updated network operation status with phase Failed if the last condition is false
     */
    public static NetworkOperationStatus checkLastCondition(final NetworkOperationStatus updated) {
        final Condition c = updated.getConditions().getLast(); // assuming the last condition is the most recent
        if (c != null && c.getStatus().equals(ConditionStatus.False)) {
            log.warnf("Network operation failed [ Message = %s ]", c.getMessage());
            return edit(updated)
                    .with(s -> s.withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }

        return updated;
    }
}
