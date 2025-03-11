// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.Condition;

public final class NetworkOperationUtil {
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
}
