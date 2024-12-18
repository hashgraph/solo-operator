// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.library.processor.ProcessorTask;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyConsensusNodeChangeset;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyStreamUploaderChangeset;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyConsensusNodeChangeset;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyStreamUploaderChangeset;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/** ExecuteImmediateProcessor processes the ExecuteImmediate step of network upgrade process */
@ApplicationScoped
public class NowFrozenTask implements ProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final long MAX_RETRY = 24;
    public static final Duration RETRY_DELAY = Duration.ofHours(1);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new ReverifyConsensusNodeChangeset(),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangeset(Constants.TPL_VAR_BACKUP_UPLOADER),
                new ApplyConsensusNodeChangeset(),
                new ApplyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ApplyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ApplyStreamUploaderChangeset(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ApplyStreamUploaderChangeset(Constants.TPL_VAR_BACKUP_UPLOADER));
    }
}
