// SPDX-License-Identifier: Apache-2.0

package prefetch

import (
	"crypto/sha256"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func BuildPrefetchJob(
	prepare *operatorv1alpha1.NetworkUpgradePrepare,
	capsule *operatorv1alpha1.ConsensusCapsule,
	image string,
	ttlSeconds int32,
	scheme *runtime.Scheme,
) (*batchv1.Job, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(prepare.Spec.OperationID+"\x00"+capsule.Name+"\x00"+image)))[:8]
	opID := dnsNormalize(prepare.Spec.OperationID, 20)
	capName := dnsNormalize(capsule.Name, 20)
	name := fmt.Sprintf("prefetch-%s-%s-%s", opID, capName, hash)
	if len(name) > 63 {
		name = name[:63]
	}

	backoffLimit := int32(3)
	activeDeadlineSeconds := int64(600)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: prepare.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID:  prepare.Spec.OperationID,
				operatorv1alpha1.LabelStep:         operatorv1alpha1.StepPrepare,
				operatorv1alpha1.LabelPrefetchRole: operatorv1alpha1.RolePrefetch,
				operatorv1alpha1.LabelCapsule:      capsule.Name,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSeconds,
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "prefetch",
							Image:           image,
							Command:         []string{"/bin/sh", "-c", "exit 0"},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					// Copy scheduling constraints so the prefetch pod lands on the
					// same node as the consensus pod, caching the image where it
					// will actually be needed during Execute.
					NodeSelector:       capsule.Spec.PodProperties.NodeSelector,
					Tolerations:        capsule.Spec.PodProperties.Tolerations,
					Affinity:           capsule.Spec.PodProperties.Affinity,
					ServiceAccountName: serviceAccountName(capsule),
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(prepare, job, scheme); err != nil {
		return nil, err
	}
	return job, nil
}

// dnsNormalize lowercases s, replaces non-alphanumeric chars with hyphens,
// and truncates to maxLen — producing a valid DNS-1123 label fragment.
func dnsNormalize(s string, maxLen int) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return strings.TrimRight(out, "-")
}

func serviceAccountName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	if capsule.Spec.PodProperties.ServiceAccount != nil && capsule.Spec.PodProperties.ServiceAccount.Name != "" {
		return capsule.Spec.PodProperties.ServiceAccount.Name
	}
	return ""
}
