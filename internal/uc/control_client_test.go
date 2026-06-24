// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// serveOnSocket starts an HTTP server on a Unix socket in a temp dir and returns
// the socket path; the listener is closed when the test ends.
func serveOnSocket(t *testing.T, h http.Handler) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "uc.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return sock
}

func TestFetchStatusPrettyPrints(t *testing.T) {
	sock := serveOnSocket(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"marker-monitor":{"state":"running"}}`))
	}))

	out, err := FetchStatus(context.Background(), sock)
	if err != nil {
		t.Fatalf("FetchStatus: %v", err)
	}
	if !strings.Contains(out, "marker-monitor") || !strings.Contains(out, "\n") {
		t.Fatalf("expected indented JSON containing marker-monitor, got %q", out)
	}
}

func TestFetchStatusNon200(t *testing.T) {
	sock := serveOnSocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	if _, err := FetchStatus(context.Background(), sock); err == nil {
		t.Fatal("FetchStatus on 500 = nil error, want error")
	}
}

func TestFetchStatusNoSocket(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.sock")
	if _, err := FetchStatus(context.Background(), missing); err == nil {
		t.Fatal("FetchStatus on missing socket = nil error, want dial error")
	}
}
