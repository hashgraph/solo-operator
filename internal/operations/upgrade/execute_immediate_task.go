// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
)

// ExecuteImmediateTask performs the actions when the network node emits the "execute_immediate.mf" marker file.
//
// Upon receiving a PREPARE_UPGRADE FreezeTransaction, the network node emits this marker file.
// This triggers validation of:
// 1. Consensus node changeset - image manifest verification
// 2. Stream uploader changesets - record, sidecar, events, backup uploaders
// 3. OpenTelemetry collector changeset
//
// The reconciler uses a 1-hour delay after completion since FreezeScheduled may not be triggered
// for up to 24-48 hours.
type ExecuteImmediateTask struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *UpgradeStatus
}

// NewExecuteImmediateTask creates a new ExecuteImmediateTask
func NewExecuteImmediateTask(c client.Client) *ExecuteImmediateTask {
	return &ExecuteImmediateTask{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *UpgradeStatus]{
			NewVerifyConsensusNodeChangesetStep(c),
			NewVerifyStreamUploaderChangesetStep(c, StreamUploaderTypeRecordStream),
			NewVerifyStreamUploaderChangesetStep(c, StreamUploaderTypeSidecarFiles),
			NewVerifyStreamUploaderChangesetStep(c, StreamUploaderTypeEventStream),
			NewVerifyStreamUploaderChangesetStep(c, StreamUploaderTypeBackup),
			NewVerifyOtelCollectorStep(c),
		},
		statusGetter: StatusFromNetworkOperation,
	}
}

// Execute runs the execute immediate task
func (t *ExecuteImmediateTask) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*UpgradeStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	result, err := baseTask.Execute(ctx, op)
	if err != nil {
		return result, err
	}

	// Override the reconcile delay if all steps are done
	if t.IsDone(op, result.Status) {
		result.ReconcileDelay = ExecuteImmediateReconcileDelay
	} else {
		result.ReconcileDelay = 1 * time.Second
	}

	return result, nil
}

// IsDone checks if all execute immediate steps are complete
func (t *ExecuteImmediateTask) IsDone(op *operatorv1alpha1.NetworkOperation, status *UpgradeStatus) bool {
	// Check consensus node changeset
	if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonConsensusNode) {
		return false
	}

	// Check stream uploaders if they have changesets
	if hasStreamUploaderChangeset(op, StreamUploaderTypeRecordStream) {
		if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonRecordStreamUploader) {
			return false
		}
	}
	if hasStreamUploaderChangeset(op, StreamUploaderTypeSidecarFiles) {
		if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonSidecarFilesUploader) {
			return false
		}
	}
	if hasStreamUploaderChangeset(op, StreamUploaderTypeEventStream) {
		if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonEventStreamUploader) {
			return false
		}
	}
	if hasStreamUploaderChangeset(op, StreamUploaderTypeBackup) {
		if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonBackupUploader) {
			return false
		}
	}

	// Check otel collector if it has a changeset
	if hasOtelChangeset(op) {
		if !status.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonOtelCollector) {
			return false
		}
	}

	return true
}
