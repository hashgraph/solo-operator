// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// res is a minimal authn.Resource for resolving against a keychain in tests.
type res struct{ registry string }

func (r res) String() string      { return r.registry + "/some/image:tag" }
func (r res) RegistryStr() string { return r.registry }

func keychainTestClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// dockerSecret builds a kubernetes.io/dockerconfigjson secret whose auths map the
// given registry hosts to "<host>-user"/"<host>-pass" credentials via the base64
// "auth" field (the form the kubelet and docker write).
func dockerSecret(secretName, namespace string, hosts ...string) *corev1.Secret {
	auths := ""
	for i, h := range hosts {
		if i > 0 {
			auths += ","
		}
		user, pass := h+"-user", h+"-pass"
		b64 := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		auths += fmt.Sprintf(`%q:{"auth":%q}`, h, b64)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{%s}}`, auths)),
		},
	}
}

func resolveConfig(t *testing.T, kc authn.Keychain, registry string) authn.AuthConfig {
	t.Helper()
	a, err := kc.Resolve(res{registry: registry})
	if err != nil {
		t.Fatalf("Resolve(%q): %v", registry, err)
	}
	cfg, err := a.Authorization()
	if err != nil {
		t.Fatalf("Authorization(): %v", err)
	}
	return *cfg
}

func TestKeychainFromPullSecrets_EmptyRefsReturnsDefault(t *testing.T) {
	kc, err := KeychainFromPullSecrets(context.Background(), keychainTestClient(), "ns", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kc != authn.DefaultKeychain {
		t.Fatalf("expected DefaultKeychain for empty refs, got %T", kc)
	}
}

func TestKeychainFromPullSecrets_ResolvesPrivateRegistry(t *testing.T) {
	secret := dockerSecret("ghcr-creds", "ns", "ghcr.io")
	c := keychainTestClient(secret)

	kc, err := KeychainFromPullSecrets(context.Background(), c, "ns", []corev1.LocalObjectReference{{Name: "ghcr-creds"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resolveConfig(t, kc, "ghcr.io")
	if got.Username != "ghcr.io-user" || got.Password != "ghcr.io-pass" {
		t.Fatalf("wrong credentials: %+v", got)
	}
}

func TestKeychainFromPullSecrets_ExplicitUsernamePassword(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com:5000":{"username":"alice","password":"s3cret"}}}`),
		},
	}
	c := keychainTestClient(secret)

	kc, err := KeychainFromPullSecrets(context.Background(), c, "ns", []corev1.LocalObjectReference{{Name: "creds"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resolveConfig(t, kc, "registry.example.com:5000")
	if got.Username != "alice" || got.Password != "s3cret" {
		t.Fatalf("wrong credentials for host:port registry: %+v", got)
	}
}

func TestKeychainFromPullSecrets_AuthFillsOnlyMissingFields(t *testing.T) {
	// Entry sets an explicit username but no password, plus an "auth" blob whose
	// decoded username differs. The decode must fill only the missing password and
	// must NOT clobber the explicitly-set username.
	authBlob := base64.StdEncoding.EncodeToString([]byte("authuser:authpass"))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mixed", Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(fmt.Sprintf(`{"auths":{"registry.example.com":{"username":"explicit","auth":%q}}}`, authBlob)),
		},
	}
	c := keychainTestClient(secret)

	kc, err := KeychainFromPullSecrets(context.Background(), c, "ns", []corev1.LocalObjectReference{{Name: "mixed"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resolveConfig(t, kc, "registry.example.com")
	if got.Username != "explicit" {
		t.Errorf("explicit username was clobbered: got %q, want explicit", got.Username)
	}
	if got.Password != "authpass" {
		t.Errorf("missing password not filled from auth: got %q, want authpass", got.Password)
	}
}

func TestKeychainFromPullSecrets_DockerHubAlias(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dh", Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"https://index.docker.io/v1/":{"username":"bob","password":"pw"}}}`),
		},
	}
	c := keychainTestClient(secret)

	kc, err := KeychainFromPullSecrets(context.Background(), c, "ns", []corev1.LocalObjectReference{{Name: "dh"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resolveConfig(t, kc, name.DefaultRegistry)
	if got.Username != "bob" || got.Password != "pw" {
		t.Fatalf("docker hub alias did not resolve: %+v", got)
	}
}

func TestKeychainFromPullSecrets_MissingSecretSkipped(t *testing.T) {
	// No secret objects exist; the reference is skipped and, with no usable
	// credentials, the builder falls back to the default keychain.
	kc, err := KeychainFromPullSecrets(context.Background(), keychainTestClient(), "ns", []corev1.LocalObjectReference{{Name: "absent"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kc != authn.DefaultKeychain {
		t.Fatalf("expected DefaultKeychain when the only secret is missing, got %T", kc)
	}
}

func TestKeychainFromPullSecrets_MalformedJSON(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{not json`)},
	}
	c := keychainTestClient(secret)

	_, err := KeychainFromPullSecrets(context.Background(), c, "ns", []corev1.LocalObjectReference{{Name: "bad"}})
	if err == nil {
		t.Fatal("expected error for malformed dockerconfigjson, got nil")
	}
}

func TestKeychainFromPullSecrets_MultipleSecretsFirstWins(t *testing.T) {
	first := dockerSecret("first", "ns", "ghcr.io")
	// second also declares ghcr.io but with different (host-derived) creds; the
	// first-listed secret must win, matching kubelet first-match semantics.
	second := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "second", Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"ghcr.io":{"username":"loser","password":"x"},"quay.io":{"username":"quayuser","password":"q"}}}`),
		},
	}
	c := keychainTestClient(first, second)

	kc, err := KeychainFromPullSecrets(context.Background(), c, "ns",
		[]corev1.LocalObjectReference{{Name: "first"}, {Name: "second"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resolveConfig(t, kc, "ghcr.io"); got.Username != "ghcr.io-user" {
		t.Fatalf("first secret should win for ghcr.io, got %+v", got)
	}
	if got := resolveConfig(t, kc, "quay.io"); got.Username != "quayuser" {
		t.Fatalf("second secret should provide quay.io, got %+v", got)
	}
}

func TestKeychainFromPullSecrets_NonDockerConfigSecretIgnored(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "opaque", Namespace: "ns"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"foo": []byte("bar")},
	}
	c := keychainTestClient(secret)

	kc, err := KeychainFromPullSecrets(context.Background(), c, "ns", []corev1.LocalObjectReference{{Name: "opaque"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kc != authn.DefaultKeychain {
		t.Fatalf("opaque secret should contribute no credentials, got %T", kc)
	}
}

func TestSecretKeychain_UnknownHostIsAnonymous(t *testing.T) {
	k := &secretKeychain{auths: map[string]authn.AuthConfig{"ghcr.io": {Username: "u", Password: "p"}}}
	a, err := k.Resolve(res{registry: "registry.other.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != authn.Anonymous {
		t.Fatalf("unknown host should resolve to Anonymous so multiKeychain falls through, got %T", a)
	}
}
