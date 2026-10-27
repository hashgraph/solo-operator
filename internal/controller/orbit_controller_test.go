// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestDeleteOwnedCapsules_CascadesNetworkGenesis(t *testing.T) {
	scheme := newControllerTestScheme()

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
	}
	genesis := &operatorv1alpha1.NetworkGenesis{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit-genesis", Namespace: "test"},
		Spec: operatorv1alpha1.NetworkGenesisSpec{
			Orbit:       orbit.Name,
			OperationID: "op-1",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orbit, genesis).Build()
	r := &OrbitReconciler{Client: c, Scheme: scheme}

	remaining, err := r.deleteOwnedCapsules(context.Background(), orbit)
	if err != nil {
		t.Fatalf("deleteOwnedCapsules returned error: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining owned object, got %d", remaining)
	}

	err = c.Get(context.Background(), types.NamespacedName{Name: genesis.Name, Namespace: genesis.Namespace}, &operatorv1alpha1.NetworkGenesis{})
	if err == nil {
		t.Fatalf("expected NetworkGenesis to be deleted, but it still exists")
	}
}

func TestDeleteOwnedCapsules_CascadesBlockNodesConfig(t *testing.T) {
	scheme := newControllerTestScheme()

	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
	}
	owned := &operatorv1alpha1.BlockNodesConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "owned-bnc", Namespace: "test"},
		Spec:       operatorv1alpha1.BlockNodesConfigSpec{Orbit: orbit.Name},
	}
	foreign := &operatorv1alpha1.BlockNodesConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "foreign-bnc", Namespace: "test"},
		Spec:       operatorv1alpha1.BlockNodesConfigSpec{Orbit: "other-orbit"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orbit, owned, foreign).Build()
	r := &OrbitReconciler{Client: c, Scheme: scheme}

	remaining, err := r.deleteOwnedCapsules(context.Background(), orbit)
	if err != nil {
		t.Fatalf("deleteOwnedCapsules returned error: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining owned object, got %d", remaining)
	}

	if err := c.Get(context.Background(), client.ObjectKeyFromObject(owned), &operatorv1alpha1.BlockNodesConfig{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected owned BlockNodesConfig to be deleted, got err=%v", err)
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(foreign), &operatorv1alpha1.BlockNodesConfig{}); err != nil {
		t.Fatalf("expected BlockNodesConfig of another orbit to be kept, got err=%v", err)
	}
}

func TestSetNamespaceAllowed(t *testing.T) {
	tests := []struct {
		name       string
		allowed    []string
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{name: "all namespaces", allowed: nil},
		{name: "allowed", allowed: []string{"other", "solo"}, wantStatus: metav1.ConditionTrue, wantReason: ReasonNamespaceAllowed.String()},
		{name: "not allowed", allowed: []string{"other"}, wantStatus: metav1.ConditionFalse, wantReason: ReasonNamespaceNotAllowed.String()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orbit := &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: "solo"}}
			orbit.Status.Conditions = []metav1.Condition{{Type: operatorv1alpha1.OrbitConditionTypeNamespaceAllowed, Status: metav1.ConditionUnknown}}
			r := &OrbitReconciler{AllowedNamespaces: tt.allowed}

			r.setNamespaceAllowed(context.Background(), orbit)

			cond := apimeta.FindStatusCondition(orbit.Status.Conditions, operatorv1alpha1.OrbitConditionTypeNamespaceAllowed)
			if tt.wantStatus == "" {
				if cond != nil {
					t.Fatalf("expected condition removed, got %+v", cond)
				}
				return
			}
			if cond == nil {
				t.Fatal("expected NamespaceAllowed condition")
			}
			if cond.Status != tt.wantStatus || cond.Reason != tt.wantReason {
				t.Fatalf("got status=%s reason=%s, want status=%s reason=%s", cond.Status, cond.Reason, tt.wantStatus, tt.wantReason)
			}
		})
	}
}
