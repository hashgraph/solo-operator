// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// operationIDOf returns the authoritative operation id from spec.operationId.
// spec.operationId is identity; the operator.solo.hedera.com/operation-id label
// is only for List/Watch selection.
func operationIDOf(cr *unstructured.Unstructured) string {
	id, _, _ := unstructured.NestedString(cr.Object, "spec", "operationId")
	return id
}

// ExecutePhaseOf converts a raw status.phase string into the typed enum.
func ExecutePhaseOf(s string) operatorv1alpha1.ExecutePhase {
	return operatorv1alpha1.ExecutePhase(s)
}

// sleepCtx sleeps for d or until ctx is cancelled. Returns false if cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
