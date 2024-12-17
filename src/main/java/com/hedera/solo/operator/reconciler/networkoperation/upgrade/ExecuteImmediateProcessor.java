// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyConsensusNodeChange;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyStreamUploaderChange;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.util.List;
import java.util.Objects;

/** ExecuteImmediateProcessor processes the ExecuteImmediate step of network upgrade process */
@ApplicationScoped
public class ExecuteImmediateProcessor extends AbstractUpgradeProcessor {
    private static final String processorName = "ExecuteImmediate";

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
                new VerifyConsensusNodeChange(),
                new VerifyStreamUploaderChange(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new VerifyStreamUploaderChange(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new VerifyStreamUploaderChange(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new VerifyStreamUploaderChange(Constants.TPL_VAR_BACKUP_UPLOADER));
    }
}
