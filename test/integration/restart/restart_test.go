// SPDX-License-Identifier: Apache-2.0

package restart

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/controller"
	"github.com/hashgraph/solo-operator/internal/crashtest"
	"github.com/hashgraph/solo-operator/internal/registry"
	"github.com/hashgraph/solo-operator/internal/resources/helm"
	"github.com/hashgraph/solo-operator/internal/resources/helmtest"
)

const (
	timeout  = 30 * time.Second
	interval = 250 * time.Millisecond
	digest   = operatorv1alpha1.Sha256Digest("sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestRestart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Restart Safety Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(operatorv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

type operator struct {
	dead   atomic.Bool
	cancel context.CancelFunc
	done   chan struct{}
}

func (o *operator) stop() {
	o.cancel()
	<-o.done
}

func startOperator(setup func(ctrl.Manager), dieAfter func(client.Object) bool) *operator {
	op := &operator{done: make(chan struct{})}
	opts := ctrl.Options{
		Scheme:     scheme.Scheme,
		Metrics:    metricsserver.Options{BindAddress: "0"},
		Controller: config.Controller{SkipNameValidation: ptr.To(true)},
	}
	if dieAfter != nil {
		opts.NewClient = func(c *rest.Config, o client.Options) (client.Client, error) {
			base, err := client.NewWithWatch(c, o)
			if err != nil {
				return nil, err
			}
			return crashtest.Wrap(base, &op.dead, dieAfter), nil
		}
	}
	mgr, err := ctrl.NewManager(cfg, opts)
	Expect(err).NotTo(HaveOccurred())
	setup(mgr)

	ctx, cancel := context.WithCancel(context.Background())
	op.cancel = cancel
	go func() {
		defer GinkgoRecover()
		defer close(op.done)
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
	return op
}

func createNamespace(ctx context.Context, name string) {
	Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}))).To(Succeed())
}

type checker struct{ unreachable string }

func (c checker) Check(_ context.Context, spec operatorv1alpha1.ImageRegistrySpec, _ operatorv1alpha1.ImageDeterministicSpec) registry.RegistryResult {
	if spec.Image == c.unreachable {
		return registry.RegistryResult{Image: spec.Image, State: registry.StateUnreachable}
	}
	return registry.RegistryResult{Image: spec.Image, State: registry.StateValid, Digest: digest, LayerHashesVerified: true}
}

func setupExecute(chk checker) func(ctrl.Manager) {
	return func(mgr ctrl.Manager) {
		Expect(controller.RegisterPrepareIndex(mgr)).To(Succeed())
		Expect(controller.RegisterFreezeIndex(mgr)).To(Succeed())
		Expect(controller.RegisterExecuteIndex(mgr)).To(Succeed())
		r := controller.NewNetworkUpgradeExecuteReconciler(mgr.GetClient(), mgr.GetScheme())
		r.Checker = chk
		Expect(r.SetupWithManager(mgr)).To(Succeed())
	}
}

var _ = Describe("Operator restart mid-upgrade", func() {
	const (
		ns   = "restart-execute"
		opID = "restart-op"
	)
	ctx := context.Background()
	execKey := types.NamespacedName{Name: "execute", Namespace: ns}
	capsKey := types.NamespacedName{Name: "node0", Namespace: ns}

	It("keeps the pre-crash registry selection when killed right after patching the capsule", func() {
		createNamespace(ctx, ns)
		Expect(k8sClient.Create(ctx, &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
			Spec: operatorv1alpha1.OrbitSpec{Consensus: operatorv1alpha1.OrbitConsensus{
				Genesis: operatorv1alpha1.OrbitGenesis{AddressBook: operatorv1alpha1.OrbitAddressBook{LedgerId: "0x01"}},
			}},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &operatorv1alpha1.ConsensusCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: capsKey.Name, Namespace: ns, Labels: map[string]string{common.LabelOrbit: ns}},
			Spec: operatorv1alpha1.ConsensusCapsuleSpec{
				Orbit: ns, NodeId: 0, AccountId: "0.0.3", Weight: 1, Offline: true,
				Log4j2ConfigRef: "log4j2", SettingsConfigRef: "settings", ApplicationPropertiesRef: "app",
				PodProperties: operatorv1alpha1.ConsensusPodProperties{Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io/old", ImageName: "cn", ImageTag: "0.74.0"},
					},
				}},
			},
		})).To(Succeed())

		images := operatorv1alpha1.UpgradeImagesSpec{ConsensusNode: operatorv1alpha1.ImageComponentSpec{
			Version: "0.75.0",
			Registries: []operatorv1alpha1.ImageRegistrySpec{
				{Image: "reg-a.io/cn:0.75.0"}, {Image: "reg-b.io/cn:0.75.0"},
			},
		}}
		selected := operatorv1alpha1.SelectedSources{ConsensusNode: &operatorv1alpha1.SelectedSource{
			Image: "reg-a.io/cn:0.75.0", Digest: digest, LayerHashesVerified: true,
		}}
		execute := &operatorv1alpha1.NetworkUpgradeExecute{
			ObjectMeta: metav1.ObjectMeta{Name: execKey.Name, Namespace: ns, Labels: map[string]string{
				operatorv1alpha1.LabelOperationID: opID, operatorv1alpha1.LabelStep: operatorv1alpha1.StepExecute,
			}},
			Spec: operatorv1alpha1.NetworkUpgradeExecuteSpec{
				Orbit: ns, OperationID: opID, ConsensusCapsule: capsKey.Name, Images: images, SelectedSources: selected,
			},
		}
		Expect(k8sClient.Create(ctx, execute)).To(Succeed())
		now := metav1.Now()
		execute.Status.Phase = operatorv1alpha1.ExecutePhasePendingNodeUpgrade
		execute.Status.StartTime = &now
		execute.Status.NodeUpgradeStartTime = &now
		Expect(k8sClient.Status().Update(ctx, execute)).To(Succeed())

		By("running an operator that dies right after its capsule image patch")
		first := startOperator(setupExecute(checker{unreachable: "reg-a.io/cn:0.75.0"}), func(obj client.Object) bool {
			_, ok := obj.(*operatorv1alpha1.ConsensusCapsule)
			return ok
		})
		Eventually(first.dead.Load, timeout, interval).Should(BeTrue())
		first.stop()

		By("restarting with registry health inverted, so a re-selection would pick reg-a")
		second := startOperator(setupExecute(checker{unreachable: "reg-b.io/cn:0.75.0"}), nil)
		defer second.stop()

		Eventually(func(g Gomega) {
			got := &operatorv1alpha1.NetworkUpgradeExecute{}
			g.Expect(k8sClient.Get(ctx, execKey, got)).To(Succeed())
			g.Expect(got.Status.Phase).NotTo(Equal(operatorv1alpha1.ExecutePhaseFailed))
			g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, operatorv1alpha1.CondConsensusCapsulePatched)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		caps := &operatorv1alpha1.ConsensusCapsule{}
		Expect(k8sClient.Get(ctx, capsKey, caps)).To(Succeed())
		Expect(caps.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion.Repository).To(Equal("reg-b.io"))
	})
})

var _ = Describe("Operator restart mid-Helm-install", func() {
	const ns = "restart-helm"
	ctx := context.Background()
	key := types.NamespacedName{Name: "mirror", Namespace: ns}

	It("unlocks the release left pending by the killed operator and finishes the install", func() {
		createNamespace(ctx, ns)
		mock := helmtest.NewMockClient()
		started := make(chan struct{})
		var once sync.Once
		mock.InstallOrUpgradeFunc = func(ctx context.Context, name, namespace string, _ operatorv1alpha1.HelmChartSpec, _ map[string]interface{}, _ helm.ReleaseOptions) (*helm.ReleaseInfo, error) {
			mock.SetRelease(&helm.ReleaseInfo{Name: name, Namespace: namespace, Version: 1, Status: "pending-install", LastDeployed: time.Now()})
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil, ctx.Err()
		}
		setup := func(mgr ctrl.Manager) {
			Expect(controller.NewHelmCapsuleReconciler(mgr.GetClient(), mgr.GetScheme(), mock).SetupWithManager(mgr)).To(Succeed())
		}

		Expect(k8sClient.Create(ctx, &operatorv1alpha1.HelmCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: ns},
			Spec: operatorv1alpha1.HelmCapsuleSpec{
				OrbitRef: &corev1.LocalObjectReference{Name: ns},
				Chart:    operatorv1alpha1.HelmChartSpec{Repo: "oci://ghcr.io/hashgraph", Chart: "hedera-mirror", Version: "0.129.1"},
				Timeout:  &metav1.Duration{Duration: 2 * time.Second},
			},
		})).To(Succeed())

		By("killing the operator while helm install is in flight")
		first := startOperator(setup, nil)
		Eventually(started, timeout).Should(BeClosed())
		first.stop()
		mock.InstallOrUpgradeFunc = nil

		By("restarting; the stale pending release must be unlocked once its timeout passes")
		second := startOperator(setup, nil)
		defer second.stop()

		Eventually(func(g Gomega) {
			got := &operatorv1alpha1.HelmCapsule{}
			g.Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(operatorv1alpha1.PhaseSucceeded))
			g.Expect(got.Status.ReleaseStatus).To(Equal("deployed"))
		}, timeout, interval).Should(Succeed())
		info, err := mock.GetRelease(key.Name, ns)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Version).To(Equal(2))
	})
})
