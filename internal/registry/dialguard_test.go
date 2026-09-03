// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"net/http"
	"testing"
)

func TestDenyPrivateConn(t *testing.T) {
	cases := []struct {
		name         string
		address      string
		allowPrivate bool
		wantErr      bool
	}{
		{"public IP denied-by-default posture", "93.184.216.34:443", false, false},
		{"cloud metadata endpoint", "169.254.169.254:80", false, true},
		{"loopback", "127.0.0.1:5000", false, true},
		{"rfc1918", "10.0.0.5:5000", false, true},
		{"unspecified", "0.0.0.0:5000", false, true},
		{"ipv6 loopback", "[::1]:443", false, true},
		{"ipv6 ula", "[fd00::1]:443", false, true},
		{"allow-listed host may reach a private IP", "10.0.0.5:5000", true, false},
		{"allow-listed host reaching public IP", "93.184.216.34:443", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := denyPrivateConn(tc.address, tc.allowPrivate)
			if tc.wantErr && err == nil {
				t.Fatalf("denyPrivateConn(%q, %v): expected denial, got nil", tc.address, tc.allowPrivate)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("denyPrivateConn(%q, %v): expected no denial, got %v", tc.address, tc.allowPrivate, err)
			}
		})
	}
}

// TestGuardedTransport_PolicyByHost verifies the per-connection policy: the
// original hostname decides whether a private target is permitted, so an
// allow-listed internal registry works while everything else is denied private
// targets (blocking rebinding and redirect-to-internal alike).
func TestGuardedTransport_PolicyByHost(t *testing.T) {
	tr, ok := guardedTransport([]string{"registry.corp.internal"}).(*http.Transport)
	if !ok || tr.DialContext == nil {
		t.Fatal("guardedTransport must return an *http.Transport with a custom DialContext")
	}

	// The connect-time decision is keyed off the dialed hostname.
	if !hostAllowListed(hostOnly("registry.corp.internal:5000"), []string{"registry.corp.internal"}) {
		t.Fatal("allow-listed host should be permitted to reach a private target")
	}
	if hostAllowListed(hostOnly("169.254.169.254:80"), []string{"registry.corp.internal"}) {
		t.Fatal("non-allow-listed host must not be permitted a private target")
	}
}
