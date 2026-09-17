// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// s6 control-interface commands, full paths per HIP-1494 (the image's consensus/run
// documents these). The consensus s6 service is registered at /run/service/consensus.
const (
	s6SvcPath    = "/package/admin/s6/command/s6-svc"
	s6SvstatPath = "/package/admin/s6/command/s6-svstat"
	s6ServiceDir = "/run/service/consensus"
	s6UpValue    = "true" // s6-svstat -o up prints "true"/"false"
)

var (
	s6StartCmd = []string{s6SvcPath, "-o", s6ServiceDir} // -o: run once, no restart on exit
	s6StopCmd  = []string{s6SvcPath, "-d", s6ServiceDir}
	s6QueryCmd = []string{s6SvstatPath, "-o", "up", s6ServiceDir}
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// executorFactory is a struct field so tests can substitute a fake that streams
// canned output without a real API server.
type executorFactory func(cfg *rest.Config, method, url string) (remotecommand.Executor, error)

// newWebSocketExecutor uses the WebSocket exec protocol, available since k8s 1.32,
// which the chart requires (Chart.yaml kubeVersion).
func newWebSocketExecutor(cfg *rest.Config, method, url string) (remotecommand.Executor, error) {
	return remotecommand.NewWebSocketExecutor(cfg, method, url)
}

// S6Runner controls and inspects the s6-supervised consensus process.
type S6Runner interface {
	Start(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error
	Stop(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error
	IsUp(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error)
}

// S6Controller controls the s6 service via pods/exec.
type S6Controller struct {
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	newExecutor executorFactory
}

func NewS6Controller(clientset kubernetes.Interface, restConfig *rest.Config) *S6Controller {
	return &S6Controller{
		clientset:   clientset,
		restConfig:  restConfig,
		newExecutor: newWebSocketExecutor,
	}
}

func (c *S6Controller) Start(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	_, err := c.exec(ctx, capsule, s6StartCmd)
	return err
}

func (c *S6Controller) Stop(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error {
	_, err := c.exec(ctx, capsule, s6StopCmd)
	return err
}

func (c *S6Controller) IsUp(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	res, err := c.exec(ctx, capsule, s6QueryCmd)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(res.Stdout) == s6UpValue, nil
}

// exec streams command into the capsule's consensus container and returns the result.
// A non-zero exit is classified terminal (errExecCommand); a transport/stream failure
// is classified retryable (errExecTransport).
func (c *S6Controller) exec(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, command []string) (ExecResult, error) {
	podName := PodName(capsule)
	container := ConsensusContainerName(capsule)

	req := c.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(capsule.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	executor, err := c.newExecutor(c.restConfig, http.MethodPost, req.URL().String())
	if err != nil {
		return ExecResult{}, errx.WithHints(
			errx.WithReason(errExecTransport.Wrap(err, "build exec stream for %s/%s", capsule.Namespace, podName), ReasonExecTransportFailed),
			"Verify the operator's in-cluster REST config is valid",
		)
	}

	var stdout, stderr bytes.Buffer
	streamErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
	res := ExecResult{Stdout: stdout.String(), Stderr: stderr.String()}

	if streamErr != nil {
		// A non-zero command exit surfaces as exec.ExitError; anything else is a
		// transport/stream failure (RBAC forbidden, pod not ready, connection
		// reset, rescheduling).
		var exitErr exec.ExitError
		if errors.As(streamErr, &exitErr) {
			res.ExitCode = exitErr.ExitStatus()
			return res, errx.WithReason(
				errExecCommand.Wrap(streamErr, "s6 command %q exited %d: %s", strings.Join(command, " "), res.ExitCode, strings.TrimSpace(res.Stderr)),
				ReasonExecCommandFailed,
			)
		}
		// The WebSocket exec transport is authorized against `create`, not `get`, despite upgrading via HTTP GET.
		if isForbidden(streamErr) {
			return res, errx.WithHints(
				errx.WithReason(errExecTransport.Wrap(streamErr, "exec into %s/%s", capsule.Namespace, podName), ReasonExecForbidden),
				"Grant the operator ServiceAccount the `get` and `create` verbs on pods/exec "+
					`(ClusterRole rule apiGroups:[""] resources:["pods/exec"] verbs:["get","create"])`,
				"If installed via Helm, upgrade to a solo-operator chart whose RBAC includes the pods/exec get+create rule",
			)
		}
		return res, errx.WithHints(
			errx.WithReason(errExecTransport.Wrap(streamErr, "exec into %s/%s", capsule.Namespace, podName), ReasonExecTransportFailed),
			"Verify the pod is Running and the consensus-node container is ready",
		)
	}
	return res, nil
}

// isForbidden reports whether err is an API-server 403. The WebSocket exec
// transport does not always surface a typed *apierrors.StatusError — an upgrade
// failure arrives as a plain error whose message wraps the 403 body ("... is
// forbidden: ... cannot get resource \"pods/exec\" ...") — so fall back to a
// string check when the typed matcher misses.
func isForbidden(err error) bool {
	if apierrors.IsForbidden(err) {
		return true
	}
	return strings.Contains(err.Error(), "forbidden")
}

type ActuationSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
	s6       S6Runner
	recorder events.EventRecorder
}

func NewActuationSubReconciler(c client.Client, s *runtime.Scheme, s6 S6Runner, recorder events.EventRecorder) *ActuationSubReconciler {
	return &ActuationSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, s, "Actuation"),
		s6:                s6,
		recorder:          recorder,
	}
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (r *ActuationSubReconciler) ShouldReconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) bool {
	if r.s6 == nil {
		return false
	}

	pod := &corev1.Pod{}
	podName := PodName(capsule)
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: capsule.Namespace, Name: podName}, pod); err != nil {
		return false
	}

	return isPodReady(pod)
}

func (r *ActuationSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	if r.s6 == nil {
		return ctrl.Result{}, nil
	}

	isUp, err := r.s6.IsUp(ctx, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isUp {
		capsule.Status.RunState = operatorv1alpha1.RunStateRunning
	} else {
		capsule.Status.RunState = operatorv1alpha1.RunStateStopped
	}

	desired := capsule.Spec.DesiredRunState
	if desired == "" {
		return ctrl.Result{}, nil
	}

	switch {
	case desired == operatorv1alpha1.RunStateRunning && !isUp:
		if err := r.s6.Start(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
		capsule.Status.RunState = operatorv1alpha1.RunStateRunning
		if r.recorder != nil {
			r.recorder.Eventf(capsule, nil, corev1.EventTypeNormal,
				operatorv1alpha1.EventConsensusNodeStarted.String(),
				"Actuation", "Started s6 consensus service in pod %s", PodName(capsule))
		}
	case desired == operatorv1alpha1.RunStateStopped && isUp:
		if err := r.s6.Stop(ctx, capsule); err != nil {
			return ctrl.Result{}, err
		}
		capsule.Status.RunState = operatorv1alpha1.RunStateStopped
		if r.recorder != nil {
			r.recorder.Eventf(capsule, nil, corev1.EventTypeNormal,
				operatorv1alpha1.EventConsensusNodeStopped.String(),
				"Actuation", "Stopped s6 consensus service in pod %s", PodName(capsule))
		}
	}

	return ctrl.Result{}, nil
}
