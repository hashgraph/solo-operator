// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	// kindClusterName is the name of the kind cluster for E2E tests
	kindClusterName = "solo-operator-e2e"

	// Timeouts for E2E tests (longer than integration tests)
	timeout  = time.Minute * 5
	interval = time.Second * 2

	// shortTimeout for quick checks
	shortTimeout = time.Second * 30
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *E2ETestEnvironment
	ctx       context.Context
	cancel    context.CancelFunc
)

// E2ETestEnvironment manages the E2E test environment
type E2ETestEnvironment struct {
	ClusterName    string
	KubeconfigPath string
	CRDsInstalled  bool
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("Setting up E2E test environment")
	testEnv = &E2ETestEnvironment{
		ClusterName: kindClusterName,
	}

	// Check if we should use an existing cluster
	useExistingCluster := os.Getenv("USE_EXISTING_CLUSTER") == "true"

	if useExistingCluster {
		By("Using existing Kubernetes cluster")
		var err error
		cfg, err = ctrl.GetConfig()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
	} else {
		By("Creating kind cluster for E2E tests")
		err := testEnv.CreateKindCluster()
		if err != nil {
			// Cluster might already exist, try to use it
			By("Kind cluster may already exist, attempting to use it")
			err = testEnv.UseExistingKindCluster()
			Expect(err).NotTo(HaveOccurred())
		}

		cfg, err = testEnv.GetKubeconfig()
		Expect(err).NotTo(HaveOccurred())
	}

	// Create scheme
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())
	err = operatorv1alpha1.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	// Create client
	k8sClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("Installing CRDs")
	err = testEnv.InstallCRDs()
	Expect(err).NotTo(HaveOccurred())

	By("Verifying cluster connectivity")
	nodeList := &corev1.NodeList{}
	err = k8sClient.List(ctx, nodeList)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(nodeList.Items)).To(BeNumerically(">", 0))
	fmt.Fprintf(GinkgoWriter, "Connected to cluster with %d nodes\n", len(nodeList.Items))
})

var _ = AfterSuite(func() {
	cancel()

	// Only delete cluster if we created it
	if os.Getenv("USE_EXISTING_CLUSTER") != "true" && os.Getenv("KEEP_CLUSTER") != "true" {
		By("Cleaning up kind cluster")
		if testEnv != nil {
			_ = testEnv.DeleteKindCluster()
		}
	}
})

// CreateKindCluster creates a new kind cluster for E2E tests
func (e *E2ETestEnvironment) CreateKindCluster() error {
	cmd := exec.Command("kind", "create", "cluster", "--name", e.ClusterName, "--wait", "60s")
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	return cmd.Run()
}

// UseExistingKindCluster configures the environment to use an existing kind cluster
func (e *E2ETestEnvironment) UseExistingKindCluster() error {
	// Verify cluster exists
	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get kind clusters: %w", err)
	}

	if !contains(string(output), e.ClusterName) {
		return fmt.Errorf("kind cluster %s not found", e.ClusterName)
	}

	return nil
}

// DeleteKindCluster deletes the kind cluster
func (e *E2ETestEnvironment) DeleteKindCluster() error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", e.ClusterName)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	return cmd.Run()
}

// GetKubeconfig returns the kubeconfig for the kind cluster
func (e *E2ETestEnvironment) GetKubeconfig() (*rest.Config, error) {
	// Get kubeconfig from kind
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", e.ClusterName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Write to temp file
	tmpDir := os.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, fmt.Sprintf("kubeconfig-%s", e.ClusterName))
	err = os.WriteFile(kubeconfigPath, output, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	e.KubeconfigPath = kubeconfigPath

	// Load config
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// InstallCRDs installs the operator CRDs into the cluster
func (e *E2ETestEnvironment) InstallCRDs() error {
	// Find CRD directory
	crdDir := findCRDDirectory()
	if crdDir == "" {
		return fmt.Errorf("CRD directory not found")
	}

	fmt.Fprintf(GinkgoWriter, "Installing CRDs from: %s\n", crdDir)

	// Apply CRDs using kubectl with server-side apply to handle large CRDs
	// The ConsensusCapsule CRD is too large for client-side apply
	cmd := exec.Command("kubectl", "apply", "--server-side", "--force-conflicts", "-f", crdDir)
	if e.KubeconfigPath != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", e.KubeconfigPath))
	}
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install CRDs: %w", err)
	}

	e.CRDsInstalled = true

	// Wait for CRDs to be established
	time.Sleep(2 * time.Second)
	return nil
}

// findCRDDirectory finds the CRD directory relative to the test file
func findCRDDirectory() string {
	// Try common locations
	paths := []string{
		"../../config/crd/bases",
		"../../../config/crd/bases",
		"config/crd/bases",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

	// Try from working directory
	wd, _ := os.Getwd()
	for _, p := range paths {
		full := filepath.Join(wd, p)
		if _, err := os.Stat(full); err == nil {
			return full
		}
	}

	return ""
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper functions for tests

// createNamespace creates a namespace and waits for it to be ready
func createNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{}
	ns.Name = name

	err := k8sClient.Create(ctx, ns)
	if err != nil {
		return err
	}

	// Wait for namespace to be active
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, ns); err != nil {
			return false
		}
		return ns.Status.Phase == corev1.NamespaceActive
	}, shortTimeout, interval).Should(BeTrue())

	return nil
}

// deleteNamespace deletes a namespace
func deleteNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{}
	ns.Name = name
	return client.IgnoreNotFound(k8sClient.Delete(ctx, ns))
}
