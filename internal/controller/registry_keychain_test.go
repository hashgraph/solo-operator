// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/registry"
)

func capsuleWithSecrets(name string) *operatorv1alpha1.ConsensusCapsule {
	c := &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"}}
	ct := &c.Spec.PodProperties.Containers
	ct.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "cn-cred"}},
	}
	ct.UC = &operatorv1alpha1.UcSidecar{
		Enabled: true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "uc-cred"}},
		},
	}
	// A disabled uploader whose secret must NOT be harvested.
	ct.RecordsUploader = &operatorv1alpha1.CheetahUploader{
		Enabled: false,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "disabled-cred"}},
		},
	}
	return c
}

func TestCapsulePullSecretRefs_EnabledContainersOnly(t *testing.T) {
	capsule := capsuleWithSecrets("node0")
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).WithObjects(capsule).Build()

	refs, err := capsulePullSecretRefs(context.Background(), c, "test", "node0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := map[string]bool{}
	for _, r := range refs {
		names[r.Name] = true
	}
	if !names["cn-cred"] || !names["uc-cred"] {
		t.Errorf("expected cn-cred and uc-cred, got %v", refs)
	}
	if names["disabled-cred"] {
		t.Errorf("harvested a disabled container's pull secret: %v", refs)
	}
}

func TestCapsuleRegistryChecker_OverrideWins(t *testing.T) {
	override := &fakeImageChecker{}
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).Build()

	got, err := capsuleRegistryChecker(context.Background(), c, override, "test", "node0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != registry.ImageChecker(override) {
		t.Error("expected the injected override checker to be returned unchanged")
	}
}

func TestCapsuleRegistryChecker_MissingCapsuleFallsBack(t *testing.T) {
	// The named capsule does not exist (e.g. scaled down mid-upgrade): no secrets to
	// harvest, so fall back to the default keychain rather than failing.
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).Build()

	got, err := capsuleRegistryChecker(context.Background(), c, nil, "test", "gone", []string{"ghcr.io"})
	if err != nil {
		t.Fatalf("expected no error for a missing capsule, got %v", err)
	}
	oci, ok := got.(registry.OCIChecker)
	if !ok {
		t.Fatalf("expected an OCIChecker, got %T", got)
	}
	// No pull secrets to harvest → KeychainFromPullSecrets returns the default keychain.
	if oci.Keychain != authn.DefaultKeychain {
		t.Errorf("expected the default keychain for a missing capsule, got %#v", oci.Keychain)
	}
	if len(oci.AllowedHosts) != 1 || oci.AllowedHosts[0] != "ghcr.io" {
		t.Errorf("AllowedHosts not preserved: %v", oci.AllowedHosts)
	}
}

func TestCapsuleRegistryChecker_MalformedSecretFallsBackNoError(t *testing.T) {
	// The capsule references a malformed dockerconfigjson secret. Keychain construction
	// fails, but the checker must NOT return an error (which would infinite-backoff and
	// block even public upgrades); it falls back to the default keychain.
	capsule := &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"}}
	capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "bad"}},
	}
	badSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "test"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{not json`)},
	}
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).WithObjects(capsule, badSecret).Build()

	got, err := capsuleRegistryChecker(context.Background(), c, nil, "test", "node0", nil)
	if err != nil {
		t.Fatalf("expected no error on a malformed secret (best-effort fallback), got %v", err)
	}
	oci, ok := got.(registry.OCIChecker)
	if !ok {
		t.Fatalf("expected an OCIChecker, got %T", got)
	}
	if oci.Keychain != nil {
		t.Errorf("expected fallback to default keychain, got %#v", oci.Keychain)
	}
}

func TestCapsuleRegistryChecker_BuildsKeychainFromValidSecret(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"}}
	capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion = &operatorv1alpha1.SoftwareVersion{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "ghcr-creds"}},
	}
	auth := base64.StdEncoding.EncodeToString([]byte("u:p"))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ghcr-creds", Namespace: "test"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{"ghcr.io":{"auth":%q}}}`, auth)),
		},
	}
	c := fake.NewClientBuilder().WithScheme(newControllerTestScheme()).WithObjects(capsule, secret).Build()

	got, err := capsuleRegistryChecker(context.Background(), c, nil, "test", "node0", []string{"ghcr.io"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oci, ok := got.(registry.OCIChecker)
	if !ok {
		t.Fatalf("expected an OCIChecker, got %T", got)
	}
	if oci.Keychain == nil {
		t.Error("expected a non-nil keychain built from the capsule's secret")
	}
	if len(oci.AllowedHosts) != 1 || oci.AllowedHosts[0] != "ghcr.io" {
		t.Errorf("AllowedHosts not threaded through: %v", oci.AllowedHosts)
	}
}
