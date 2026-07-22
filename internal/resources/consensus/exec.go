// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
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

// S6Controller starts, stops, and queries the consensus s6 service in a capsule's
// pod (StatefulSet ordinal 0) via pods/exec.
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
			"Confirm the operator ServiceAccount has create on pods/exec",
		)
	}

	var stdout, stderr bytes.Buffer
	streamErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
	res := ExecResult{Stdout: stdout.String(), Stderr: stderr.String()}

	if streamErr != nil {
		// A non-zero command exit surfaces as exec.ExitError; anything else is a
		// transport/stream failure (pod not ready, connection reset, rescheduling).
		var exitErr exec.ExitError
		if errors.As(streamErr, &exitErr) {
			res.ExitCode = exitErr.ExitStatus()
			return res, errx.WithReason(
				errExecCommand.Wrap(streamErr, "s6 command %q exited %d: %s", strings.Join(command, " "), res.ExitCode, strings.TrimSpace(res.Stderr)),
				ReasonExecCommandFailed,
			)
		}
		return res, errx.WithHints(
			errx.WithReason(errExecTransport.Wrap(streamErr, "exec into %s/%s", capsule.Namespace, podName), ReasonExecTransportFailed),
			"Verify the pod is Running and the consensus-node container is ready",
		)
	}
	return res, nil
}
