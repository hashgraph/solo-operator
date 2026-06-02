// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

var _ = Describe("NetworkUpgradePrepare Controller", func() {
	ctx := context.Background()

	const (
		ns      = "nup-registry"
		nupName = "prepare-1"
		opID    = "nup-op-001"
	)

	BeforeEach(func() {
		createNamespaceIgnoreExisting(ctx, ns)
	})

	It("audits every registry of each enabled component and emits RegistryAuditComplete", func() {
		nup := &operatorv1alpha1.NetworkUpgradePrepare{
			ObjectMeta: metav1.ObjectMeta{Name: nupName, Namespace: ns},
			Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
				Orbit:       "nup-orbit",
				OperationID: opID,
				Images: operatorv1alpha1.UpgradeImagesSpec{
					ConsensusNode: operatorv1alpha1.UpgradeComponentSpec{
						Version: "0.62.0",
						Registries: []operatorv1alpha1.UpgradeRegistrySpec{
							{Image: "gcr.io/a/cn:0.62.0"},
							{Image: "ghcr.io/b/cn:0.62.0"},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, nup)).To(Succeed())

		// The suite configures the Sequential strategy; all registries are still
		// audited end-to-end.
		Eventually(func() bool {
			return hasEvent(ctx, ns, nupName, operatorv1alpha1.EventRegistryAuditComplete,
				"consensusNode: 2/2 registries reachable")
		}, timeout, interval).Should(BeTrue())
	})
})

func hasEvent(ctx context.Context, ns, objName, reason, msgSubstr string) bool {
	var events corev1.EventList
	if err := k8sClient.List(ctx, &events, client.InNamespace(ns)); err != nil {
		return false
	}
	for _, e := range events.Items {
		if e.InvolvedObject.Name == objName && e.Reason == reason && strings.Contains(e.Message, msgSubstr) {
			return true
		}
	}
	return false
}
