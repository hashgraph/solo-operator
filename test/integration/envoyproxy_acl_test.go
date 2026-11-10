// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/envoy"
)

var _ = Describe("EnvoyProxy ACL files", func() {
	const ns = "envoy-acl-test"
	ctx := context.Background()

	BeforeEach(func() { createNamespaceIgnoreExisting(ctx, ns) })

	lds := func(name string) func() string {
		return func() string {
			cm := &corev1.ConfigMap{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-config", Namespace: ns}, cm); err != nil {
				return ""
			}
			return cm.Data[envoy.LDSFileName]
		}
	}
	checksum := func(name string) string {
		d := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, d)).To(Succeed())
		return d.Spec.Template.Annotations[envoy.ConfigChecksumAnnotation]
	}

	It("re-renders the listeners when the ACL ConfigMap changes, without rolling the pod", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "node0-ofac-list", Namespace: ns},
			Data:       map[string]string{"blocklist.acl": "# ofac\n203.0.113.7\n"},
		}
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())

		proxy := &operatorv1alpha1.EnvoyProxy{
			ObjectMeta: metav1.ObjectMeta{Name: "envoy-node0", Namespace: ns},
			Spec: operatorv1alpha1.EnvoyProxySpec{
				Orbit:       ns,
				ServiceType: operatorv1alpha1.EnvoyServiceTypeClusterIP,
				Listeners:   &operatorv1alpha1.EnvoyListenersSpec{GRPC: &operatorv1alpha1.EnvoyListenerSpec{}},
				ACLFiles:    []operatorv1alpha1.EnvoyConfigMapKeyRef{{Name: cm.Name, Key: "blocklist.acl"}},
			},
		}
		Expect(k8sClient.Create(ctx, proxy)).To(Succeed())
		Eventually(lds(proxy.Name), timeout, interval).Should(ContainSubstring("address_prefix: '203.0.113.7'"))
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: proxy.Name, Namespace: ns}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())
		before := checksum(proxy.Name)

		cm.Data["blocklist.acl"] = "203.0.113.7\n198.51.100.0/24\n"
		Expect(k8sClient.Update(ctx, cm)).To(Succeed())
		Eventually(lds(proxy.Name), timeout, interval).Should(ContainSubstring("address_prefix: '198.51.100.0'"))
		Expect(checksum(proxy.Name)).To(Equal(before))

		By("keeping the last rendered list when the ConfigMap disappears")
		Expect(k8sClient.Delete(ctx, cm)).To(Succeed())
		Consistently(lds(proxy.Name), "2s", interval).Should(ContainSubstring("address_prefix: '198.51.100.0'"))

		Expect(k8sClient.Delete(ctx, proxy)).To(Succeed())
	})

	It("reports a missing ACL ConfigMap on the Reconciled condition", func() {
		proxy := &operatorv1alpha1.EnvoyProxy{
			ObjectMeta: metav1.ObjectMeta{Name: "envoy-node1", Namespace: ns},
			Spec: operatorv1alpha1.EnvoyProxySpec{
				Orbit:       ns,
				ServiceType: operatorv1alpha1.EnvoyServiceTypeClusterIP,
				ACLFiles:    []operatorv1alpha1.EnvoyConfigMapKeyRef{{Name: "node1-ofac-list", Key: "blocklist.acl"}},
			},
		}
		Expect(k8sClient.Create(ctx, proxy)).To(Succeed())
		reconciled := func() *metav1.Condition {
			p := &operatorv1alpha1.EnvoyProxy{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: proxy.Name, Namespace: ns}, p); err != nil {
				return nil
			}
			return apimeta.FindStatusCondition(p.Status.Conditions, operatorv1alpha1.EnvoyProxyConditionTypeReconciled)
		}
		Eventually(reconciled, timeout, interval).Should(And(
			Not(BeNil()),
			HaveField("Status", metav1.ConditionFalse),
			HaveField("Reason", "ACLSourceMissing"),
		))

		Expect(k8sClient.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "node1-ofac-list", Namespace: ns},
			Data:       map[string]string{"blocklist.acl": "203.0.113.7\n"},
		})).To(Succeed())
		Eventually(reconciled, timeout, interval).Should(HaveField("Status", metav1.ConditionTrue))

		Expect(k8sClient.Delete(ctx, proxy)).To(Succeed())
	})

	DescribeTable("rejects invalid specs",
		func(mutate func(*operatorv1alpha1.EnvoyProxySpec), msg string) {
			proxy := &operatorv1alpha1.EnvoyProxy{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "envoy-invalid-", Namespace: ns},
				Spec:       operatorv1alpha1.EnvoyProxySpec{Orbit: ns},
			}
			mutate(&proxy.Spec)
			err := k8sClient.Create(ctx, proxy)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(msg))
		},
		Entry("ACL with a Cluster traffic policy on a LoadBalancer",
			func(s *operatorv1alpha1.EnvoyProxySpec) {
				s.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyCluster
				s.ACLFiles = []operatorv1alpha1.EnvoyConfigMapKeyRef{{Name: "a", Key: "b"}}
			}, "externalTrafficPolicy Local"),
		Entry("trusted proxy that is not a CIDR",
			func(s *operatorv1alpha1.EnvoyProxySpec) {
				s.Listeners = &operatorv1alpha1.EnvoyListenersSpec{GRPCWeb: &operatorv1alpha1.EnvoyGRPCWebListenerSpec{
					TrustedProxyCIDRs: []operatorv1alpha1.EnvoyCIDR{"not-a-cidr"},
				}}
			}, "must be a CIDR"),
		Entry("grpcs without a TLS secret",
			func(s *operatorv1alpha1.EnvoyProxySpec) {
				s.Listeners = &operatorv1alpha1.EnvoyListenersSpec{GRPCS: &operatorv1alpha1.EnvoyListenerSpec{}}
			}, "grpcs requires tlsSecretName"),
	)
})
