// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

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

func siblingExists[TList client.ObjectList](
	ctx context.Context,
	c client.Client,
	namespace, selfUID, operationID, kind string,
	list TList,
	getUID func(TList, int) string,
	length func(TList) int,
) (bool, error) {
	if err := c.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return false, fmt.Errorf("listing %s siblings: %w", kind, err)
	}
	for i := range length(list) {
		if getUID(list, i) != selfUID {
			return true, nil
		}
	}
	return false, nil
}

func prepareSiblingExists(ctx context.Context, c client.Client, namespace, selfUID, operationID string) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradePrepareList
	return siblingExists(ctx, c, namespace, selfUID, operationID, "NetworkUpgradePrepare", &list,
		func(l *operatorv1alpha1.NetworkUpgradePrepareList, i int) string { return string(l.Items[i].UID) },
		func(l *operatorv1alpha1.NetworkUpgradePrepareList) int { return len(l.Items) },
	)
}

func freezeSiblingExists(ctx context.Context, c client.Client, namespace, selfUID, operationID string) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeFreezeList
	return siblingExists(ctx, c, namespace, selfUID, operationID, "NetworkUpgradeFreeze", &list,
		func(l *operatorv1alpha1.NetworkUpgradeFreezeList, i int) string { return string(l.Items[i].UID) },
		func(l *operatorv1alpha1.NetworkUpgradeFreezeList) int { return len(l.Items) },
	)
}

func executeSiblingExists(ctx context.Context, c client.Client, namespace, selfUID, operationID string) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	return siblingExists(ctx, c, namespace, selfUID, operationID, "NetworkUpgradeExecute", &list,
		func(l *operatorv1alpha1.NetworkUpgradeExecuteList, i int) string { return string(l.Items[i].UID) },
		func(l *operatorv1alpha1.NetworkUpgradeExecuteList) int { return len(l.Items) },
	)
}

func abortSiblingExists(ctx context.Context, c client.Client, namespace, selfUID, operationID string) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeFreezeAbortList
	return siblingExists(ctx, c, namespace, selfUID, operationID, "NetworkUpgradeFreezeAbort", &list,
		func(l *operatorv1alpha1.NetworkUpgradeFreezeAbortList, i int) string { return string(l.Items[i].UID) },
		func(l *operatorv1alpha1.NetworkUpgradeFreezeAbortList) int { return len(l.Items) },
	)
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
