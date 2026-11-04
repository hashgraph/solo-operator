// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/haproxy"
)

var _ = Describe("HAProxyCapsule reload sources", func() {
	const ns = "haproxy-reload-test"
	ctx := context.Background()

	BeforeEach(func() { createNamespaceIgnoreExisting(ctx, ns) })

	checksumOf := func(name string) func() string {
		return func() string {
			d := &appsv1.Deployment{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, d); err != nil {
				return ""
			}
			return d.Spec.Template.Annotations[haproxy.ConfigChecksumAnnotation]
		}
	}

	It("re-renders the pod checksum when a referenced Secret or ConfigMap changes (Restart)", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "restart-tls", Namespace: ns},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"tls.crt": []byte("crt-v1"), "tls.key": []byte("key-v1")},
		}
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "node0-ofac-list", Namespace: ns},
			Data:       map[string]string{"list": "203.0.113.7\n"},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())

		capsule := &operatorv1alpha1.HAProxyCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: "haproxy-restart", Namespace: ns},
			Spec: operatorv1alpha1.HAProxyCapsuleSpec{
				Orbit:          ns,
				ServiceType:    operatorv1alpha1.HAProxyServiceTypeClusterIP,
				TLSSecretName:  secret.Name,
				ReloadStrategy: operatorv1alpha1.HAProxyReloadStrategyRestart,
				ACLFiles: []operatorv1alpha1.HAProxyACLFile{
					{Name: "ofac", ConfigMapRef: operatorv1alpha1.HAProxyConfigMapKeyRef{Name: cm.Name, Key: "list"}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, capsule)).To(Succeed())
		Eventually(checksumOf(capsule.Name), timeout, interval).ShouldNot(BeEmpty())
		initial := checksumOf(capsule.Name)()

		By("updating the ACL ConfigMap")
		cm.Data["list"] = "203.0.113.7\n198.51.100.1\n"
		Expect(k8sClient.Update(ctx, cm)).To(Succeed())
		Eventually(checksumOf(capsule.Name), timeout, interval).ShouldNot(Equal(initial))
		afterACL := checksumOf(capsule.Name)()

		By("rotating the TLS Secret")
		secret.Data["tls.crt"] = []byte("crt-v2")
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
		Eventually(checksumOf(capsule.Name), timeout, interval).ShouldNot(Equal(afterACL))

		Expect(k8sClient.Delete(ctx, capsule)).To(Succeed())
	})

	It("adds the reloader sidecar and no pod checksum by default (HotReload)", func() {
		capsule := &operatorv1alpha1.HAProxyCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: "haproxy-hot", Namespace: ns},
			Spec: operatorv1alpha1.HAProxyCapsuleSpec{
				Orbit:       ns,
				ServiceType: operatorv1alpha1.HAProxyServiceTypeClusterIP,
			},
		}
		Expect(k8sClient.Create(ctx, capsule)).To(Succeed())

		d := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: capsule.Name, Namespace: ns}, d)
		}, timeout, interval).Should(Succeed())
		Expect(d.Spec.Template.Spec.Containers).To(HaveLen(2))
		Expect(d.Spec.Template.Spec.Containers[1].Name).To(Equal(haproxy.ReloaderContainerName))
		Expect(d.Spec.Template.Annotations).NotTo(HaveKey(haproxy.ConfigChecksumAnnotation))

		Expect(k8sClient.Delete(ctx, capsule)).To(Succeed())
	})

	DescribeTable("rejects invalid specs",
		func(mutate func(*operatorv1alpha1.HAProxyCapsuleSpec), msg string) {
			capsule := &operatorv1alpha1.HAProxyCapsule{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "haproxy-invalid-", Namespace: ns},
				Spec:       operatorv1alpha1.HAProxyCapsuleSpec{Orbit: ns},
			}
			mutate(&capsule.Spec)
			err := k8sClient.Create(ctx, capsule)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(msg))
		},
		Entry("grpcs backend without TLS termination",
			func(s *operatorv1alpha1.HAProxyCapsuleSpec) {
				s.Backends = &operatorv1alpha1.HAProxyBackendsSpec{Grpcs: &operatorv1alpha1.HAProxyBackendSpec{}}
			}, "backends.grpcs requires tlsSecretName"),
		Entry("grpc tls without HTTPS",
			func(s *operatorv1alpha1.HAProxyCapsuleSpec) {
				s.Backends = &operatorv1alpha1.HAProxyBackendsSpec{Grpc: &operatorv1alpha1.HAProxyBackendSpec{
					TLS: &operatorv1alpha1.HAProxyBackendTLSSpec{},
				}}
			}, "backends.grpc.tls requires protocol HTTPS"),
		Entry("grpcs tls with explicit HTTP",
			func(s *operatorv1alpha1.HAProxyCapsuleSpec) {
				s.TLSSecretName = "tls"
				s.Backends = &operatorv1alpha1.HAProxyBackendsSpec{Grpcs: &operatorv1alpha1.HAProxyBackendSpec{
					Protocol: operatorv1alpha1.HAProxyBackendProtocolHTTP,
					TLS:      &operatorv1alpha1.HAProxyBackendTLSSpec{},
				}}
			}, "backends.grpcs.tls requires protocol HTTPS"),
		Entry("master CLI port collision",
			func(s *operatorv1alpha1.HAProxyCapsuleSpec) { s.MetricsPort = 9999 },
			"port 9999 is reserved"),
	)

	It("accepts tls on grpcs when protocol is omitted (defaults to HTTPS)", func() {
		capsule := &operatorv1alpha1.HAProxyCapsule{
			ObjectMeta: metav1.ObjectMeta{Name: "haproxy-grpcs-default", Namespace: ns},
			Spec: operatorv1alpha1.HAProxyCapsuleSpec{
				Orbit:         ns,
				ServiceType:   operatorv1alpha1.HAProxyServiceTypeClusterIP,
				TLSSecretName: "tls",
				Backends: &operatorv1alpha1.HAProxyBackendsSpec{Grpcs: &operatorv1alpha1.HAProxyBackendSpec{
					TLS: &operatorv1alpha1.HAProxyBackendTLSSpec{ServerName: "node1.example.com"},
				}},
			},
		}
		Expect(k8sClient.Create(ctx, capsule)).To(Succeed())
		Expect(k8sClient.Delete(ctx, capsule)).To(Succeed())
	})
})
