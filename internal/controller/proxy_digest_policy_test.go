// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/joomcode/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestValidateProxyDigestPolicy(t *testing.T) {
	const pinned = "envoyproxy/envoy:v1.40.2@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	orbit := func(require bool) *operatorv1alpha1.Orbit {
		return &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{Name: "orbit"},
			Spec:       operatorv1alpha1.OrbitSpec{RequireDigestOnDeploy: require},
		}
	}
	tests := []struct {
		name     string
		orbit    *operatorv1alpha1.Orbit
		image    string
		terminal bool
	}{
		{"policy off allows tag", orbit(false), "envoyproxy/envoy:v1.40.2", false},
		{"policy on allows digest", orbit(true), pinned, false},
		{"policy on rejects tag", orbit(true), "envoyproxy/envoy:v1.40.2", true},
		{"missing orbit is not blocking", nil, "envoyproxy/envoy:v1.40.2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := fake.NewClientBuilder().WithScheme(newControllerTestScheme())
			if tt.orbit != nil {
				b = b.WithObjects(tt.orbit)
			}
			err := validateProxyDigestPolicy(context.Background(), b.Build(), "orbit", "ep", tt.image)
			if !tt.terminal {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, reconcile.TerminalError(nil)) {
				t.Fatalf("expected a terminal error, got %v", err)
			}
			if !errorx.IsOfType(err, errImageDigestRequired) {
				t.Errorf("expected errImageDigestRequired, got %v", err)
			}
		})
	}
}
