// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/controller"
	"github.com/hashgraph/solo-operator/internal/registry"
	"github.com/hashgraph/solo-operator/internal/resources/helmtest"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Start manager
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// Setup controllers
	err = controller.NewOrbitReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewConsensusCapsuleReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewHAProxyCapsuleReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewEnvoyProxyReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewNetworkGenesisReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewHelmCapsuleReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
		helmtest.NewMockClient(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewLog4j2ConfigReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewNodeSettingsReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = controller.NewApplicationPropertiesReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	Expect(controller.RegisterPrepareIndex(k8sManager)).To(Succeed())

	prepareReconciler := controller.NewNetworkUpgradePrepareReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	)
	prepareReconciler.StrategyName = registry.StrategySequential
	prepareReconciler.Checker = selectiveImageChecker{}
	Expect(prepareReconciler.SetupWithManager(k8sManager)).To(Succeed())

	freezeReconciler := controller.NewNetworkUpgradeFreezeReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	)
	freezeReconciler.Checker = selectiveImageChecker{}
	Expect(freezeReconciler.SetupWithManager(k8sManager)).To(Succeed())

	Expect(controller.NewNetworkUpgradeFreezeAbortReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	).SetupWithManager(k8sManager)).To(Succeed())

	Expect(controller.RegisterFreezeIndex(k8sManager)).To(Succeed())
	Expect(controller.RegisterExecuteIndex(k8sManager)).To(Succeed())
	Expect(controller.RegisterFreezeAbortIndex(k8sManager)).To(Succeed())

	executeReconciler := controller.NewNetworkUpgradeExecuteReconciler(
		k8sManager.GetClient(),
		k8sManager.GetScheme(),
	)
	executeReconciler.Checker = selectiveImageChecker{}
	Expect(executeReconciler.SetupWithManager(k8sManager)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// Helper constants for tests
const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)
