// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyConsensusPodChangeStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyOrbitConfigurationChangeStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyConsensusNodeChangesetStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyStreamUploaderChangesetStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/**
 * NowFrozenTask executes the network upgrade steps in response to the "now-frozen.mf" marker file
 *
 * <p>It performs the following action steps:
 *
 * <ol>
 *   <li>Reverify the consensus node and stream-uploader changesets and image sources. Check reverification steps of
 *       {@link FreezeScheduledTask}.
 *   <li>Rollout config file bundle as specified in "configurationFiles" field (patch the orbit CR).
 *       {@link NetworkCheckProcessor}] will check the following:
 *       <ol>
 *         <li>Orbit reconciler creates config-maps: settings.txt, application.properties, api-permission.properties,
 *             etc.
 *         <li>Network-operation reconciler creates config.txt.
 *       </ol>
 *   <li>Single atomic patch to consensus-capsule CR using the following changesets. It will be checked by
 *       {@link NetworkCheckProcessor} when deferred.
 *       <ol>
 *         <li>Apply consensus-node changeset (image source, nodeProperties and logging).
 *         <li>Apply streamUploader changesets (e.g., recordStreamUploader, backUploader, etc.).
 *       </ol>
 *   <li>Add a condition to the status field explaining the execution status.
 * </ol>
 *
 * <p>In the {@link NowFrozenTask#computeDeferredProcessor} method, check if it has already patched the CRs and return
 * the {@link NetworkCheckProcessor} to ensure the network is in a valid state.
 */
@ApplicationScoped
public class NowFrozenTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofSeconds(1);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new ReverifyConsensusNodeChangesetStep(),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_BACKUP_UPLOADER),
                new ApplyConsensusPodChangeStep(),
                new ApplyOrbitConfigurationChangeStep());
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
