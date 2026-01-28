// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Condition types
	StepConditionTypeVerifyChangeset    = "VerifyChangeset"
	StepConditionTypeApplyChangeset     = "ApplyChangeset"
	StepConditionTypeConfigDeleted      = "ConfigDeleted"
	StepConditionTypeConfigCreated      = "ConfigCreated"
	StepConditionTypeOrbitConfigApplied = "OrbitConfigApplied"
	StepConditionTypeFreezeAbort        = "FreezeAbort"

	// Condition reasons
	StepConditionReasonConsensusNode        = "ConsensusNode"
	StepConditionReasonRecordStreamUploader = "RecordStreamUploader"
	StepConditionReasonSidecarFilesUploader = "SidecarFilesUploader"
	StepConditionReasonEventStreamUploader  = "EventStreamUploader"
	StepConditionReasonBackupUploader       = "BackupUploader"
	StepConditionReasonOtelCollector        = "OtelCollector"
	StepConditionReasonClusterConfig        = "ClusterConfig"
	StepConditionReasonConsensusPodChange   = "ConsensusPodChange"
	StepConditionReasonOrbitConfiguration   = "OrbitConfiguration"
	StepConditionReasonFreezeAborted        = "FreezeAborted"

	// Default delays
	ExecuteImmediateReconcileDelay = 1 * time.Hour
	FreezeScheduledReconcileDelay  = 1 * time.Second
	NowFrozenReconcileDelay        = 1 * time.Second
	FreezeAbortReconcileDelay      = 1 * time.Second
)

// UpgradeStatus represents the status during upgrade processing
type UpgradeStatus struct {
	Phase                operatorv1alpha1.Phase
	Conditions           []metav1.Condition
	SoftwareSourceStatus *operatorv1alpha1.SoftwareSourceStatus
	UpgradedNodes        []int64
}

// GetPhase returns the current phase
func (s *UpgradeStatus) GetPhase() operatorv1alpha1.Phase {
	return s.Phase
}

// SetPhase sets the phase
func (s *UpgradeStatus) SetPhase(phase operatorv1alpha1.Phase) {
	s.Phase = phase
}

// GetConditions returns the conditions
func (s *UpgradeStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

// AddCondition adds a condition to the status
func (s *UpgradeStatus) AddCondition(condition metav1.Condition) {
	filtered := make([]metav1.Condition, 0, len(s.Conditions))
	for _, c := range s.Conditions {
		if c.Type != condition.Type || c.Reason != condition.Reason {
			filtered = append(filtered, c)
		}
	}
	s.Conditions = append(filtered, condition)
}

// HasSuccessfulCondition checks if a condition with the given type and reason is successful
func (s *UpgradeStatus) HasSuccessfulCondition(conditionType, reason string) bool {
	for _, c := range s.Conditions {
		if c.Type == conditionType && c.Reason == reason && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// StatusFromNetworkOperation extracts UpgradeStatus from a NetworkOperation
func StatusFromNetworkOperation(op *operatorv1alpha1.NetworkOperation) *UpgradeStatus {
	return &UpgradeStatus{
		Phase:                op.Status.Phase,
		Conditions:           op.Status.Conditions,
		SoftwareSourceStatus: op.Status.SoftwareSourceStatus,
		UpgradedNodes:        op.Status.UpgradedNodes,
	}
}

// Processor handles the network upgrade workflow based on the workflow step
type Processor struct {
	client               client.Client
	executeImmediateTask *ExecuteImmediateTask
	freezeScheduledTask  *FreezeScheduledTask
	nowFrozenTask        *NowFrozenTask
	freezeAbortTask      *FreezeAbortTask
}

// NewProcessor creates a new upgrade processor
func NewProcessor(c client.Client) *Processor {
	return &Processor{
		client:               c,
		executeImmediateTask: NewExecuteImmediateTask(c),
		freezeScheduledTask:  NewFreezeScheduledTask(c),
		nowFrozenTask:        NewNowFrozenTask(c),
		freezeAbortTask:      NewFreezeAbortTask(c),
	}
}

// Process executes the appropriate task based on the workflow step
func (p *Processor) Process(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*UpgradeStatus], error) {
	if op.Spec.NetworkUpgrade == nil {
		return &processor.Result[*UpgradeStatus]{
			Status: &UpgradeStatus{
				Phase: operatorv1alpha1.PhaseFailed,
				Conditions: []metav1.Condition{{
					Type:               "Failed",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "InvalidSpec",
					Message:            "NetworkUpgrade spec is required",
				}},
			},
		}, nil
	}

	switch op.Spec.NetworkUpgrade.WorkflowStep {
	case operatorv1alpha1.NetworkUpgradeWorkflowStepExecuteImmediate:
		return p.executeImmediateTask.Execute(ctx, op)
	case operatorv1alpha1.NetworkUpgradeWorkflowStepFreezeScheduled:
		return p.freezeScheduledTask.Execute(ctx, op)
	case operatorv1alpha1.NetworkUpgradeWorkflowStepNowFrozen:
		return p.nowFrozenTask.Execute(ctx, op)
	case operatorv1alpha1.NetworkUpgradeWorkflowStepFreezeAbort:
		return p.freezeAbortTask.Execute(ctx, op)
	default:
		return &processor.Result[*UpgradeStatus]{
			Status: &UpgradeStatus{
				Phase: operatorv1alpha1.PhaseFailed,
				Conditions: []metav1.Condition{{
					Type:               "Failed",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "UnknownWorkflowStep",
					Message:            "Unknown workflow step: " + string(op.Spec.NetworkUpgrade.WorkflowStep),
				}},
			},
		}, nil
	}
}

// StreamUploaderType represents the type of stream uploader
type StreamUploaderType string

const (
	StreamUploaderTypeRecordStream StreamUploaderType = "recordStreamUploader"
	StreamUploaderTypeSidecarFiles StreamUploaderType = "recordStreamSidecarFilesUploader"
	StreamUploaderTypeEventStream  StreamUploaderType = "eventsStreamUploader"
	StreamUploaderTypeBackup       StreamUploaderType = "backupUploader"
)

// hasStreamUploaderChangeset checks if the network operation has a changeset for the given uploader type
func hasStreamUploaderChangeset(op *operatorv1alpha1.NetworkOperation, uploaderType StreamUploaderType) bool {
	if op.Spec.NetworkUpgrade == nil {
		return false
	}

	changesets := &op.Spec.NetworkUpgrade.Changesets

	switch uploaderType {
	case StreamUploaderTypeRecordStream:
		return changesets.RecordStreamUploaderChangeset != nil
	case StreamUploaderTypeSidecarFiles:
		return changesets.RecordStreamSidecarUploaderChangeset != nil
	case StreamUploaderTypeEventStream:
		return changesets.EventsStreamUploaderChangeset != nil
	case StreamUploaderTypeBackup:
		return changesets.BackupUploaderChangeset != nil
	}
	return false
}

// hasOtelChangeset checks if the network operation has an otel collector changeset
func hasOtelChangeset(op *operatorv1alpha1.NetworkOperation) bool {
	if op.Spec.NetworkUpgrade == nil {
		return false
	}
	return op.Spec.NetworkUpgrade.Changesets.OtelCollector != nil
}

// getStreamUploaderConditionReason returns the condition reason for the given uploader type
func getStreamUploaderConditionReason(uploaderType StreamUploaderType) string {
	switch uploaderType {
	case StreamUploaderTypeRecordStream:
		return StepConditionReasonRecordStreamUploader
	case StreamUploaderTypeSidecarFiles:
		return StepConditionReasonSidecarFilesUploader
	case StreamUploaderTypeEventStream:
		return StepConditionReasonEventStreamUploader
	case StreamUploaderTypeBackup:
		return StepConditionReasonBackupUploader
	}
	return ""
}

// hasSelectedRepository checks if there's a selected repository in the status for the given component
func hasSelectedRepository(status *UpgradeStatus, component string) bool {
	if status.SoftwareSourceStatus == nil {
		return false
	}

	var repos []operatorv1alpha1.ImageRepositoryStatus
	switch component {
	case "consensusNode":
		repos = status.SoftwareSourceStatus.ConsensusNode
	case "recordStreamUploader":
		repos = status.SoftwareSourceStatus.RecordStreamUploader
	case "recordStreamSidecarUploader":
		repos = status.SoftwareSourceStatus.RecordStreamSidecarUploader
	case "eventsStreamUploader":
		repos = status.SoftwareSourceStatus.EventsStreamUploader
	case "backupUploader":
		repos = status.SoftwareSourceStatus.BackupUploader
	case "otelCollector":
		repos = status.SoftwareSourceStatus.OtelCollector
	}

	for _, repo := range repos {
		if repo.SelectedForDeployment {
			return true
		}
	}
	return false
}
