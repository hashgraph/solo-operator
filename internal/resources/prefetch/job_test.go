// SPDX-License-Identifier: Apache-2.0

package prefetch

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = operatorv1alpha1.AddToScheme(s)
	return s
}

func TestBuildPrefetchJob(t *testing.T) {
	scheme := testScheme()
	prepare := &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prep-1",
			Namespace: "test-ns",
			UID:       "uid-1",
		},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			OperationID: "op-001",
			Orbit:       "test-orbit",
		},
	}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				NodeSelector: map[string]string{"kubernetes.io/hostname": "node-3"},
			},
		},
	}

	job, err := BuildPrefetchJob(prepare, capsule, "ghcr.io/cn:0.62.0", 7776000, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Namespace != "test-ns" {
		t.Errorf("namespace = %q, want test-ns", job.Namespace)
	}
	if job.Labels[operatorv1alpha1.LabelOperationID] != "op-001" {
		t.Errorf("operation-id label = %q, want op-001", job.Labels[operatorv1alpha1.LabelOperationID])
	}
	if job.Labels[operatorv1alpha1.LabelPrefetchRole] != operatorv1alpha1.RolePrefetch {
		t.Errorf("role label = %q, want %s", job.Labels[operatorv1alpha1.LabelPrefetchRole], operatorv1alpha1.RolePrefetch)
	}
	if job.Labels[operatorv1alpha1.LabelCapsule] != "node0" {
		t.Errorf("capsule label = %q, want node0", job.Labels[operatorv1alpha1.LabelCapsule])
	}

	spec := job.Spec.Template.Spec
	if len(spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(spec.Containers))
	}
	if spec.Containers[0].Image != "ghcr.io/cn:0.62.0" {
		t.Errorf("image = %q, want ghcr.io/cn:0.62.0", spec.Containers[0].Image)
	}
	if spec.NodeSelector["kubernetes.io/hostname"] != "node-3" {
		t.Errorf("nodeSelector not copied from capsule")
	}
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 7776000 {
		t.Errorf("TTLSecondsAfterFinished = %v, want 7776000", job.Spec.TTLSecondsAfterFinished)
	}
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 600 {
		t.Errorf("ActiveDeadlineSeconds = %v, want 600", job.Spec.ActiveDeadlineSeconds)
	}
	if len(job.OwnerReferences) == 0 {
		t.Error("expected owner reference to be set")
	}
}

func TestBuildPrefetchJob_NameTruncation(t *testing.T) {
	scheme := testScheme()
	prepare := &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prep-1",
			Namespace: "test-ns",
			UID:       "uid-1",
		},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			OperationID: "very-long-operation-id-that-exceeds",
			Orbit:       "test-orbit",
		},
	}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "extremely-long-capsule-name-here", Namespace: "test-ns"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{},
		},
	}

	job, err := BuildPrefetchJob(prepare, capsule, "ghcr.io/cn:0.62.0", 300, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(job.Name) > 63 {
		t.Errorf("job name length %d exceeds 63: %s", len(job.Name), job.Name)
	}
}

func TestBuildPrefetchJob_ServiceAccountName(t *testing.T) {
	scheme := testScheme()
	prepare := &operatorv1alpha1.NetworkUpgradePrepare{
		ObjectMeta: metav1.ObjectMeta{Name: "prep-1", Namespace: "ns", UID: "uid-1"},
		Spec: operatorv1alpha1.NetworkUpgradePrepareSpec{
			OperationID: "op-001",
			Orbit:       "test-orbit",
		},
	}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "ns"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				ServiceAccount: &operatorv1alpha1.ServiceAccountConfig{Name: "my-sa"},
			},
		},
	}

	job, err := BuildPrefetchJob(prepare, capsule, "img:v1", 300, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Spec.Template.Spec.ServiceAccountName != "my-sa" {
		t.Errorf("ServiceAccountName = %q, want my-sa", job.Spec.Template.Spec.ServiceAccountName)
	}
}
