// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
)

const (
	// StepConditionTypeVerifySecrets is the condition type for secret verification
	StepConditionTypeVerifySecrets = "VerifySecrets"
	// StepConditionReasonConsensusNodeSecrets is the condition reason for consensus node secrets
	StepConditionReasonConsensusNodeSecrets = "ConsensusNodeSecrets"

	// StepConditionTypeConfigCreated is the condition type for config creation
	StepConditionTypeConfigCreated = "ConfigCreated"
	// StepConditionReasonClusterConfig is the condition reason for cluster config
	StepConditionReasonClusterConfig = "ClusterConfig"
	// StepConditionReasonPublicKeyConfig is the condition reason for public key config
	StepConditionReasonPublicKeyConfig = "PublicKeyConfig"

	// ClusterConfigName is the name of the cluster config ConfigMap
	ClusterConfigName = "cluster-config"
	// PublicKeysName is the name of the public keys ConfigMap
	PublicKeysName = "public-keys"

	// ConfigKeyConfigTxt is the key for config.txt in the ConfigMap
	ConfigKeyConfigTxt = "config.txt"
	// ConfigKeyPodIPs is the key for pod IPs in the ConfigMap
	ConfigKeyPodIPs = "pod-ips"
)

// GenesisStatus represents the status during genesis processing
type GenesisStatus struct {
	Phase             operatorv1alpha1.Phase
	Conditions        []metav1.Condition
	ConsensusCapsules []string
}

// GetPhase returns the current phase
func (s *GenesisStatus) GetPhase() operatorv1alpha1.Phase {
	return s.Phase
}

// SetPhase sets the phase
func (s *GenesisStatus) SetPhase(phase operatorv1alpha1.Phase) {
	s.Phase = phase
}

// GetConditions returns the conditions
func (s *GenesisStatus) GetConditions() []metav1.Condition {
	return s.Conditions
}

// AddCondition adds a condition to the status
func (s *GenesisStatus) AddCondition(condition metav1.Condition) {
	// Remove existing condition with same type
	filtered := make([]metav1.Condition, 0, len(s.Conditions))
	for _, c := range s.Conditions {
		if c.Type != condition.Type {
			filtered = append(filtered, c)
		}
	}
	s.Conditions = append(filtered, condition)
}

// HasSuccessfulCondition checks if a condition with the given type and reason is successful
func (s *GenesisStatus) HasSuccessfulCondition(conditionType, reason string) bool {
	for _, c := range s.Conditions {
		if c.Type == conditionType && c.Reason == reason && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// GenesisContext holds context for genesis processing
type GenesisContext struct {
	Client    client.Client
	Namespace string
	OrbitName string
}

// Task represents the genesis task
type Task struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *GenesisStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *GenesisStatus
}

// NewTask creates a new genesis task
func NewTask(c client.Client) *Task {
	return &Task{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *GenesisStatus]{
			NewCheckSecretsStep(c),
			NewCreatePublicKeysStep(c),
			processor.NewRetryStepWithConfig[*operatorv1alpha1.NetworkOperation, *GenesisStatus](
				NewCreateClusterConfigStep(c),
				2*time.Second,
				150, // ~5 minutes of retries
			),
		},
		statusGetter: func(op *operatorv1alpha1.NetworkOperation) *GenesisStatus {
			return &GenesisStatus{
				Phase:             op.Status.Phase,
				Conditions:        op.Status.Conditions,
				ConsensusCapsules: op.Status.ConsensusCapsules,
			}
		},
	}
}

// Steps returns the steps for this task
func (t *Task) Steps() []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *GenesisStatus] {
	return t.steps
}

// StatusGetter returns the status getter function
func (t *Task) StatusGetter() func(*operatorv1alpha1.NetworkOperation) *GenesisStatus {
	return t.statusGetter
}

// Execute runs the genesis task
func (t *Task) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*GenesisStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	return baseTask.Execute(ctx, op)
}
