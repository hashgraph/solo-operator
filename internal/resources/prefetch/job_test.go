// SPDX-License-Identifier: Apache-2.0

package prefetch

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = operatorv1alpha1.AddToScheme(s)
	return s
}

func testPrepare() *operatorv1alpha1.NetworkUpgradePrepare {
	return &operatorv1alpha1.NetworkUpgradePrepare{
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
}

func TestBuildPrefetchJob_NodePinned(t *testing.T) {
	scheme := testScheme()
	tolerations := []corev1.Toleration{{
		Key:      "dedicated",
		Operator: corev1.TolerationOpEqual,
		Value:    "consensus",
		Effect:   corev1.TaintEffectNoExecute,
	}}
	target := TargetFromNode("node-3", "my-sa", tolerations)

	job, err := BuildPrefetchJob(testPrepare(), target, "node-3", []string{"ghcr.io/cn:0.62.0"}, 7776000, scheme)
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

	spec := job.Spec.Template.Spec
	if spec.NodeName != "node-3" {
		t.Errorf("nodeName = %q, want node-3", spec.NodeName)
	}
	if len(spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(spec.Containers))
	}
	if spec.Containers[0].Image != "ghcr.io/cn:0.62.0" {
		t.Errorf("image = %q, want ghcr.io/cn:0.62.0", spec.Containers[0].Image)
	}
	if spec.Containers[0].ImagePullPolicy != corev1.PullAlways {
		t.Errorf("ImagePullPolicy = %q, want Always (must refresh mutable tags)", spec.Containers[0].ImagePullPolicy)
	}
	if spec.ServiceAccountName != "my-sa" {
		t.Errorf("ServiceAccountName = %q, want my-sa", spec.ServiceAccountName)
	}
	if len(spec.Tolerations) != 1 || spec.Tolerations[0].Key != "dedicated" ||
		spec.Tolerations[0].Effect != corev1.TaintEffectNoExecute {
		t.Errorf("node-pinned Job must carry tolerations, got %+v", spec.Tolerations)
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

func TestBuildPrefetchJob_CapsuleBased(t *testing.T) {
	scheme := testScheme()
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				NodeSelector: map[string]string{"kubernetes.io/hostname": "node-3"},
			},
		},
	}
	target := TargetFromCapsule(capsule)

	job, err := BuildPrefetchJob(testPrepare(), target, capsule.Name, []string{"ghcr.io/cn:0.62.0"}, 7776000, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spec := job.Spec.Template.Spec
	if spec.NodeName != "" {
		t.Errorf("nodeName should be empty for capsule-based, got %q", spec.NodeName)
	}
	if spec.NodeSelector["kubernetes.io/hostname"] != "node-3" {
		t.Errorf("nodeSelector not copied from capsule")
	}
}

func TestBuildPrefetchJob_NameBounded(t *testing.T) {
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
	target := TargetFromNode("extremely-long-node-name-here-that-is-very-long", "", nil)

	job, err := BuildPrefetchJob(prepare, target, "extremely-long-node-name-here-that-is-very-long", []string{"ghcr.io/cn:0.62.0"}, 300, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Even with over-long inputs the name stays within 63 chars (so the
	// job-name label the Job controller adds to pods is a valid label value)
	// and is a valid DNS-1123 subdomain.
	if len(job.Name) > 63 {
		t.Errorf("job name length %d exceeds 63: %s", len(job.Name), job.Name)
	}
	if errs := validation.IsDNS1123Subdomain(job.Name); len(errs) > 0 {
		t.Errorf("job name %q is not a valid DNS-1123 subdomain: %v", job.Name, errs)
	}
	if errs := validation.IsValidLabelValue(job.Name); len(errs) > 0 {
		t.Errorf("job name %q is not a valid label value: %v", job.Name, errs)
	}
}

func TestBuildPrefetchJob_LabelValueNormalized(t *testing.T) {
	scheme := testScheme()
	// A target name longer than 63 chars, mixed case, with characters that are
	// invalid in a Kubernetes label value (underscore-prefixed, trailing dot).
	longName := "Node_" + strings.Repeat("x", 70) + "."
	target := TargetFromNode(longName, "", nil)

	job, err := BuildPrefetchJob(testPrepare(), target, longName, []string{"img:v1"}, 300, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := job.Labels[operatorv1alpha1.LabelCapsule]
	if errs := validation.IsValidLabelValue(v); len(errs) > 0 {
		t.Errorf("LabelCapsule value %q is not a valid label value: %v", v, errs)
	}
	if len(job.Name) > 63 {
		t.Errorf("job name length %d exceeds 63: %s", len(job.Name), job.Name)
	}
	if errs := validation.IsDNS1123Subdomain(job.Name); len(errs) > 0 {
		t.Errorf("job name %q is not a valid DNS-1123 subdomain: %v", job.Name, errs)
	}
}

func TestBuildPrefetchJob_ServiceAccountFromCapsule(t *testing.T) {
	scheme := testScheme()
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "ns"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				ServiceAccount: &operatorv1alpha1.ServiceAccountConfig{Name: "my-sa"},
			},
		},
	}
	target := TargetFromCapsule(capsule)

	job, err := BuildPrefetchJob(testPrepare(), target, capsule.Name, []string{"img:v1"}, 300, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Spec.Template.Spec.ServiceAccountName != "my-sa" {
		t.Errorf("ServiceAccountName = %q, want my-sa", job.Spec.Template.Spec.ServiceAccountName)
	}
}

// A single Job pulls all images: images[0] as the main container and the rest as
// init containers (sequential pulls). The deadline scales per image.
func TestBuildPrefetchJob_MultiImage(t *testing.T) {
	scheme := testScheme()
	images := []string{"reg/cn:1", "reg/uploader:1", "reg/uc:1"}

	job, err := BuildPrefetchJob(testPrepare(), TargetFromNode("node-3", "my-sa", nil), "node-3", images, 300, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spec := job.Spec.Template.Spec
	if len(spec.Containers) != 1 || spec.Containers[0].Image != images[0] {
		t.Fatalf("main container should be images[0]=%q, got %+v", images[0], spec.Containers)
	}
	if len(spec.InitContainers) != 2 {
		t.Fatalf("expected 2 init containers for 3 images, got %d", len(spec.InitContainers))
	}

	pulled := map[string]bool{spec.Containers[0].Image: true}
	names := map[string]bool{spec.Containers[0].Name: true}
	for _, c := range spec.InitContainers {
		if c.ImagePullPolicy != corev1.PullAlways {
			t.Errorf("init container %s must use PullAlways", c.Name)
		}
		if names[c.Name] {
			t.Errorf("duplicate container name %q", c.Name)
		}
		names[c.Name] = true
		pulled[c.Image] = true
	}
	for _, img := range images {
		if !pulled[img] {
			t.Errorf("image %q is not pulled by any container", img)
		}
	}

	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 1800 {
		t.Errorf("ActiveDeadlineSeconds = %v, want 1800 (600*3)", job.Spec.ActiveDeadlineSeconds)
	}
}

func TestBuildPrefetchJob_NoImages(t *testing.T) {
	scheme := testScheme()
	if _, err := BuildPrefetchJob(testPrepare(), TargetFromNode("n", "", nil), "n", nil, 300, scheme); err == nil {
		t.Error("expected an error when no images are provided")
	}
}
