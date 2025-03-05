// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyConsensusNodeChangesetStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyOtelCollectorStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ReverifyStreamUploaderChangesetStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/**
 * FreezeScheduledTask performs the actions necessary when the network node emits the "freeze-scheduled.mf" marker file.
 *
 * <p>Since FREEZE_UPGRADE transaction may appear 24-48 hours after the PREPARE_UPGRADE transaction, the selected image
 * source may become invalid during that time period. Therefore, we need to revalidate the images and select a new
 * source if necessary.
 *
 * <p>It performs the following action steps:
 *
 * <ol>
 *   <li>Reverify selected image source for consensus-node and select a new one if the previous one is invalid. Update
 *       {@link NetworkOperationStatus} accordingly
 *   <li>Reverify selected image source for each of the stream-uploader containers and select a new one if the previous
 *       one is invalid. Update {@link NetworkOperationStatus} accordingly
 * </ol>
 *
 * <p>The subsequent now-frozen task will apply the changesets to the network, but it may occur around 12 hours after
 * this. So it returns a reconcile delay of 12 hours to reduce the load on the reconciler.
 *
 * <p>This task returns a NoopProcessor instance in the computeDeferredProcessor method if the status indicates that the
 * images are already validated and selected recently.
 *
 * <p>If valid image source cannot be found, it needs to abort and mark the {@link NetworkOperationStatus} with error
 * condition.
 */
@ApplicationScoped
public class FreezeScheduledTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofHours(12);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new ReverifyConsensusNodeChangesetStep(),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new ReverifyStreamUploaderChangesetStep(Constants.TPL_VAR_BACKUP_UPLOADER),
                new ReverifyOtelCollectorStep());
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }

    @Override
    protected Processor<?, ?, ?> computeDeferredProcessor(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        // TODO: check status and return NoopProcessor if nothing to do. For example, if the images were checked very
        // recently, it doesn't need to reverify those.
        return null;
    }
}
