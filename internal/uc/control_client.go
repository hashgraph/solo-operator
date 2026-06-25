// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// defaultControlTimeout bounds a single control-socket request.
const defaultControlTimeout = 5 * time.Second

// ControlClient talks to a running UC sidecar over its daemonkit control socket.
// It exposes one method per endpoint the server registers — currently GET /health
// and GET /status — so the `uc` binary (and tests) have the same coverage of the
// control plane that solo-provisioner-daemon's client has of its daemon socket.
// New endpoints added to the server should gain a matching method here.
type ControlClient struct {
	socketPath string
	http       *http.Client
}

// NewControlClient builds a client bound to the UC control socket at socketPath.
func NewControlClient(socketPath string) *ControlClient {
	return &ControlClient{
		socketPath: socketPath,
		http: &http.Client{
			Timeout: defaultControlTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
	}
}

// Health queries GET /health. It returns nil when the process is live (HTTP 200)
// and an error otherwise (unreachable socket or non-200 response).
func (c *ControlClient) Health(ctx context.Context) error {
	_, err := c.get(ctx, "/health")
	return err
}

// Status queries GET /status and returns the raw JSON body. Use StatusPretty for
// human-facing output.
func (c *ControlClient) Status(ctx context.Context) ([]byte, error) {
	return c.get(ctx, "/status")
}

// StatusPretty queries GET /status and returns the body indented for display,
// falling back to the raw body if it is not valid JSON.
func (c *ControlClient) StatusPretty(ctx context.Context) (string, error) {
	body, err := c.Status(ctx)
	if err != nil {
		return "", err
	}
	return prettyJSON(body), nil
}

// get performs a GET against path on the control socket and returns the body.
// A non-200 status is reported as an error including the body for context. The
// host in the URL is ignored by the Unix-socket dialer but must be syntactically
// valid.
func (c *ControlClient) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://uc"+path, http.NoBody)
	if err != nil {
		return nil, errx.WithReason(errControl.Wrap(err, "build request"), "ControlRequestFailed")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errx.WithHints(
			errx.WithReason(errControl.Wrap(err, "query %s on %s", path, c.socketPath), "ControlDialFailed"),
			"Confirm the UC sidecar is running and --socket-path matches its control socket",
		)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errx.WithReason(errControl.Wrap(err, "read response"), "ControlReadFailed")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errx.WithReason(
			errControl.New("control server returned %s: %s", resp.Status, string(body)), "ControlBadStatus")
	}
	return body, nil
}

// prettyJSON re-indents a JSON body for human readability, returning the input
// unchanged if it does not parse as JSON.
func prettyJSON(body []byte) string {
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return string(body)
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(out)
}
