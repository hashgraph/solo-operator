// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const validLog4j2XML = `<?xml version="1.0" encoding="UTF-8"?>
<Configuration status="WARN" monitorInterval="600">
  <Appenders>
    <Console name="Console" target="SYSTEM_OUT">
      <PatternLayout pattern="%d{yyyy-MM-dd HH:mm:ss.SSS} %-5p %-4L %c{1} - %m{nolookups}%n"/>
    </Console>
  </Appenders>
  <Loggers>
    <Root level="WARN">
      <AppenderRef ref="Console"/>
    </Root>
  </Loggers>
</Configuration>`

var _ = Describe("Log4j2Config Controller", func() {

	Context("When creating a Log4j2Config with valid XML", func() {
		const (
			crName    = "test-log4j2"
			namespace = "log4j2-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should create the ConfigMap with the correct content", func() {
			By("Creating a Log4j2Config")
			cr := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: validLog4j2XML},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying the ConfigMap is created")
			cmKey := types.NamespacedName{Name: crName, Namespace: namespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, cmKey, createdCM) == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdCM.Data["log4j2.xml"]).To(Equal(validLog4j2XML))

			By("Verifying the status is updated")
			crKey := types.NamespacedName{Name: crName, Namespace: namespace}
			updatedCR := &operatorv1alpha1.Log4j2Config{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, crKey, updatedCR); err != nil {
					return ""
				}
				return updatedCR.Status.ConfigMapName
			}, timeout, interval).Should(Equal(crName))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})
	})

	Context("When updating spec.content", func() {
		const (
			crName    = "test-log4j2-update"
			namespace = "log4j2-update-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should update the ConfigMap when content changes", func() {
			By("Creating a Log4j2Config")
			cr := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: validLog4j2XML},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			cmKey := types.NamespacedName{Name: crName, Namespace: namespace}
			createdCM := &corev1.ConfigMap{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, cmKey, createdCM) == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating spec.content")
			updatedXML := `<?xml version="1.0" encoding="UTF-8"?>
<Configuration status="INFO" monitorInterval="30">
  <Appenders>
    <Console name="Console" target="SYSTEM_OUT">
      <PatternLayout pattern="%m%n"/>
    </Console>
  </Appenders>
  <Loggers>
    <Root level="INFO">
      <AppenderRef ref="Console"/>
    </Root>
  </Loggers>
</Configuration>`

			crKey := types.NamespacedName{Name: crName, Namespace: namespace}
			Eventually(func() error {
				fetchedCR := &operatorv1alpha1.Log4j2Config{}
				if err := k8sClient.Get(ctx, crKey, fetchedCR); err != nil {
					return err
				}
				fetchedCR.Spec.Content = updatedXML
				return k8sClient.Update(ctx, fetchedCR)
			}, timeout, interval).Should(Succeed())

			By("Verifying the ConfigMap is updated")
			Eventually(func() string {
				if err := k8sClient.Get(ctx, cmKey, createdCM); err != nil {
					return ""
				}
				return createdCM.Data["log4j2.xml"]
			}, timeout, interval).Should(Equal(updatedXML))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
			})).Should(Succeed())
		})
	})

	Context("When creating a Log4j2Config with invalid XML", func() {
		const (
			crName    = "test-log4j2-invalid"
			namespace = "log4j2-invalid-test"
		)
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})

		It("Should not create a ConfigMap for malformed XML", func() {
			By("Creating a Log4j2Config with malformed XML")
			cr := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: namespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: `<Configuration status="WARN"><Unclosed`},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying no ConfigMap is created")
			cmKey := types.NamespacedName{Name: crName, Namespace: namespace}
			Consistently(func() bool {
				cm := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, cmKey, cm) == nil
			}, "3s", interval).Should(BeFalse())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})

		It("Should not create a ConfigMap for wrong root element", func() {
			By("Creating a Log4j2Config with wrong root element")
			cr := &operatorv1alpha1.Log4j2Config{
				ObjectMeta: metav1.ObjectMeta{Name: crName + "-wrongroot", Namespace: namespace},
				Spec:       operatorv1alpha1.Log4j2ConfigSpec{Content: `<?xml version="1.0"?><NotLog4j/>`},
			}
			Expect(k8sClient.Create(ctx, cr)).Should(Succeed())

			By("Verifying no ConfigMap is created")
			cmKey := types.NamespacedName{Name: crName + "-wrongroot", Namespace: namespace}
			Consistently(func() bool {
				cm := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, cmKey, cm) == nil
			}, "3s", interval).Should(BeFalse())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cr)).Should(Succeed())
		})
	})
})
