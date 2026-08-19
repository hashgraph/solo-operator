// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/automa-saga/errx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ensureFinalizer adds finalizer to obj if absent. A non-nil result means the
// caller should return it, short-circuiting reconcile; nil means proceed.
func ensureFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) (*ctrl.Result, error) {
	// Never re-add a finalizer to a deleting object: the API server rejects it,
	// and there is nothing to reconcile on a doomed object.
	if !obj.GetDeletionTimestamp().IsZero() {
		return &ctrl.Result{}, nil
	}
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil, nil
	}
	controllerutil.AddFinalizer(obj, finalizer)
	if err := c.Update(ctx, obj); err != nil {
		return &ctrl.Result{}, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "adding finalizer"), ReasonFinalizerFailed),
			"Confirm the operator has update permission on the resource's finalizers")
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
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing %s", kind), ReasonListFailed),
			"Confirm the operator has list permission on "+kind)
	}
	its := items(list)
	if len(its) == 0 {
		return nil, nil
	}
	if len(its) > 1 {
		return nil, errx.WithHints(
			errx.WithReason(errReconcile.New("expected at most one %s for operation %q, found %d", kind, operationID, len(its)), ReasonDuplicateSibling),
			"Delete the duplicate "+kind+" CRs sharing this operationId")
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
		return false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing %s siblings", kind), ReasonListFailed),
			"Confirm the operator has list permission on "+kind)
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

func executeSiblingExists(ctx context.Context, c client.Client, namespace, selfUID, selfConsensusNode, operationID string) (bool, error) {
	var list operatorv1alpha1.NetworkUpgradeExecuteList
	if err := c.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return false, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "listing NetworkUpgradeExecute siblings"), ReasonListFailed),
			"Confirm the operator has list permission on NetworkUpgradeExecute")
	}
	for i := range list.Items {
		item := &list.Items[i]
		if string(item.UID) != selfUID && item.Spec.ConsensusNode == selfConsensusNode {
			return true, nil
		}
	}
	return false, nil
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

func getOrbit(ctx context.Context, c client.Client, orbitName string) (*operatorv1alpha1.Orbit, error) {
	if orbitName == "" {
		return nil, nil
	}
	var orbit operatorv1alpha1.Orbit
	if err := c.Get(ctx, client.ObjectKey{Name: orbitName}, &orbit); err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "getting Orbit %s", orbitName), ReasonGetFailed),
			"Confirm the Orbit exists and the operator has get permission on orbits")
	}
	return &orbit, nil
}

func resolveUpgradeConfigFromOrbit(ctx context.Context, c client.Client, orbitName string) (operatorv1alpha1.EffectiveUpgradeConfig, error) {
	orbit, err := getOrbit(ctx, c, orbitName)
	if err != nil {
		return operatorv1alpha1.EffectiveUpgradeConfig{}, err
	}
	return operatorv1alpha1.ResolveUpgradeConfig(orbit), nil
}

// prefetchCompleted checks the Prepare CR's prefetch conditions.
// Returns (true, true) when prefetch Jobs were created AND all completed
// successfully. Returns (true, false) when prefetch was attempted but failed.
// Returns (false, false) when prefetch is pending, was never attempted, or was
// skipped (no capsules) — caller should fall through to normal registry checks.
func prefetchCompleted(prepare *operatorv1alpha1.NetworkUpgradePrepare) (decided, succeeded bool) {
	if prepare == nil {
		return false, false
	}
	created := findCondition(prepare.Status.Conditions, operatorv1alpha1.CondImagePrefetchCreated)
	if created == nil || created.Status != metav1.ConditionTrue {
		return false, false
	}
	completed := findCondition(prepare.Status.Conditions, operatorv1alpha1.CondImagePrefetchCompleted)
	if completed == nil {
		return false, false
	}
	return true, completed.Status == metav1.ConditionTrue
}
