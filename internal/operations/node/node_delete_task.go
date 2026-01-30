// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
)

// NodeDeleteTask performs the node delete operation
type NodeDeleteTask struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *NodeStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *NodeStatus
}

// NewNodeDeleteTask creates a new NodeDeleteTask
func NewNodeDeleteTask(c client.Client) *NodeDeleteTask {
	return &NodeDeleteTask{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *NodeStatus]{
			NewNodeDeleteStep(c),
		},
		statusGetter: StatusFromNetworkOperation,
	}
}

// Execute runs the node delete task
func (t *NodeDeleteTask) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*NodeStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	result, err := baseTask.Execute(ctx, op)
	if err != nil {
		return result, err
	}

	result.ReconcileDelay = NodeOperationReconcileDelay
	return result, nil
}
