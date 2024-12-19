// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.library.processor.ProcessorTask;
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
public class NetworkUpgradeProcessor
        implements Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>> {
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

    private ProcessorTask<NetworkOperation, NetworkOperationStatus> taskFromStep(NetworkUpgrade.WorkflowStep step) {
        return switch (step) {
            case ExecuteImmediate -> executeImmediateTask;
            case FreezeScheduled -> freezeScheduledTask;
            case NowFrozen -> nowFrozenTask;
            case FreezeAbort -> freezeAbortTask;
            default -> throw new ProcessorException("Unknown workflow step: " + step);
        };
    }

    @Override
    public ProcessorResult<NetworkOperationStatus> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {

        this.validate(resource, context);

        final NetworkUpgrade.WorkflowStep step =
                resource.getSpec().getNetworkUpgrade().getWorkflowStep();
        return taskFromStep(step).execute(resource, context);
    }
}
