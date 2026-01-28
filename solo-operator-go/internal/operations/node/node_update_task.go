// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/processor"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeUpdateTask performs the node update operation
type NodeUpdateTask struct {
	client       client.Client
	steps        []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *NodeStatus]
	statusGetter func(*operatorv1alpha1.NetworkOperation) *NodeStatus
}

// NewNodeUpdateTask creates a new NodeUpdateTask
func NewNodeUpdateTask(c client.Client) *NodeUpdateTask {
	return &NodeUpdateTask{
		client: c,
		steps: []processor.ProcessorStep[*operatorv1alpha1.NetworkOperation, *NodeStatus]{
			NewNodeUpdateStep(c),
		},
		statusGetter: StatusFromNetworkOperation,
	}
}

// Execute runs the node update task
func (t *NodeUpdateTask) Execute(ctx context.Context, op *operatorv1alpha1.NetworkOperation) (*processor.Result[*NodeStatus], error) {
	baseTask := processor.NewBaseTask(t.steps, t.statusGetter)
	result, err := baseTask.Execute(ctx, op)
	if err != nil {
		return result, err
	}

	result.ReconcileDelay = NodeOperationReconcileDelay
	return result, nil
}
