// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// These finalizers block accidental deletion of NetworkUpgrade CRs; the
// operator never removes them, so deletion requires an admin patching the
// finalizer off deliberately.
const (
	FinalizerNetworkUpgradePrepare     = "operator.solo.hedera.com/networkupgradeprepare"
	FinalizerNetworkUpgradeFreeze      = "operator.solo.hedera.com/networkupgradefreeze"
	FinalizerNetworkUpgradeExecute     = "operator.solo.hedera.com/networkupgradeexecute"
	FinalizerNetworkUpgradeFreezeAbort = "operator.solo.hedera.com/networkupgradefreezeabort"
)

// ensureFinalizer adds finalizer to obj if absent. It returns a non-nil result
// (which the caller should return, short-circuiting reconcile) when it wrote the
// finalizer or the write failed, and nil when the finalizer was already present.
func ensureFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) (*ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil, nil
	}
	controllerutil.AddFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return &ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
	}
	return &ctrl.Result{Requeue: true}, nil
}

func findOneByOperationID[T any, TList client.ObjectList](
	ctx context.Context,
	c client.Client,
	namespace, operationID, kind string,
	newList func() TList,
	items func(TList) []T,
) (*T, error) {
	list := newList()
	if err := c.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return nil, fmt.Errorf("listing %s: %w", kind, err)
	}
	its := items(list)
	if len(its) == 0 {
		return nil, nil
	}
	if len(its) > 1 {
		return nil, fmt.Errorf("expected at most one %s for operation %q, found %d", kind, operationID, len(its))
	}
	return &its[0], nil
}

func findPrepare(ctx context.Context, c client.Client, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradePrepare, error) {
	return findOneByOperationID(ctx, c, namespace, operationID, "NetworkUpgradePrepare",
		func() *operatorv1alpha1.NetworkUpgradePrepareList {
			return &operatorv1alpha1.NetworkUpgradePrepareList{}
		},
		func(l *operatorv1alpha1.NetworkUpgradePrepareList) []operatorv1alpha1.NetworkUpgradePrepare {
			return l.Items
		},
	)
}

func findFreeze(ctx context.Context, c client.Client, namespace, operationID string) (*operatorv1alpha1.NetworkUpgradeFreeze, error) {
	return findOneByOperationID(ctx, c, namespace, operationID, "NetworkUpgradeFreeze",
		func() *operatorv1alpha1.NetworkUpgradeFreezeList { return &operatorv1alpha1.NetworkUpgradeFreezeList{} },
		func(l *operatorv1alpha1.NetworkUpgradeFreezeList) []operatorv1alpha1.NetworkUpgradeFreeze {
			return l.Items
		},
	)
}
