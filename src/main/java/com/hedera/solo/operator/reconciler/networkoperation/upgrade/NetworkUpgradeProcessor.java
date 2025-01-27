// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.library.processor.ProcessorTask;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.ExecuteImmediateTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeAbortTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeScheduledTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.util.Objects;

/**
 * NetworkUpgradeProcessor processes the network upgrade based on the current workflow step.
 *
 * <p>NetworkUpgrade requires a series of steps to be executed in sequence to synchronize the upgrade of all the nodes
 * in the network. A special FreezeTransaction {@link com.hederahashgraph.api.proto.java.FreezeTransactionBody} is sent
 * to the network with various freezeTypes {@link com.hederahashgraph.api.proto.java.FreezeType} to indicate the type of
 * freeze operation needed across the network simultaneously. In response to the FreezeTransactions, the network node
 * generates marker files at "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current". The presence of these marker
 * files notifies a separate process (an agent running on the node) to initiate various actions for the network upgrade,
 * including the restarts of the network nodes.
 *
 * <p>In the context of solo-operator, there will be two separate components to assist with the network upgrade. A
 * lightweight agent will monitor the file system and detect the creation of the marker files. This agent will then
 * construct an appropriate CR and send it to solo-operator. Solo-operator will perform the actions needed so that the
 * Kubernetes controller can perform the necessary restarts of the network nodes with the new upgrade configuration.
 *
 * <p>Below are the freeze types of FreezeTransaction that are relevant for this processor:
 *
 * <ol>
 *   <li><b>PREPARE_UPGRADE:</b> Indicates the intention of the upcoming network upgrade event time. In response to the
 *       PREPARE_UPGRADE transaction, the network node generates a marker file at
 *       "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current/execute_immediate.mf". This triggers the
 *       ExecuteImmediate step in network upgrade workflow in solo-operator, where it validates various image sources
 *       and selects one for deployment. At this step, the network node is not restarted. The subsequent FREEZE_UPGRADE
 *       FreezeTransaction is usually triggered in around 24 to 48 hours.
 *   <li><b>FREEZE_UPGRADE:</b> Indicates the initiation of freezing the network and upgrading the network nodes. The
 *       network node generates a new marker file at
 *       "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current/freeze_scheduled.mf" and then prepares to go into
 *       a "frozen" state where it no longer processes any new transactions. The marker file triggers the
 *       FreezeScheduled step of the network upgrade workflow in solo-operator, where it re-validates the image sources
 *       and selects one for actual deployment. The network node is not restarted yet at this stage, but it waits for
 *       "now_frozen.mf" to be created by the network node. Once the network node stops processing new transactions
 *       around the selected network freeze time (in ~24 hours), it generates the
 *       "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current/now_frozen.mf" marker file. This triggers the
 *       NowFrozen workflow step in solo-operator, where it updates relevant CRs so that Kubernetes restarts the pods
 *       with the new versions of the network node software.
 *   <li><b>FREEZE_ABORT:</b> If the network upgrade needs to be aborted, a FreezeTransaction is sent with freezeType
 *       set to FREEZE_ABORT, which invalidates the current network upgrade process and requires the flow to begin with
 *       PREPARE_UPGRADE. The network node generates a "freeze_aborted.mf" marker file at
 *       "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current/freeze_aborted.mf". This triggers the FreezeAbort
 *       step in solo-operator so that it can perform any cleanup actions to discard the current upgrade process. In
 *       practice, this is rarely used and only expected to be sent before FREEZE_UPGRADE transaction is sent.
 * </ol>
 *
 * <p>This network upgrade process is represented as a workflow with steps corresponding to each marker file as below:
 *
 * <ol>
 *   <li><b>ExecuteImmediate:</b> Triggered by the "execute_immediate.mf" marker file, where it performs the image
 *       source validation process and prepares for the upcoming network upgrade event (~in 48 hours).
 *   <li><b>FreezeScheduled:</b> Triggered by the "freeze_scheduled.mf" marker file, where it performs revalidation of
 *       image sources and prepares for the imminent network upgrade event.
 *   <li><b>NowFrozen:</b> Triggered by the "now_frozen.mf" marker file, where it performs revalidation of image sources
 *       and updates CRs to trigger restarts of network nodes with new images.
 *   <li><b>FreezeAbort:</b> Triggered by the "freeze_aborted.mf" marker file, where this is essentially a noop
 *       operation since network nodes haven't been impacted (e.g. frozen or upgraded). However, solo-operator may need
 *       to do some housekeeping to discard the current upgrade cycle.
 * </ol>
 *
 * <p>For each of the above workflow step, NetworkUpgradeProcessor delegates to a corresponding task that performs the
 * necessary action steps. The task and associated action steps are defined in the respective task as well as step
 * classes:
 *
 * @see "https://gitmediator.com/swirlds/swirlds-docker/blob/main/docs%2Fdocker-infrastructure-design.md#phase-1-design"
 *     for initial design details on how this network upgrade process is implemented in the network node along with Node
 *     Management Tool(NMT) that acts as the agent to monitor the marker files. Solo-operator is going to replace NMT
 *     and help with the upgrade process in Kubernetes.
 * @see com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.ExecuteImmediateTask
 * @see com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeScheduledTask
 * @see com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask
 * @see com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeAbortTask
 */
@ApplicationScoped
public class NetworkUpgradeProcessor
        implements Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>> {
    @Inject
    ExecuteImmediateTask executeImmediateTask;

    @Inject
    FreezeScheduledTask freezeScheduledTask;

    @Inject
    NowFrozenTask nowFrozenTask;

    @Inject
    FreezeAbortTask freezeAbortTask;

    private void validate(NetworkOperation resource, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "resource must not be null");
        Objects.requireNonNull(resource.getSpec(), "resource.spec must not be null");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "resource.spec.networkUpgrade must not be null");
        Objects.requireNonNull(
                resource.getSpec().getNetworkUpgrade().getWorkflowStep(),
                "resource.spec.networkUpgrade.workflowStep must not be null");
        Objects.requireNonNull(context, "context must not be null");
    }

    private ProcessorTask<NetworkOperation, NetworkOperationStatus> taskFromStep(NetworkUpgrade.WorkflowStep step) {
        return switch (step) {
            case ExecuteImmediate -> executeImmediateTask;
            case FreezeScheduled -> freezeScheduledTask;
            case NowFrozen -> nowFrozenTask;
            case FreezeAbort -> freezeAbortTask;
            default -> throw new ProcessorException("Unknown workflow step: " + step);
        };
    }

    @Override
    public ProcessorResult<NetworkOperationStatus> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {

        this.validate(resource, context);

        final NetworkUpgrade.WorkflowStep step =
                resource.getSpec().getNetworkUpgrade().getWorkflowStep();
        return taskFromStep(step).execute(resource, context);
    }
}
