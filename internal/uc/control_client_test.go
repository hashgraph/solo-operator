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

// daemonkitMux mimics the two routes the daemonkit control server registers.
func daemonkitMux(status string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(status))
	})
	return mux
}

func TestControlClientStatusPretty(t *testing.T) {
	sock := serveOnSocket(t, daemonkitMux(`{"marker-monitor":{"state":"running"}}`))

	out, err := NewControlClient(sock).StatusPretty(context.Background())
	if err != nil {
		t.Fatalf("StatusPretty: %v", err)
	}
	if !strings.Contains(out, "marker-monitor") || !strings.Contains(out, "\n") {
		t.Fatalf("expected indented JSON containing marker-monitor, got %q", out)
	}
}

func TestControlClientHealth(t *testing.T) {
	sock := serveOnSocket(t, daemonkitMux("{}"))

	if err := NewControlClient(sock).Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestControlClientNon200(t *testing.T) {
	sock := serveOnSocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	c := NewControlClient(sock)
	if _, err := c.Status(context.Background()); err == nil {
		t.Fatal("Status on 500 = nil error, want error")
	}
	if err := c.Health(context.Background()); err == nil {
		t.Fatal("Health on 500 = nil error, want error")
	}
}

func TestControlClientNoSocket(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.sock")
	c := NewControlClient(missing)
	if _, err := c.Status(context.Background()); err == nil {
		t.Fatal("Status on missing socket = nil error, want dial error")
	}
	if err := c.Health(context.Background()); err == nil {
		t.Fatal("Health on missing socket = nil error, want dial error")
	}
}
