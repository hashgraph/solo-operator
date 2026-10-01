// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

func TestS6Controller_PodNotReady_DistinctType(t *testing.T) {
	cases := map[string]string{
		"container_not_created": `Internal error occurred: error executing command in container: container is not created or running`,
		"container_not_found":   `unable to upgrade connection: container not found ("consensus-node")`,
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestController(t, &fakeExecutor{streamErr: errors.New(msg)})
			_, err := c.exec(context.Background(), testCapsule(), s6QueryCmd)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errorx.IsOfType(err, errExecPodNotReady) {
				t.Errorf("expected errExecPodNotReady, got %v", err)
			}
			if !errorx.HasTrait(err, errorx.Temporary()) {
				t.Error("pod-not-ready error must be temporary (retryable)")
			}
			if reason, ok := errx.ReasonOf(err); !ok || reason != ReasonExecPodNotReady {
				t.Errorf("expected reason %s, got %q (ok=%v)", ReasonExecPodNotReady, reason, ok)
			}
		})
	}
}

func TestS6Controller_Forbidden_RBACHint(t *testing.T) {
	// The API server's 403 body as it arrives wrapped in a WebSocket upgrade error.
	forbidden := errors.New(`unable to upgrade streaming request: pods "n-0" is forbidden: ` +
		`User "system:serviceaccount:solo-operator:solo-operator" cannot get resource "pods/exec" in API group ""`)
	c := newTestController(t, &fakeExecutor{streamErr: forbidden})
	_, err := c.exec(context.Background(), testCapsule(), s6QueryCmd)
	if err == nil {
		t.Fatal("expected error on forbidden exec")
	}
	if reason, ok := errx.ReasonOf(err); !ok || reason != ReasonExecForbidden {
		t.Errorf("expected reason %s, got %q (ok=%v)", ReasonExecForbidden, reason, ok)
	}
	hints, ok := errx.Hints(err)
	if !ok || len(hints) == 0 {
		t.Fatal("expected RBAC remediation hints")
	}
	if !strings.Contains(strings.Join(hints, " "), "pods/exec") {
		t.Errorf("expected a pods/exec RBAC hint, got %v", hints)
	}
	// Forbidden is a config problem the controller recovers from once RBAC is
	// fixed, so it stays retryable like the other transport failures.
	if !errorx.HasTrait(err, errorx.Temporary()) {
		t.Error("forbidden error must be temporary (retryable)")
	}
}

type fakeS6Runner struct {
	isUp     bool
	startHit bool
	stopHit  bool
	err      error
}

func (f *fakeS6Runner) Start(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	f.startHit = true
	return f.err
}

func (f *fakeS6Runner) Stop(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	f.stopHit = true
	return f.err
}

func (f *fakeS6Runner) IsUp(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	return f.isUp, f.err
}

func TestActuationSubReconciler_SetsStatusRunState(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = operatorv1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	t.Run("sets_running_when_is_up", func(t *testing.T) {
		runner := &fakeS6Runner{isUp: true}
		rec := NewActuationSubReconciler(client, scheme, runner, nil)
		capsule := testCapsule()

		_, err := rec.Reconcile(context.Background(), capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capsule.Status.RunState != operatorv1alpha1.RunStateRunning {
			t.Errorf("expected RunStateRunning, got %v", capsule.Status.RunState)
		}
	})

	t.Run("sets_stopped_when_not_is_up", func(t *testing.T) {
		runner := &fakeS6Runner{isUp: false}
		rec := NewActuationSubReconciler(client, scheme, runner, nil)
		capsule := testCapsule()

		_, err := rec.Reconcile(context.Background(), capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capsule.Status.RunState != operatorv1alpha1.RunStateStopped {
			t.Errorf("expected RunStateStopped, got %v", capsule.Status.RunState)
		}
	})

	t.Run("starts_node_when_desired_running_and_not_up", func(t *testing.T) {
		runner := &fakeS6Runner{isUp: false}
		rec := NewActuationSubReconciler(client, scheme, runner, nil)
		capsule := testCapsule()
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning

		_, err := rec.Reconcile(context.Background(), capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !runner.startHit {
			t.Error("expected Start to be called")
		}
		if capsule.Status.RunState != operatorv1alpha1.RunStateRunning {
			t.Errorf("expected RunStateRunning after Start, got %v", capsule.Status.RunState)
		}
	})

	t.Run("stops_node_when_desired_stopped_and_is_up", func(t *testing.T) {
		runner := &fakeS6Runner{isUp: true}
		rec := NewActuationSubReconciler(client, scheme, runner, nil)
		capsule := testCapsule()
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateStopped

		_, err := rec.Reconcile(context.Background(), capsule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !runner.stopHit {
			t.Error("expected Stop to be called")
		}
		if capsule.Status.RunState != operatorv1alpha1.RunStateStopped {
			t.Errorf("expected RunStateStopped after Stop, got %v", capsule.Status.RunState)
		}
	})

	t.Run("pod_not_ready_requeues_quietly", func(t *testing.T) {
		podNotReadyErr := errExecPodNotReady.New("container not found")
		runner := &fakeS6Runner{err: podNotReadyErr}
		rec := NewActuationSubReconciler(client, scheme, runner, nil)
		capsule := testCapsule()
		capsule.Spec.DesiredRunState = operatorv1alpha1.RunStateRunning

		result, err := rec.Reconcile(context.Background(), capsule)
		if err != nil {
			t.Fatalf("expected no error (quiet requeue), got: %v", err)
		}
		if result.RequeueAfter != 2*time.Second {
			t.Errorf("expected RequeueAfter=2s, got %v", result.RequeueAfter)
		}
		if capsule.Status.RunState != operatorv1alpha1.RunStateStopped {
			t.Errorf("expected RunStateStopped during pod-not-ready, got %v", capsule.Status.RunState)
		}
	})
}
