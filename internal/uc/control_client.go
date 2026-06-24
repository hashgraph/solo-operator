// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// FetchStatus queries the UC control server's GET /status endpoint over its Unix
// socket and returns the response body pretty-printed as JSON. It is the backing
// call for the `uc status` subcommand, giving an operator the same per-monitor
// liveness the daemonkit control plane exposes without hand-rolling curl.
func FetchStatus(ctx context.Context, socketPath string) (string, error) {
	return fetch(ctx, socketPath, "/status")
}

// fetch performs a GET against the given path on the UC control socket. The host
// in the URL is ignored by the Unix-socket dialer but must be syntactically valid.
func fetch(ctx context.Context, socketPath, path string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://uc"+path, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("query %s on %s: %w", path, socketPath, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("control server returned %s: %s", resp.Status, string(body))
	}

	// Re-indent for human readability; fall back to the raw body if it is not JSON.
	var pretty json.RawMessage
	if err := json.Unmarshal(body, &pretty); err != nil {
		return string(body), nil
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return string(body), nil
	}
	return string(out), nil
}
