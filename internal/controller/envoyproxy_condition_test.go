// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"errors"
	"strings"
	"testing"

	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconciledCondition(t *testing.T) {
	ok := reconciledCondition(3, nil)
	if ok.Status != metav1.ConditionTrue || ok.Reason != "Reconciled" || ok.ObservedGeneration != 3 {
		t.Errorf("unexpected success condition %+v", ok)
	}

	typed := errx.WithHints(errx.WithReason(errorx.NewNamespace("t").NewType("x").New("ACL ConfigMap ofac not found"), "ACLSourceMissing"), "Create ConfigMap ofac")
	failed := reconciledCondition(4, typed)
	if failed.Status != metav1.ConditionFalse || failed.Reason != "ACLSourceMissing" ||
		!strings.Contains(failed.Message, "ofac not found") || !strings.Contains(failed.Message, "Create ConfigMap ofac") {
		t.Errorf("unexpected failure condition %+v", failed)
	}

	if got := reconciledCondition(1, errors.New(strings.Repeat("x", 5000))); got.Reason != "ReconcileFailed" || len(got.Message) != 1024 {
		t.Errorf("expected fallback reason and truncated message, got %s (%d)", got.Reason, len(got.Message))
	}
}
