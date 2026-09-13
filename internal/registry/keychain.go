// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/automa-saga/errx"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KeychainFromPullSecrets builds an authn.Keychain from the named Kubernetes
// image-pull secrets in namespace, so the OCIChecker can inspect and verify
// images in private registries — the same credentials the kubelet uses to pull
// them. Secret-derived credentials take precedence over ambient/host credentials
// (authn.DefaultKeychain), which remain as a fallback so public registries and
// host-configured logins keep working.
//
// refs are resolved by name (spec.imagePullSecrets style). Missing secrets are
// skipped (best effort — a stale reference must not block verification), but an
// API error other than NotFound is returned. Only kubernetes.io/dockerconfigjson
// secrets contribute credentials; other secret types are ignored.
//
// When refs is empty or yields no usable credentials, it returns
// authn.DefaultKeychain unchanged.
func KeychainFromPullSecrets(ctx context.Context, c client.Client, namespace string, refs []corev1.LocalObjectReference) (authn.Keychain, error) {
	if len(refs) == 0 {
		return authn.DefaultKeychain, nil
	}

	auths := map[string]authn.AuthConfig{}
	for _, ref := range refs {
		if ref.Name == "" {
			continue
		}
		var secret corev1.Secret
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				// A referenced pull secret that does not exist is not fatal to
				// verification: the candidate may still be public, or covered by
				// another secret or ambient credentials. Skip it.
				continue
			}
			return nil, errx.WithHints(
				errx.WithReason(errPullSecret.Wrap(err, "reading image-pull secret %s/%s", namespace, ref.Name), ReasonPullSecretRead),
				"Confirm the operator has get permission on Secrets in the capsule namespace",
			)
		}
		if err := mergeDockerConfig(&secret, auths); err != nil {
			return nil, err
		}
	}

	if len(auths) == 0 {
		return authn.DefaultKeychain, nil
	}
	return authn.NewMultiKeychain(&secretKeychain{auths: auths}, authn.DefaultKeychain), nil
}

// dockerConfigJSON is the .dockerconfigjson shape:
// {"auths":{"<registry>":{"username":..,"password":..,"auth":"base64(user:pass)"}}}.
type dockerConfigJSON struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Auth          string `json:"auth"`
	IdentityToken string `json:"identitytoken"`
}

// mergeDockerConfig parses a dockerconfigjson secret and merges its per-registry
// credentials into auths, keyed by normalized registry host. Non-dockerconfigjson
// secrets are ignored. The first secret to provide a host wins (refs are applied
// in order), matching the kubelet's first-match semantics.
func mergeDockerConfig(secret *corev1.Secret, auths map[string]authn.AuthConfig) error {
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return nil
	}
	raw, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok || len(raw) == 0 {
		return nil
	}

	var cfg dockerConfigJSON
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return errx.WithHints(
			errx.WithReason(errPullSecretParse.Wrap(err, "parsing %s in secret %s/%s", corev1.DockerConfigJsonKey, secret.Namespace, secret.Name), ReasonPullSecretParse),
			"Confirm the secret is a valid kubernetes.io/dockerconfigjson credential",
		)
	}

	for registry, entry := range cfg.Auths {
		key := normalizeRegistryHost(registry)
		if _, exists := auths[key]; exists {
			continue
		}
		ac := authn.AuthConfig{
			Username:      entry.Username,
			Password:      entry.Password,
			IdentityToken: entry.IdentityToken,
		}
		// The "auth" field is base64("username:password"). When present, decode it to
		// fill only the missing username/password — never clobber a value the entry
		// already set explicitly.
		if entry.Auth != "" && (ac.Username == "" || ac.Password == "") {
			if u, p, ok := decodeDockerAuth(entry.Auth); ok {
				if ac.Username == "" {
					ac.Username = u
				}
				if ac.Password == "" {
					ac.Password = p
				}
			}
		}
		if ac.Username == "" && ac.Password == "" && ac.IdentityToken == "" {
			continue
		}
		auths[key] = ac
	}
	return nil
}

func decodeDockerAuth(auth string) (username, password string, ok bool) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(auth))
	if err != nil {
		return "", "", false
	}
	user, pass, found := strings.Cut(string(decoded), ":")
	if !found {
		return "", "", false
	}
	return user, pass, true
}

// normalizeRegistryHost reduces a dockerconfigjson auths key to the bare registry
// host used by authn.Resource.RegistryStr(): it strips any scheme and path and
// folds the Docker Hub aliases onto name.DefaultRegistry ("index.docker.io").
func normalizeRegistryHost(key string) string {
	key = strings.TrimPrefix(key, "https://")
	key = strings.TrimPrefix(key, "http://")
	if i := strings.IndexByte(key, '/'); i >= 0 {
		key = key[:i]
	}
	switch key {
	case "docker.io", "registry-1.docker.io", "index.docker.io":
		return name.DefaultRegistry
	default:
		return key
	}
}

// secretKeychain resolves credentials for a target registry from image-pull
// secrets. A host with no configured credential resolves to Anonymous so an
// enclosing multiKeychain falls through to the next keychain.
type secretKeychain struct {
	auths map[string]authn.AuthConfig
}

func (k *secretKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if ac, ok := k.auths[normalizeRegistryHost(target.RegistryStr())]; ok {
		return authn.FromConfig(ac), nil
	}
	return authn.Anonymous, nil
}
