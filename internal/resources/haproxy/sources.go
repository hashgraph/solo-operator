// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

func restartChecksum(ctx context.Context, c client.Client, haproxy *operatorv1alpha1.HAProxyCapsule) (string, error) {
	cfg, err := HAProxyConfig(haproxy)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(cfg))
	for _, v := range resolve(haproxy).volumes {
		for _, s := range v.sources {
			values, err := s.read(ctx, c, haproxy.Namespace)
			if err != nil {
				return "", err
			}
			_, _ = fmt.Fprintf(h, "\n%s/%s:%t", s.kind, s.name, values != nil)
			for i, it := range s.items {
				_, _ = fmt.Fprintf(h, "\n%s=", it.Key)
				if values != nil {
					_, _ = h.Write(values[i])
				}
			}
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (s source) read(ctx context.Context, c client.Client, namespace string) ([][]byte, error) {
	secret := &corev1.Secret{}
	cm := &corev1.ConfigMap{}
	obj := client.Object(secret)
	if s.kind == configMapSource {
		obj = cm
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: s.name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			common.Warn(ctx, ReasonSourceMissing).Str("kind", string(s.kind)).Str("name", s.name).Msg("Referenced source not found; HAProxy pod will wait for it")
			return nil, nil
		}
		return nil, errx.WithHints(
			errx.WithReason(errK8sAPI.Wrap(err, "get %s %s/%s", s.kind, namespace, s.name), ReasonSourceGetFailed),
			fmt.Sprintf("Confirm the operator ServiceAccount has get permission on %ss", strings.ToLower(string(s.kind))),
		)
	}
	values := make([][]byte, len(s.items))
	for i, it := range s.items {
		if s.kind == secretSource {
			values[i] = secret.Data[it.Key]
		} else {
			values[i] = []byte(cm.Data[it.Key])
		}
	}
	return values, nil
}
