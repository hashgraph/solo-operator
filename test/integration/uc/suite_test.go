// SPDX-License-Identifier: Apache-2.0

// Package uc_integration exercises the UC sidecar (internal/uc) against a real
// API server via envtest. Unlike the unit tests — which use the fake dynamic
// client that IGNORES field selectors — this suite validates the parts that
// only a real apiserver can: that the NetworkUpgrade* CRDs' spec.operationId
// selectableField is honoured server-side, and that the UC's dynamic-client
// dedup/handlers/recovery behave end-to-end (#1057).
//
// The controllers are deliberately NOT started here: the UC client behaviour is
// tested in isolation so no reconciler mutates the CR fixtures mid-test.
package uc_integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var (
	restCfg   *rest.Config
	dynClient dynamic.Interface
	testEnv   *envtest.Environment
)

// gvr builds the GroupVersionResource for a NetworkUpgrade* resource from the
// operator API group (the uc package's own GVR vars are unexported).
func gvr(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    operatorv1alpha1.GroupVersion.Group,
		Version:  operatorv1alpha1.GroupVersion.Version,
		Resource: resource,
	}
}

var (
	prepareGVR     = gvr("networkupgradeprepares")
	freezeAbortGVR = gvr("networkupgradefreezeaborts")
)

func TestMain(m *testing.M) {
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "uc integration suite setup failed:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

// run bootstraps envtest, wires the clients, creates the test namespace, runs
// the suite, and tears everything down. It returns via a named func so deferred
// teardown still executes before os.Exit in TestMain.
func run(m *testing.M) (int, error) {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	restCfg, err = testEnv.Start()
	if err != nil {
		return 0, fmt.Errorf("start envtest (is KUBEBUILDER_ASSETS set? run via `task test:integration`): %w", err)
	}
	defer func() { _ = testEnv.Stop() }()

	if err := operatorv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return 0, fmt.Errorf("add to scheme: %w", err)
	}

	dynClient, err = dynamic.NewForConfig(restCfg)
	if err != nil {
		return 0, fmt.Errorf("build dynamic client: %w", err)
	}

	return m.Run(), nil
}
