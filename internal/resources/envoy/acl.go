// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func ReferencedConfigMaps(envoy *operatorv1alpha1.EnvoyProxy) []string {
	names := make([]string, 0, len(envoy.Spec.ACLFiles))
	for _, ref := range envoy.Spec.ACLFiles {
		names = append(names, ref.Name)
	}
	return names
}

func loadACL(ctx context.Context, c client.Client, envoy *operatorv1alpha1.EnvoyProxy) ([]netip.Prefix, error) {
	var all []netip.Prefix
	for _, ref := range envoy.Spec.ACLFiles {
		cm := &corev1.ConfigMap{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: envoy.Namespace, Name: ref.Name}, cm); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, aclMissing(ref)
			}
			return nil, errx.WithHints(
				errx.WithReason(errACLSource.Wrap(err, "get ConfigMap %s/%s", envoy.Namespace, ref.Name), ReasonACLGetFailed),
				"Confirm the operator ServiceAccount has get permission on configmaps",
			)
		}
		data, ok := cm.Data[ref.Key]
		if !ok {
			return nil, aclMissing(ref)
		}
		prefixes, err := parseACL(data, ref.Name+"/"+ref.Key)
		if err != nil {
			return nil, err
		}
		all = append(all, prefixes...)
	}
	slices.SortFunc(all, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}
		return a.Bits() - b.Bits()
	})
	// Sorted by address then length, a covering prefix always precedes the ones it contains.
	merged := all[:0]
	for _, p := range all {
		if n := len(merged); n > 0 && merged[n-1].Bits() <= p.Bits() && merged[n-1].Contains(p.Addr()) {
			continue
		}
		merged = append(merged, p)
	}
	return merged, nil
}

func aclMissing(ref operatorv1alpha1.EnvoyConfigMapKeyRef) error {
	return errx.WithHints(
		errx.WithReason(errACLSource.New("ACL ConfigMap %s key %q not found", ref.Name, ref.Key), ReasonACLSourceMissing),
		"Create ConfigMap "+ref.Name+" with key "+ref.Key+"; the previous ACL stays in effect until then",
	)
}

func parseACL(data, source string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for i, line := range strings.Split(data, "\n") {
		entry, _, _ := strings.Cut(line, "#")
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		p, err := netip.ParsePrefix(entry)
		if err != nil {
			addr, addrErr := netip.ParseAddr(entry)
			if addrErr != nil {
				return nil, reconcile.TerminalError(errx.WithHints(
					errx.WithReason(errACLInvalid.New("%s line %d: %q is not an IP or CIDR", source, i+1, entry), ReasonACLEntryInvalid),
					"Fix or remove line "+strconv.Itoa(i+1)+" in "+source,
				))
			}
			p = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, p.Masked())
	}
	return prefixes, nil
}
