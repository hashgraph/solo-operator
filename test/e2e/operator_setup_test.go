// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	operatorNamespace = "solo-operator"
	operatorImage     = "ghcr.io/hashgraph/solo-operator/solo-operator:dev"
	mockImage         = "ghcr.io/hashgraph/solo-operator/consensus-mock:e2e"
)

var operatorReady bool

// InstallOperator leaves CRDs to InstallCRDs, so the chart's CRD subchart is disabled.
func (e *E2ETestEnvironment) InstallOperator() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	if os.Getenv("E2E_SKIP_BUILD") != "true" {
		if err := e.run(root, "task", "docker:build"); err != nil {
			return err
		}
		if err := e.run(root, "task", "-d", "test/consensus-mock", "docker:build:e2e", "MOCK_IMG="+mockImage); err != nil {
			return err
		}
	}
	for _, img := range []string{operatorImage, mockImage} {
		if err := e.run(root, "kind", "load", "docker-image", img, "--name", e.ClusterName); err != nil {
			return err
		}
	}
	return e.run(root, "helm", "upgrade", "--install", "solo-operator", "charts/solo-operator",
		"--kubeconfig", e.KubeconfigPath,
		"--namespace", operatorNamespace, "--create-namespace",
		"--wait", "--timeout", "180s",
		"--set", "image.tag=dev",
		"--set", "image.pullPolicy=Never",
		"--set", "crds.install=false",
	)
}

func (e *E2ETestEnvironment) run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	if e.KubeconfigPath != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+e.KubeconfigPath)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

func operatorRunning(ctx context.Context) bool {
	deploys := &appsv1.DeploymentList{}
	if err := k8sClient.List(ctx, deploys, client.InNamespace(operatorNamespace),
		client.MatchingLabels{"app.kubernetes.io/name": "solo-operator"}); err != nil {
		return false
	}
	for _, d := range deploys.Items {
		if d.Status.AvailableReplicas > 0 {
			return true
		}
	}
	return false
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
