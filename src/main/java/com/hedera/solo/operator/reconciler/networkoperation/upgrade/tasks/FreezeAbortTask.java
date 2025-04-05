// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.FreezeAbortStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/**
 * FreezeAbortTask aborts network upgrade process in response to the "freeze-aborted.mf" marker file
 *
 * <p>If we received a {@link NetworkOperation} CR previously and processed {@link ExecuteImmediateTask} earlier, this
 * steps needs to update the {@link NetworkOperationStatus} with a status saying that network upgrade was aborted.
 */
@ApplicationScoped
public class FreezeAbortTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ZERO;

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(new FreezeAbortStep()); // define a lambda or step to encapsulate the logic
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }

    @Override
    protected Processor<?, ?, ?> computeDeferredProcessor(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        // TODO: check status and return NoopProcessor if nothing to do.
        return null;
    }
}
