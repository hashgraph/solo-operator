// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/joomcode/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// fakeExecutor is a remotecommand.Executor that writes canned stdout/stderr and
// returns a canned stream error, capturing nothing else — enough to drive the
// S6Controller branches without an API server.
type fakeExecutor struct {
	stdout    string
	stderr    string
	streamErr error
}

func (f *fakeExecutor) Stream(remotecommand.StreamOptions) error { return nil }

func (f *fakeExecutor) StreamWithContext(_ context.Context, opts remotecommand.StreamOptions) error {
	if opts.Stdout != nil && f.stdout != "" {
		_, _ = io.WriteString(opts.Stdout, f.stdout)
	}
	if opts.Stderr != nil && f.stderr != "" {
		_, _ = io.WriteString(opts.Stderr, f.stderr)
	}
	return f.streamErr
}

func newTestController(t *testing.T, fe *fakeExecutor) *S6Controller {
	t.Helper()
	// A real clientset over an unreachable config builds valid request URLs without
	// ever dialing the server — the faked executor short-circuits before any request
	// is sent. (The fake clientset's RESTClient cannot build request URLs.)
	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: "https://example.test"})
	if err != nil {
		t.Fatalf("build clientset: %v", err)
	}
	c := NewS6Controller(clientset, &rest.Config{Host: "https://example.test"})
	c.newExecutor = func(*rest.Config, string, string) (remotecommand.Executor, error) {
		return fe, nil
	}
	return c
}

func testCapsule() *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "solo"},
	}
}

func TestS6Controller_Start_Success(t *testing.T) {
	c := newTestController(t, &fakeExecutor{})
	if err := c.Start(context.Background(), testCapsule()); err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}
}

func TestS6Controller_IsUp(t *testing.T) {
	cases := map[string]struct {
		stdout string
		want   bool
	}{
		"up":              {stdout: "true\n", want: true},
		"down":            {stdout: "false\n", want: false},
		"whitespace_trim": {stdout: "  true  ", want: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestController(t, &fakeExecutor{stdout: tc.stdout})
			up, err := c.IsUp(context.Background(), testCapsule())
			if err != nil {
				t.Fatalf("IsUp: unexpected error: %v", err)
			}
			if up != tc.want {
				t.Errorf("IsUp = %v, want %v", up, tc.want)
			}
		})
	}
}

func TestS6Controller_CommandExit_Terminal(t *testing.T) {
	c := newTestController(t, &fakeExecutor{
		stderr:    "s6-svc: fatal: unable to control /run/service/consensus",
		streamErr: exec.CodeExitError{Err: errors.New("command terminated"), Code: 111},
	})
	_, err := c.exec(context.Background(), testCapsule(), s6StartCmd)
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
	if !errorx.IsOfType(err, errExecCommand) {
		t.Errorf("expected errExecCommand, got %v", err)
	}
	if errorx.HasTrait(err, errorx.Temporary()) {
		t.Error("command-exit error must be terminal, not temporary")
	}
}

func TestS6Controller_TransportError_Retryable(t *testing.T) {
	c := newTestController(t, &fakeExecutor{streamErr: errors.New("connection reset by peer")})
	_, err := c.exec(context.Background(), testCapsule(), s6QueryCmd)
	if err == nil {
		t.Fatal("expected error on transport failure")
	}
	if !errorx.IsOfType(err, errExecTransport) {
		t.Errorf("expected errExecTransport, got %v", err)
	}
	if !errorx.HasTrait(err, errorx.Temporary()) {
		t.Error("transport error must be temporary (retryable)")
	}
}
