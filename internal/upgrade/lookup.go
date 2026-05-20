// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// FindPrepareForOperation returns the unique NetworkUpgradePrepare CR carrying
// the given operation-id label in the given namespace, or nil when none is
// found. An error is returned when more than one matches (UC contract violation).
func FindPrepareForOperation(ctx context.Context, c client.Client, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradePrepare, error) {
	list := &operatorv1alpha1.NetworkUpgradePrepareList{}
	if err := c.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{operatorv1alpha1.LabelOperationID: operationID}); err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("expected at most one NetworkUpgradePrepare for operation %q, found %d", operationID, len(list.Items))
	}
	return &list.Items[0], nil
}

func FindFreezeForOperation(ctx context.Context, c client.Client, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradeFreeze, error) {
	list := &operatorv1alpha1.NetworkUpgradeFreezeList{}
	if err := c.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{operatorv1alpha1.LabelOperationID: operationID}); err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("expected at most one NetworkUpgradeFreeze for operation %q, found %d", operationID, len(list.Items))
	}
	return &list.Items[0], nil
}

func FindExecuteForOperation(ctx context.Context, c client.Client, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradeExecute, error) {
	list := &operatorv1alpha1.NetworkUpgradeExecuteList{}
	if err := c.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingLabels{operatorv1alpha1.LabelOperationID: operationID}); err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	if len(list.Items) > 1 {
		return nil, fmt.Errorf("expected at most one NetworkUpgradeExecute for operation %q, found %d", operationID, len(list.Items))
	}
	return &list.Items[0], nil
}
