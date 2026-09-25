// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
