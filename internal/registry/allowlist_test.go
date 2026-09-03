// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

func TestCheckHostAllowed_DefaultDeniesPrivateRanges(t *testing.T) {
	cases := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"loopback IP", "127.0.0.1/x:1", true},
		{"link-local metadata IP", "169.254.169.254/x:1", true},
		{"rfc1918 IP", "10.0.0.5:5000/x:1", true},
		{"public host", "ghcr.io/hashgraph/solo-operator:1.0.0", false},
		{"public host with port", "registry.example.com:443/x:1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkHostAllowed(tc.image, nil)
			if tc.wantErr && err == nil {
				t.Fatalf("expected denial for %q, got nil", tc.image)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no denial for %q, got %v", tc.image, err)
			}
		})
	}
}

func TestCheckHostAllowed_ExplicitAllowList(t *testing.T) {
	allowed := []string{"corp.internal", "ghcr.io"}

	cases := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"exact match", "ghcr.io/x:1", false},
		{"subdomain match", "registry.corp.internal:5000/x:1", false},
		{"public registry not listed", "docker.io/library/alpine:latest", true},
		{"private IP not listed", "10.0.0.5/x:1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkHostAllowed(tc.image, allowed)
			if tc.wantErr && err == nil {
				t.Fatalf("expected denial for %q, got nil", tc.image)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no denial for %q, got %v", tc.image, err)
			}
		})
	}
}

func TestCheckHostAllowed_MalformedImagePassesThrough(t *testing.T) {
	if err := checkHostAllowed("not a valid ref!!", nil); err != nil {
		t.Fatalf("malformed image should not be denied here (caller's ParseReference handles it): %v", err)
	}
}
