// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyConsensusNodeChangesetStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyOtelCollectorStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyStreamUploaderChangesetStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

/**
 * ExecuteImmediateProcessor performs the actions necessary when the network node emits the "execute-immediate.mf"
 * marker file.
 *
 * <p>Context: Upon receiving a PREPARE_UPGRADE FreezeTransaction, the network node emits a marker file at
 * "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current/execute_immediate.mf". This triggers an agent to prepare
 * a NetworkOperation CR and send it to the solo-operator. The CR includes changesets for the consensus node based on
 * the upgrade manifest or archive file (build.zip file with FileID 0.0.150) as well as changesets for stream uploaders.
 * NetworkOperationReconciler then performs the necessary actions using NetworkUpgradeProcessor, which delegates the
 * actual execution to ExecuteImmediateTask. See "NetworkOperationSpec.networkUpgrade.changesets".
 *
 * <p>The ExecuteImmediateTask executes the following action steps:
 *
 * <ol>
 *   <li>Verify the consensus node changeset by checking the image manifest from each of the sources and then select one
 *       randomly for deployment.
 *   <li>Verify the stream uploader changeset for the record stream uploader by checking the image manifest from each of
 *       the sources and select one randomly for deployment.
 *   <li>Verify the stream uploader changeset for the record stream sidecar files uploader, similar to the record stream
 *       uploader.
 *   <li>Verify the stream uploader changeset for the event stream uploader, similar to the record stream uploader.
 *   <li>Verify the stream uploader changeset for the backup uploader, similar to the record stream uploader.
 *   <li>Update the status of the NetworkOperation CR after running each of the above steps.
 * </ol>
 *
 * <p>This task implements a method "computeReconcileDelay" to advise the reconcile delay so that the reconciler may
 * reconcile with an appropriate delay. Since FreezeScheduledTask may not be triggered until 24 hours after the creation
 * of the "execute-immediate.mf" marker file, the reconciler can safely use a reconcile delay that is smaller than 24
 * hours (e.g., 12 hours). Although this is not critical, this larger reconcile delay helps to make the reconciler less
 * overloaded and keeps an option to optimize it further later.
 *
 * <p>This task implements "computeDeferredProcessor" method where it inspects NetworkOperationStatus to see if it has
 * already validated and selected the images for deployment; in that case, it has nothing more to do. So it returns a
 * NoopProcessor instance. If needed, NoopProcessor can also be detected by the reconciler to determine that it should
 * avoid processing the same workflow step repeatedly and take necessary actions. In order to keep the logic simple,
 * there is nothing wrong if this task is called repeatedly, and it returns NoopProcessor accordingly every time.
 */
@ApplicationScoped
public class ExecuteImmediateTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofHours(12);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new VerifyConsensusNodeChangesetStep(),
                new VerifyStreamUploaderChangesetStep(Constants.TPL_VAR_RECORD_STREAM_UPLOADER),
                new VerifyStreamUploaderChangesetStep(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER),
                new VerifyStreamUploaderChangesetStep(Constants.TPL_VAR_EVENT_STREAM_UPLOADER),
                new VerifyStreamUploaderChangesetStep(Constants.TPL_VAR_BACKUP_UPLOADER),
                new VerifyOtelCollectorStep());
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
