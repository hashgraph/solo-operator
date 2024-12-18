// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.library.processor.ProcessorTask;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyConsensusNodeChangeset;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyStreamUploaderChangeset;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/** ExecuteImmediateProcessor processes the ExecuteImmediate step of network upgrade process */
@ApplicationScoped
public class ExecuteImmediateTask implements ProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RETRY_DELAY = Duration.ofHours(1);
    public static final long MAX_RETRY = 24;

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new VerifyConsensusNodeChangeset(),
                new VerifyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new VerifyStreamUploaderChangeset(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new VerifyStreamUploaderChangeset(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new VerifyStreamUploaderChangeset(Constants.TPL_VAR_BACKUP_UPLOADER));
    }
}
