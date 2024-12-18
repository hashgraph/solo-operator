// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.ExecuteImmediateTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeAbortTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeScheduledTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.util.Objects;

@ApplicationScoped
public class NetworkUpgradeProcessor implements Processor<NetworkOperation, ProcessorResult> {
    @Inject
    ExecuteImmediateTask executeImmediateTask;

    @Inject
    FreezeScheduledTask freezeScheduledTask;

    @Inject
    NowFrozenTask nowFrozenTask;

    @Inject
    FreezeAbortTask freezeAbortTask;

    private void validate(NetworkOperation resource, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "resource must not be null");
        Objects.requireNonNull(resource.getSpec(), "resource.spec must not be null");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "resource.spec.networkUpgrade must not be null");
        Objects.requireNonNull(
                resource.getSpec().getNetworkUpgrade().getWorkflowStep(),
                "resource.spec.networkUpgrade.workflowStep must not be null");
        Objects.requireNonNull(context, "context must not be null");
    }

    @Override
    public ProcessorResult process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {

        this.validate(resource, context);

        final NetworkOperationStatus status;
        final NetworkUpgrade.WorkflowStep step =
                resource.getSpec().getNetworkUpgrade().getWorkflowStep();

        return switch (step) {
            case ExecuteImmediate -> {
                status = executeImmediateTask.execute(resource, context);
                yield new ProcessorResult(status, ExecuteImmediateTask.RETRY_DELAY, ExecuteImmediateTask.MAX_RETRY);
            }
            case FreezeScheduled -> {
                status = freezeScheduledTask.execute(resource, context);
                yield new ProcessorResult(status, FreezeScheduledTask.RETRY_DELAY, FreezeScheduledTask.MAX_RETRY);
            }
            case NowFrozen -> {
                status = nowFrozenTask.execute(resource, context);
                yield new ProcessorResult(status, NowFrozenTask.RETRY_DELAY, NowFrozenTask.MAX_RETRY);
            }
            case FreezeAbort -> {
                status = freezeAbortTask.execute(resource, context);
                yield ProcessorResult.noRetry(status);
            }
        };
    }
}
