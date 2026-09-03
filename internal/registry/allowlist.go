// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"net"
	"strings"

	"github.com/automa-saga/errx"
	"github.com/google/go-containerregistry/pkg/name"
)

// checkHostAllowed is the pre-flight SSRF gate on a user-controlled Image string:
// a fast, well-messaged reject before any network call. Empty allowedHosts denies
// only statically-private hosts (IP literals); non-empty requires an exact or
// subdomain match, denying everything else too.
//
// It is not the whole boundary: a DNS name that resolves to a private address is
// caught authoritatively at connect time by guardedTransport (dialguard.go), which
// closes the DNS-rebinding window this pre-flight check cannot.
func checkHostAllowed(image string, allowedHosts []string) error {
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil // caller's own ParseReference surfaces this
	}
	hostname := ref.Context().RegistryStr()
	if h, _, splitErr := net.SplitHostPort(hostname); splitErr == nil {
		hostname = h
	}

	if len(allowedHosts) > 0 {
		if hostAllowListed(hostname, allowedHosts) {
			return nil
		}
		return errx.WithHints(
			errx.WithReason(errHostDenied.New("registry host %q is not in the configured allow-list", hostname), ReasonRegistryHostDenied),
			"Add the registry host to --allowed-registry-hosts if this registry is intentional",
		)
	}

	if isPrivateHost(hostname) {
		return errx.WithHints(
			errx.WithReason(errHostDenied.New("registry host %q is a loopback/link-local/private-network address", hostname), ReasonRegistryHostDenied),
			"Configure --allowed-registry-hosts to explicitly permit internal registries",
		)
	}
	return nil
}

func hostAllowListed(hostname string, allowedHosts []string) bool {
	hostname = strings.ToLower(hostname)
	for _, allowed := range allowedHosts {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "" {
			continue
		}
		if hostname == allowed || strings.HasSuffix(hostname, "."+allowed) {
			return true
		}
	}
	return false
}

// isPrivateHost reports whether hostname is a statically-private IP literal.
// DNS names are deliberately not resolved here: doing so is racy (the dial
// re-resolves independently) and would add a blocking, context-less lookup to
// the reconcile path. guardedTransport (dialguard.go) validates resolved hosts
// authoritatively at connect time instead.
func isPrivateHost(hostname string) bool {
	ip := net.ParseIP(hostname)
	return ip != nil && isPrivateIP(ip)
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
