// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyConsensusNodeChange;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyStreamUploaderChange;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyConsensusNodeChange;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyStreamUploaderChange;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.List;
import java.util.Objects;

public class NowFrozenProcessor extends AbstractUpgradeProcessor {
    private static final String processorName = "NowFrozen";

    @Override
    public String getProcessorName() {
        return processorName;
    }

    @Override
    public List<ProcessorStep<NetworkOperation>> prepareSteps(
            NetworkOperation primary, Context<NetworkOperation> context) throws ProcessorException {
        Objects.requireNonNull(primary, "Primary object must not be null");
        Objects.requireNonNull(context, "Context must not be null");

        return List.of(
                new ReverifyConsensusNodeChange(),
                new ReverifyStreamUploaderChange(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ReverifyStreamUploaderChange(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ReverifyStreamUploaderChange(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ReverifyStreamUploaderChange(Constants.TPL_VAR_BACKUP_UPLOADER),
                new ApplyConsensusNodeChange(),
                new ApplyStreamUploaderChange(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ApplyStreamUploaderChange(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ApplyStreamUploaderChange(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ApplyStreamUploaderChange(Constants.TPL_VAR_BACKUP_UPLOADER));
    }
}
