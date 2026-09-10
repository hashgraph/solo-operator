// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

func TestNamespaceRegistryChecker_OverrideWins(t *testing.T) {
	override := &fakeImageChecker{}
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).Build()

	got, err := namespaceRegistryChecker(context.Background(), c, override, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != registry.ImageChecker(override) {
		t.Error("expected the injected override checker to be returned unchanged")
	}
}

func TestNamespaceRegistryChecker_BuildsOCICheckerWithAllowedHosts(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).Build()
	hosts := []string{"ghcr.io", "registry.example.com"}

	got, err := namespaceRegistryChecker(context.Background(), c, nil, "test", hosts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oci, ok := got.(registry.OCIChecker)
	if !ok {
		t.Fatalf("expected an OCIChecker, got %T", got)
	}
	if oci.Keychain == nil {
		t.Error("expected a non-nil keychain")
	}
	if len(oci.AllowedHosts) != 2 || oci.AllowedHosts[0] != "ghcr.io" {
		t.Errorf("AllowedHosts not threaded through: %v", oci.AllowedHosts)
	}
}

func TestNamespacePullSecretRefs_UnionAcrossCapsules(t *testing.T) {
	mk := func(name string, cnSecret, ucSecret string) *operatorv1alpha1.ConsensusCapsule {
		cap := &operatorv1alpha1.ConsensusCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		}
		ct := &cap.Spec.PodProperties.Containers
		ct.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: cnSecret}},
		}
		ct.UC = &operatorv1alpha1.UcSidecar{
			SoftwareVersionSource: &operatorv1alpha1.SoftwareVersionSource{
				ImageRepositories: []operatorv1alpha1.ImageRepository{
					{Repository: "r", ImageName: "n", ImageTag: "t", ImagePullSecrets: []corev1.LocalObjectReference{{Name: ucSecret}}},
				},
			},
		}
		return cap
	}
	// node0 and node1 share "shared-cred"; each also has a distinct one.
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).
		WithObjects(mk("node0", "shared-cred", "uc-cred-0"), mk("node1", "shared-cred", "uc-cred-1")).
		Build()

	refs, err := namespacePullSecretRefs(context.Background(), c, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, len(refs))
	for i, r := range refs {
		got[i] = r.Name
	}
	sort.Strings(got)
	want := []string{"shared-cred", "uc-cred-0", "uc-cred-1"}
	if len(got) != len(want) {
		t.Fatalf("refs = %v, want %v (deduped union)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("refs = %v, want %v", got, want)
			break
		}
	}
}
