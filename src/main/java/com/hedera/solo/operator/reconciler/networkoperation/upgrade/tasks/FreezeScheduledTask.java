// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyConsensusNodeChangeset;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyStreamUploaderChangeset;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/** ExecuteImmediateProcessor processes the ExecuteImmediate step of network upgrade process */
@ApplicationScoped
public class FreezeScheduledTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofHours(12);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new ReverifyConsensusNodeChangeset(),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_BACKUP_UPLOADER));
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }

    @Override
    protected Processor<?, ?, ?> computeDeferredProcessor(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        // TODO: check status and return appropriate processor for deferred processing
        return null;
    }
}
