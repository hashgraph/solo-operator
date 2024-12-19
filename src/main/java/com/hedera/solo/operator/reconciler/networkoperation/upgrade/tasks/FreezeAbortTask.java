// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/** ExecuteImmediateProcessor processes the ExecuteImmediate step of network upgrade process */
@ApplicationScoped
public class FreezeAbortTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofHours(12);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(); // FIXME: no-op?
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }
}
