// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import java.time.Duration;
import java.time.Instant;
import java.util.Objects;

public class SequenceReconciler
        extends AbstractSequenceReconciler<NetworkOperation, NetworkOperationSpec, NetworkOperationStatus> {
    @Override
    public String getName() {
        return getClass().getName().toLowerCase();
    }

    @Override
    public String getOrbitName(NetworkOperation resource) {
        Objects.requireNonNull(resource, "resource must not be null");
        Objects.requireNonNull(resource.getSpec(), "spec must not be null");
        return resource.getSpec().getOrbit();
    }

    @Override
    public Long getSequenceNumber(NetworkOperation resource) {
        Objects.requireNonNull(resource, "resource must not be null");
        if (Objects.nonNull(resource.getStatus())) {
            return resource.getStatus().getSequenceNumber();
        }
        return null;
    }

    @Override
    public NetworkOperation setSequenceNumber(
            NetworkOperation resource, Context<NetworkOperation> context, Long sequenceNumber) {
        Objects.requireNonNull(resource, "resource must not be null");
        Objects.requireNonNull(resource.getStatus(), "status must not be null");
        Objects.requireNonNull(context, "context must not be null");
        Objects.requireNonNull(sequenceNumber, "sequenceNumber must not be null");
        return resource.edit()
                .editStatus()
                .withSequenceNumber(sequenceNumber)
                .withPhase(NetworkOperationStatus.Phase.Pending)
                .endStatus()
                .build();
    }

    @Override
    protected Duration computeReconcileDelay(
            NetworkOperation resource, ProcessorResult<NetworkOperationStatus> result) {
        return Duration.ofSeconds(1);
    }

    @Override
    protected UpdateControl<NetworkOperation> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws Exception {
        final Condition cond = new ConditionBuilder()
                .withMessage("Reconciled: %s".formatted(Instant.now().toString()))
                .build();
        return UpdateControl.patchStatus(resource.edit()
                .editStatus()
                .withPhase(NetworkOperationStatus.Phase.Succeeded)
                .withConditions(cond)
                .endStatus()
                .build());
    }
}
