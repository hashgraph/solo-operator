// SPDX-License-Identifier: Apache-2.0

package node

import (
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Step condition type constants
const (
	StepConditionTypeNodeOperation = "NodeOperation"

	StepConditionReasonNodeAdd    = "NodeAdd"
	StepConditionReasonNodeUpdate = "NodeUpdate"
	StepConditionReasonNodeDelete = "NodeDelete"
)

// Reconcile delay constants
const (
	NodeOperationReconcileDelay = 1 * time.Second
)

// NodeStatus represents the status of a node operation
type NodeStatus struct {
	Phase      operatorv1alpha1.Phase
	Conditions []metav1.Condition
	Message    string
}

// AddCondition adds or updates a condition in the status
func (s *NodeStatus) AddCondition(condition metav1.Condition) {
	for i, existing := range s.Conditions {
		if existing.Type == condition.Type && existing.Reason == condition.Reason {
			s.Conditions[i] = condition
			return
		}
	}
	s.Conditions = append(s.Conditions, condition)
}

// HasSuccessfulCondition checks if a condition with the given type and reason exists and is successful
func (s *NodeStatus) HasSuccessfulCondition(conditionType, reason string) bool {
	for _, condition := range s.Conditions {
		if condition.Type == conditionType && condition.Reason == reason {
			return condition.Status == metav1.ConditionTrue
		}
	}
	return false
}

// StatusFromNetworkOperation extracts NodeStatus from a NetworkOperation
func StatusFromNetworkOperation(op *operatorv1alpha1.NetworkOperation) *NodeStatus {
	status := &NodeStatus{
		Phase:      op.Status.Phase,
		Conditions: make([]metav1.Condition, len(op.Status.Conditions)),
	}
	copy(status.Conditions, op.Status.Conditions)
	return status
}

// Processor handles node operations (add, update, delete)
type Processor struct {
	client         client.Client
	nodeAddTask    *NodeAddTask
	nodeUpdateTask *NodeUpdateTask
	nodeDeleteTask *NodeDeleteTask
}

// NewProcessor creates a new node operation processor
func NewProcessor(c client.Client) *Processor {
	return &Processor{
		client:         c,
		nodeAddTask:    NewNodeAddTask(c),
		nodeUpdateTask: NewNodeUpdateTask(c),
		nodeDeleteTask: NewNodeDeleteTask(c),
	}
}

// ProcessAdd processes a node add operation
func (p *Processor) ProcessAdd(op *operatorv1alpha1.NetworkOperation) (*processor.Result[*NodeStatus], error) {
	return p.nodeAddTask.Execute(nil, op)
}

// ProcessUpdate processes a node update operation
func (p *Processor) ProcessUpdate(op *operatorv1alpha1.NetworkOperation) (*processor.Result[*NodeStatus], error) {
	return p.nodeUpdateTask.Execute(nil, op)
}

// ProcessDelete processes a node delete operation
func (p *Processor) ProcessDelete(op *operatorv1alpha1.NetworkOperation) (*processor.Result[*NodeStatus], error) {
	return p.nodeDeleteTask.Execute(nil, op)
}
