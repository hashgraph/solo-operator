// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import java.util.Objects;
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

        return prepareStatus(status, condition, NetworkOperationStatus.Phase.Failed);
    }

    /**
     * Check and throw exception if the last condition is false
     *
     * <p>Assuming last condition is the most recent
     *
     * @param updated updated network operation status
     * @return network operation status if the last condition is not false
     */
    public static NetworkOperationStatus checkLastCondition(final NetworkOperationStatus updated) {
        final Condition c = updated.getConditions().getLast();
        if (c != null && c.getStatus().equals(ConditionStatus.False)) {
            throw new ProcessorException(c.getMessage());
        }

        return updated;
    }
}
