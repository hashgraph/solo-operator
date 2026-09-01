// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"net"
	"strings"

	"github.com/automa-saga/errx"
	"github.com/google/go-containerregistry/pkg/name"
)

// checkHostAllowed guards against SSRF via a user-controlled Image string.
// Empty allowedHosts denies only private/loopback hosts; non-empty requires
// an exact or subdomain match, denying everything else too.
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

// isPrivateHost resolves hostname (DNS names too, so in-cluster Service DNS
// is caught, not just IP literals) and reports whether it lands private.
func isPrivateHost(hostname string) bool {
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip)
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return false // unresolvable now will just fail Unreachable later
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true
		}
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
