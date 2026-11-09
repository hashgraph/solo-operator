// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("Proxy alongside a consensus upstream", Ordered, Label("proxy"), func() {
	const (
		ns        = "e2e-proxy"
		upstream  = "node0"
		proxyName = "envoy-node0"
		grpcPort  = int32(50211)
	)
	proxyKey := client.ObjectKey{Namespace: ns, Name: proxyName}
	upstreamKey := client.ObjectKey{Namespace: ns, Name: upstream}

	BeforeAll(func() {
		if !operatorReady {
			Skip("operator is not running; set up kind without E2E_SKIP_OPERATOR")
		}
		if err := createNamespace(ctx, ns); err != nil {
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue(), err.Error())
		}
		Expect(k8sClient.Create(ctx, &operatorv1alpha1.Orbit{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
			Spec:       operatorv1alpha1.OrbitSpec{Network: &operatorv1alpha1.OrbitNetwork{LedgerId: "0x03", ChainId: "298"}},
		})).To(Succeed())

		By("Deploying consensus-mock as the upstream node0")
		Expect(k8sClient.Create(ctx, mockDeployment(ns, upstream, grpcPort))).To(Succeed())
		Expect(k8sClient.Create(ctx, mockService(ns, upstream, grpcPort))).To(Succeed())
		waitRolledOut(upstreamKey)

		By("Creating the EnvoyProxy")
		Expect(k8sClient.Create(ctx, &operatorv1alpha1.EnvoyProxy{
			ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: ns},
			Spec: operatorv1alpha1.EnvoyProxySpec{
				Orbit:       ns,
				Replicas:    ptr.To[int32](1),
				ServiceType: operatorv1alpha1.EnvoyServiceTypeClusterIP,
				Listeners:   &operatorv1alpha1.EnvoyListenersSpec{GRPC: &operatorv1alpha1.EnvoyListenerSpec{}},
			},
		})).To(Succeed())
	})

	AfterAll(func() {
		_ = client.IgnoreNotFound(k8sClient.Delete(ctx, &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: ns}}))
		_ = deleteNamespace(ctx, ns)
	})

	It("creates proxy resources owned by the EnvoyProxy and becomes ready", func() {
		waitRolledOut(proxyKey)
		for _, obj := range []client.Object{&appsv1.Deployment{}, &corev1.Service{}} {
			Expect(k8sClient.Get(ctx, proxyKey, obj)).To(Succeed())
			Expect(controlledBy(obj, proxyName)).To(BeTrue(), "%T not owned by %s", obj, proxyName)
		}
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: proxyName + "-config"}, cm)).To(Succeed())
		Expect(controlledBy(cm, proxyName)).To(BeTrue())
	})

	It("routes HAPI gRPC through the proxy to node0", func() {
		Eventually(func() error { return queryVersion(ns, proxyName, grpcPort) }, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("returns UNAVAILABLE while node0 is down and recovers when it is back", func() {
		scale(upstreamKey, 0)
		Eventually(func() codes.Code {
			return status.Code(queryVersion(ns, proxyName, grpcPort))
		}, 2*time.Minute, 5*time.Second).Should(Equal(codes.Unavailable))

		deploy := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, proxyKey, deploy)).To(Succeed())
		Expect(deploy.Status.AvailableReplicas).To(Equal(int32(1)), "proxy must stay available without an upstream")

		scale(upstreamKey, 1)
		waitRolledOut(upstreamKey)
		Eventually(func() error { return queryVersion(ns, proxyName, grpcPort) }, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("applies probe overrides and disables probes", func() {
		updateProxy(proxyKey, func(p *operatorv1alpha1.EnvoyProxy) {
			p.Spec.Probes = &operatorv1alpha1.EnvoyProbesSpec{
				Startup:   &operatorv1alpha1.EnvoyProbeSpec{FailureThreshold: ptr.To[int32](720)},
				Readiness: &operatorv1alpha1.EnvoyProbeSpec{PeriodSeconds: ptr.To[int32](7), FailureThreshold: ptr.To[int32](4)},
			}
		})
		Eventually(func(g Gomega) {
			c := proxyContainer(g, proxyKey)
			g.Expect(c.StartupProbe).NotTo(BeNil())
			g.Expect(c.StartupProbe.FailureThreshold).To(Equal(int32(720)))
			g.Expect(c.ReadinessProbe).NotTo(BeNil())
			g.Expect(c.ReadinessProbe.PeriodSeconds).To(Equal(int32(7)))
			g.Expect(c.ReadinessProbe.FailureThreshold).To(Equal(int32(4)))
		}, shortTimeout, interval).Should(Succeed())
		waitRolledOut(proxyKey)

		updateProxy(proxyKey, func(p *operatorv1alpha1.EnvoyProxy) {
			p.Spec.Probes = &operatorv1alpha1.EnvoyProbesSpec{Enabled: ptr.To(false)}
		})
		Eventually(func(g Gomega) {
			c := proxyContainer(g, proxyKey)
			g.Expect(c.StartupProbe).To(BeNil())
			g.Expect(c.LivenessProbe).To(BeNil())
			g.Expect(c.ReadinessProbe).To(BeNil())
		}, shortTimeout, interval).Should(Succeed())
		waitRolledOut(proxyKey)

		updateProxy(proxyKey, func(p *operatorv1alpha1.EnvoyProxy) { p.Spec.Probes = nil })
		Eventually(func(g Gomega) {
			c := proxyContainer(g, proxyKey)
			g.Expect(c.ReadinessProbe).NotTo(BeNil())
			g.Expect(c.ReadinessProbe.PeriodSeconds).To(Equal(int32(5)))
		}, shortTimeout, interval).Should(Succeed())
		waitRolledOut(proxyKey)
	})

	It("does not churn the Deployment spec while idle", func() {
		deploy := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, proxyKey, deploy)).To(Succeed())
		gen := deploy.Generation
		Consistently(func() int64 {
			Expect(k8sClient.Get(ctx, proxyKey, deploy)).To(Succeed())
			return deploy.Generation
		}, 20*time.Second, interval).Should(Equal(gen))
	})

	It("removes the proxy and its resources when the Orbit is deleted", func() {
		Expect(k8sClient.Delete(ctx, &operatorv1alpha1.Orbit{ObjectMeta: metav1.ObjectMeta{Name: ns}})).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, proxyKey, &operatorv1alpha1.EnvoyProxy{}))
		}, timeout, interval).Should(BeTrue())

		for _, obj := range []client.Object{&appsv1.Deployment{}, &corev1.Service{}} {
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, proxyKey, obj))
			}, timeout, interval).Should(BeTrue(), "%T should be garbage-collected", obj)
		}
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: proxyName + "-config"}, &corev1.ConfigMap{}))
		}, timeout, interval).Should(BeTrue())

		Expect(k8sClient.Get(ctx, upstreamKey, &appsv1.Deployment{})).To(Succeed(), "upstream is not orbit-owned")
	})
})

func mockDeployment(ns, name string, port int32) *appsv1.Deployment {
	labels := map[string]string{"app": name + "-mock"}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "consensus-mock",
						Image:           mockImage,
						ImagePullPolicy: corev1.PullNever,
						Command:         []string{"/consensus-mock"},
						Args:            []string{"--upgrade-path=/data/upgrade", "--consensus-log-file=/data/output/swirlds.log"},
						Ports:           []corev1.ContainerPort{{Name: "grpc", ContainerPort: port}},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler:  corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(port)}},
							PeriodSeconds: 2,
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
					}},
					Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
				},
			},
		},
	}
}

func mockService(ns, name string, port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name + "-mock"},
			Ports:    []corev1.ServicePort{{Name: "grpc", Port: port, TargetPort: intstr.FromInt32(port)}},
		},
	}
}

func controlledBy(obj client.Object, owner string) bool {
	ref := metav1.GetControllerOf(obj)
	return ref != nil && ref.Kind == "EnvoyProxy" && ref.Name == owner
}

func waitRolledOut(key client.ObjectKey) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		d := &appsv1.Deployment{}
		g.Expect(k8sClient.Get(ctx, key, d)).To(Succeed())
		want := ptr.Deref(d.Spec.Replicas, 1)
		g.Expect(d.Status.ObservedGeneration).To(BeNumerically(">=", d.Generation))
		g.Expect(d.Status.UpdatedReplicas).To(Equal(want))
		g.Expect(d.Status.AvailableReplicas).To(Equal(want))
		g.Expect(d.Status.Replicas).To(Equal(want))
	}, timeout, interval).Should(Succeed())
}

func scale(key client.ObjectKey, replicas int32) {
	GinkgoHelper()
	Eventually(func() error {
		d := &appsv1.Deployment{}
		if err := k8sClient.Get(ctx, key, d); err != nil {
			return err
		}
		d.Spec.Replicas = ptr.To(replicas)
		return k8sClient.Update(ctx, d)
	}, shortTimeout, interval).Should(Succeed())
}

func updateProxy(key client.ObjectKey, mutate func(*operatorv1alpha1.EnvoyProxy)) {
	GinkgoHelper()
	Eventually(func() error {
		p := &operatorv1alpha1.EnvoyProxy{}
		if err := k8sClient.Get(ctx, key, p); err != nil {
			return err
		}
		mutate(p)
		return k8sClient.Update(ctx, p)
	}, shortTimeout, interval).Should(Succeed())
}

func proxyContainer(g Gomega, key client.ObjectKey) corev1.Container {
	d := &appsv1.Deployment{}
	g.Expect(k8sClient.Get(ctx, key, d)).To(Succeed())
	g.Expect(d.Spec.Template.Spec.Containers).NotTo(BeEmpty())
	return d.Spec.Template.Spec.Containers[0]
}

func queryVersion(ns, svc string, port int32) error {
	addr, stop, err := portForward(ns, svc, port)
	if err != nil {
		return err
	}
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := services.NewNetworkServiceClient(conn).GetVersionInfo(cctx, &services.Query{})
	if err != nil {
		return err
	}
	if resp.GetNetworkGetVersionInfo().GetHapiProtoVersion() == nil {
		return fmt.Errorf("unexpected response: %v", resp)
	}
	return nil
}

var forwardingRe = regexp.MustCompile(`Forwarding from 127\.0\.0\.1:(\d+)`)

func portForward(ns, svc string, port int32) (addr string, stop func(), err error) {
	cmd := exec.Command("kubectl", "port-forward", "-n", ns, "svc/"+svc, fmt.Sprintf(":%d", port))
	if testEnv.KubeconfigPath != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+testEnv.KubeconfigPath)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	cmd.Stderr = GinkgoWriter
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	stop = func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }

	found := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if m := forwardingRe.FindStringSubmatch(sc.Text()); m != nil {
				found <- m[1]
				break
			}
		}
		_, _ = io.Copy(io.Discard, out)
	}()
	select {
	case p := <-found:
		return "127.0.0.1:" + p, stop, nil
	case <-time.After(15 * time.Second):
		stop()
		return "", nil, fmt.Errorf("port-forward to svc/%s did not become ready", svc)
	}
}
