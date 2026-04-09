// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/registry"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestApplyCredentials_nilCreds_noOp(t *testing.T) {
	rc, _ := registry.NewClient()
	var opts action.ChartPathOptions
	chart := operatorv1alpha1.HelmChartSpec{Repo: "https://charts.example.com", Chart: "myapp", Version: "1.0.0"}

	if err := applyCredentials(rc, &opts, chart, nil); err != nil {
		t.Fatalf("expected no error with nil creds, got %v", err)
	}
	if opts.Username != "" || opts.Password != "" {
		t.Error("expected empty username/password when creds is nil")
	}
}

func TestApplyCredentials_https_setsChartPathOptions(t *testing.T) {
	rc, _ := registry.NewClient()
	var opts action.ChartPathOptions
	chart := operatorv1alpha1.HelmChartSpec{Repo: "https://charts.example.com", Chart: "myapp", Version: "1.0.0"}
	creds := &ChartCredentials{Username: "user", Password: "pass"}

	if err := applyCredentials(rc, &opts, chart, creds); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Username != "user" {
		t.Errorf("expected username=user, got %q", opts.Username)
	}
	if opts.Password != "pass" {
		t.Errorf("expected password=pass, got %q", opts.Password)
	}
}

func TestApplyCredentials_https_doesNotCallRegistryLogin(t *testing.T) {
	// If OCI login were called for an HTTPS repo it would fail with a connection
	// error. This test verifies we return nil for plain HTTPS.
	rc, _ := registry.NewClient()
	var opts action.ChartPathOptions
	chart := operatorv1alpha1.HelmChartSpec{Repo: "https://charts.example.com", Chart: "myapp", Version: "1.0.0"}
	creds := &ChartCredentials{Username: "user", Password: "pass"}

	err := applyCredentials(rc, &opts, chart, creds)
	if err != nil {
		t.Fatalf("HTTPS creds should not trigger OCI login, got error: %v", err)
	}
}

func TestApplyCredentials_oci_attemptsLogin(t *testing.T) {
	// registry.Client.Login dials the registry; with a non-existent host it
	// returns a network error. That proves the OCI branch was taken.
	rc, _ := registry.NewClient()
	var opts action.ChartPathOptions
	chart := operatorv1alpha1.HelmChartSpec{Repo: "oci://registry.invalid.local", Chart: "myapp", Version: "1.0.0"}
	creds := &ChartCredentials{Username: "user", Password: "pass"}

	err := applyCredentials(rc, &opts, chart, creds)
	if err == nil {
		t.Fatal("expected OCI login to fail against a non-existent registry")
	}
	// HTTPS fields must not be set for OCI repos.
	if opts.Username != "" || opts.Password != "" {
		t.Error("expected ChartPathOptions to be empty for OCI repos")
	}
}
