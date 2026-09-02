// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/automa-saga/errx"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// guardedTransport clones remote.DefaultTransport and replaces its dialer with
// one that validates the *post-resolution* IP of every connection — the primary
// request and any registry redirect (blob CDN, token endpoint, …). Because the
// check runs at connect time on the concrete IP the socket is about to use, it
// closes the DNS-rebinding / TOCTOU window a pre-flight name lookup cannot: a
// low-TTL host that resolves public during checkHostAllowed but private at dial
// time is still refused.
//
// Policy mirrors checkHostAllowed:
//   - empty allowedHosts (default): deny loopback/link-local/private targets;
//   - non-empty allowedHosts: a connection whose original hostname is allow-listed
//     may reach a private IP (operators opt internal registries in explicitly);
//     every other host is still denied a private target, so a redirect to an
//     internal address is blocked even in allow-list mode.
//
// Note: remote.DefaultTransport uses Proxy: http.ProxyFromEnvironment. When a
// forward proxy is configured the dialer connects to the proxy (the registry
// host travels in CONNECT), so this guard validates the proxy — egress control
// then shifts to the proxy, by design.
func guardedTransport(allowedHosts []string) http.RoundTripper {
	base := remote.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		allowPrivate := hostAllowListed(hostOnly(addr), allowedHosts)
		d := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				return denyPrivateConn(address, allowPrivate)
			},
		}
		return d.DialContext(ctx, network, addr)
	}
	return base
}

// denyPrivateConn is the connect-time gate. address is the resolved ip:port the
// dialer is about to connect to; net.Dialer invokes it once per candidate IP,
// after DNS resolution and before connect(2).
func denyPrivateConn(address string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	ip := net.ParseIP(hostOnly(address))
	if ip == nil {
		// Control always receives a literal IP; anything else is unexpected — fail closed.
		return errx.WithReason(
			errHostDenied.New("unexpected non-IP dial target %q", address), ReasonRegistryHostDenied)
	}
	if isPrivateIP(ip) {
		return errx.WithReason(
			errHostDenied.New("refusing to dial %s: loopback/link-local/private-network address", ip),
			ReasonRegistryHostDenied)
	}
	return nil
}

// hostOnly strips the port from a host:port, tolerating a bare host.
func hostOnly(hostPort string) string {
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		return h
	}
	return hostPort
}
