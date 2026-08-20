// SPDX-License-Identifier: Apache-2.0

package prefetch

import (
	"crypto/sha256"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// PrefetchTarget describes where a prefetch Job should run.
type PrefetchTarget struct {
	// NodeName pins the Job to a specific node (node-based mode).
	// When empty, capsule scheduling constraints are used instead.
	NodeName string

	// Capsule-based scheduling (used when NodeName is empty).
	NodeSelector map[string]string
	Tolerations  []corev1.Toleration
	Affinity     *corev1.Affinity

	ServiceAccountName string
}

// TargetFromNode creates a target pinned to a specific node. Tolerations are
// still applied even though the Job is pinned via NodeName: pinning bypasses the
// scheduler (so NoSchedule taints are ignored), but the taint manager still
// evicts pods that lack a matching toleration for NoExecute taints. Passing the
// union of the consensus capsules' tolerations keeps prefetch pods alive on
// dedicated, tainted node pools.
func TargetFromNode(nodeName, serviceAccountName string, tolerations []corev1.Toleration) PrefetchTarget {
	return PrefetchTarget{NodeName: nodeName, ServiceAccountName: serviceAccountName, Tolerations: tolerations}
}

// TargetFromCapsule creates a target using capsule scheduling constraints.
func TargetFromCapsule(capsule *operatorv1alpha1.ConsensusCapsule) PrefetchTarget {
	return PrefetchTarget{
		NodeSelector:       capsule.Spec.PodProperties.NodeSelector,
		Tolerations:        capsule.Spec.PodProperties.Tolerations,
		Affinity:           capsule.Spec.PodProperties.Affinity,
		ServiceAccountName: serviceAccountName(capsule),
	}
}

func BuildPrefetchJob(
	prepare *operatorv1alpha1.NetworkUpgradePrepare,
	target PrefetchTarget,
	targetID string,
	image string,
	ttlSeconds int32,
	scheme *runtime.Scheme,
) (*batchv1.Job, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(prepare.Spec.OperationID+"\x00"+targetID+"\x00"+image)))[:8]
	opID := dnsNormalize(prepare.Spec.OperationID, 20)
	tgtName := dnsNormalize(targetID, 20)
	// Each segment is bounded by dnsNormalize (<=20) plus an 8-char hash, so the
	// name is at most len("prefetch-")+20+1+20+1+8 = 59 chars — always within the
	// 63-char limit the Job controller needs (it stamps the job name as the
	// job-name label value on pods, and label values must be <=63). The
	// NameBounded test guards this invariant.
	name := fmt.Sprintf("prefetch-%s-%s-%s", opID, tgtName, hash)

	backoffLimit := int32(3)
	activeDeadlineSeconds := int64(600)
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:    "prefetch",
				Image:   image,
				Command: []string{"/bin/sh", "-c", "exit 0"},
				// Always pull: a mutable :tag already cached on the node would
				// satisfy IfNotPresent and skip fetching the current layers,
				// defeating the prefetch (and letting a stale image look
				// "prefetched"). A pull failure then fails the Job so the upgrade
				// falls back to the normal registry path.
				ImagePullPolicy: corev1.PullAlways,
				// The container does nothing (the pull happens at the kubelet
				// level); keep requests tiny so the Job schedules anywhere, with
				// modest limits as a safety cap.
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("16Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
		},
		ServiceAccountName: target.ServiceAccountName,
	}

	// Tolerations apply in both modes: node-pinned pods still need them to
	// survive NoExecute taints on dedicated node pools (NodeName pinning only
	// bypasses the scheduler's NoSchedule check, not taint-manager eviction).
	podSpec.Tolerations = target.Tolerations
	if target.NodeName != "" {
		podSpec.NodeName = target.NodeName
	} else {
		podSpec.NodeSelector = target.NodeSelector
		podSpec.Affinity = target.Affinity
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: prepare.Namespace,
			Labels: map[string]string{
				operatorv1alpha1.LabelOperationID:  prepare.Spec.OperationID,
				operatorv1alpha1.LabelStep:         operatorv1alpha1.StepPrepare,
				operatorv1alpha1.LabelPrefetchRole: operatorv1alpha1.RolePrefetch,
				// targetID (a node or capsule name) is a DNS subdomain that may
				// exceed 63 chars or contain characters invalid in a label
				// value; normalize it so Job creation never fails validation.
				// This label is informational only (never used as a selector).
				operatorv1alpha1.LabelCapsule: dnsNormalize(targetID, 63),
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSeconds,
			BackoffLimit:            &backoffLimit,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}

	if err := controllerutil.SetControllerReference(prepare, job, scheme); err != nil {
		return nil, err
	}
	return job, nil
}

// dnsNormalize lowercases s, replaces non-alphanumeric chars with hyphens, and
// truncates to maxLen. The result is trimmed of leading/trailing hyphens so it
// is a valid DNS-1123 label fragment and a valid Kubernetes label value (both
// must start and end with an alphanumeric character).
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
	return strings.Trim(out, "-")
}

func serviceAccountName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	if capsule.Spec.PodProperties.ServiceAccount != nil && capsule.Spec.PodProperties.ServiceAccount.Name != "" {
		return capsule.Spec.PodProperties.ServiceAccount.Name
	}
	return ""
}
