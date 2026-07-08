// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mustAllowCreate(t *testing.T, warns []string, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("ValidateCreate: unexpected error: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("ValidateCreate: unexpected warnings: %v", warns)
	}
}

func mustRejectWithName(t *testing.T, label string, err error, name string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", label)
	}
	if !strings.Contains(err.Error(), name) {
		t.Errorf("%s: error %q does not contain %q", label, err.Error(), name)
	}
}

func TestNetworkUpgradePrepareValidator(t *testing.T) {
	v := &NetworkUpgradePrepareValidator{}
	obj := &NetworkUpgradePrepare{ObjectMeta: metav1.ObjectMeta{Name: "prep-1"}}
	ctx := context.Background()

	w, err := v.ValidateCreate(ctx, obj)
	mustAllowCreate(t, w, err)

	_, err = v.ValidateUpdate(ctx, obj, obj)
	mustRejectWithName(t, "ValidateUpdate", err, "prep-1")

	_, err = v.ValidateDelete(ctx, obj)
	mustRejectWithName(t, "ValidateDelete", err, "prep-1")
}

func TestNetworkUpgradeFreezeValidator(t *testing.T) {
	v := &NetworkUpgradeFreezeValidator{}
	obj := &NetworkUpgradeFreeze{ObjectMeta: metav1.ObjectMeta{Name: "freeze-1"}}
	ctx := context.Background()

	w, err := v.ValidateCreate(ctx, obj)
	mustAllowCreate(t, w, err)

	_, err = v.ValidateUpdate(ctx, obj, obj)
	mustRejectWithName(t, "ValidateUpdate", err, "freeze-1")

	_, err = v.ValidateDelete(ctx, obj)
	mustRejectWithName(t, "ValidateDelete", err, "freeze-1")
}

func TestNetworkUpgradeExecuteValidator(t *testing.T) {
	v := &NetworkUpgradeExecuteValidator{}
	obj := &NetworkUpgradeExecute{ObjectMeta: metav1.ObjectMeta{Name: "exec-1"}}
	ctx := context.Background()

	w, err := v.ValidateCreate(ctx, obj)
	mustAllowCreate(t, w, err)

	_, err = v.ValidateUpdate(ctx, obj, obj)
	mustRejectWithName(t, "ValidateUpdate", err, "exec-1")

	_, err = v.ValidateDelete(ctx, obj)
	mustRejectWithName(t, "ValidateDelete", err, "exec-1")
}

func TestNetworkUpgradeFreezeAbortValidator(t *testing.T) {
	v := &NetworkUpgradeFreezeAbortValidator{}
	obj := &NetworkUpgradeFreezeAbort{ObjectMeta: metav1.ObjectMeta{Name: "abort-1"}}
	ctx := context.Background()

	w, err := v.ValidateCreate(ctx, obj)
	mustAllowCreate(t, w, err)

	_, err = v.ValidateUpdate(ctx, obj, obj)
	mustRejectWithName(t, "ValidateUpdate", err, "abort-1")

	_, err = v.ValidateDelete(ctx, obj)
	mustRejectWithName(t, "ValidateDelete", err, "abort-1")
}
