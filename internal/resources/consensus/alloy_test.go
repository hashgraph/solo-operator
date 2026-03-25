// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestBuildAlloyContainer_Defaults(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if container.Name != "alloy" {
		t.Errorf("expected container name 'alloy', got %q", container.Name)
	}
	if container.Image != "grafana/alloy:latest" {
		t.Errorf("expected image 'grafana/alloy:latest', got %q", container.Image)
	}
}

func TestBuildAlloyContainer_CustomNameAndImage(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Name:    "my-alloy",
		Image:   "grafana/alloy:v1.8.1",
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if container.Name != "my-alloy" {
		t.Errorf("expected container name 'my-alloy', got %q", container.Name)
	}
	if container.Image != "grafana/alloy:v1.8.1" {
		t.Errorf("expected image 'grafana/alloy:v1.8.1', got %q", container.Image)
	}
}

func TestBuildAlloyContainer_ImagePullPolicy(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled:         true,
		ImagePullPolicy: corev1.PullAlways,
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if container.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("expected ImagePullPolicy 'Always', got %q", container.ImagePullPolicy)
	}
}

func TestBuildAlloyContainer_Resources(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if container.Resources.Requests.Cpu().String() != "100m" {
		t.Errorf("expected CPU request '100m', got %q", container.Resources.Requests.Cpu().String())
	}
	if container.Resources.Limits.Memory().String() != "512Mi" {
		t.Errorf("expected memory limit '512Mi', got %q", container.Resources.Limits.Memory().String())
	}
}

func TestBuildAlloyContainer_VolumeMounts(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		AdditionalVolumeMounts: []corev1.VolumeMount{
			{Name: "alloy-config", MountPath: "/etc/alloy"},
			{Name: "logs", MountPath: "/var/log"},
		},
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if len(container.VolumeMounts) != 2 {
		t.Fatalf("expected 2 volume mounts, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != "alloy-config" {
		t.Errorf("expected first volume mount name 'alloy-config', got %q", container.VolumeMounts[0].Name)
	}
	if container.VolumeMounts[1].MountPath != "/var/log" {
		t.Errorf("expected second volume mount path '/var/log', got %q", container.VolumeMounts[1].MountPath)
	}
}

func TestBuildAlloyContainer_NoVolumeMountsWhenEmpty(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if len(container.VolumeMounts) != 0 {
		t.Errorf("expected 0 volume mounts, got %d", len(container.VolumeMounts))
	}
}

func TestAddSidecarContainers_Alloy(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			AccountId: "0.0.3",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						Image: "test:latest",
					},
					Alloy: &operatorv1alpha1.AlloyConfig{
						Enabled: true,
						Image:   "grafana/alloy:v1.8.1",
					},
				},
			},
		},
	}

	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	reconciler.addSidecarContainers(deploy, capsule)

	// Should have 2 containers: consensus-node + alloy
	if len(deploy.Spec.Template.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(deploy.Spec.Template.Spec.Containers))
	}

	alloyContainer := deploy.Spec.Template.Spec.Containers[1]
	if alloyContainer.Name != "alloy" {
		t.Errorf("expected container name 'alloy', got %q", alloyContainer.Name)
	}
	if alloyContainer.Image != "grafana/alloy:v1.8.1" {
		t.Errorf("expected image 'grafana/alloy:v1.8.1', got %q", alloyContainer.Image)
	}
}

func TestAddSidecarContainers_AlloyDisabled(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			AccountId: "0.0.3",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						Image: "test:latest",
					},
					Alloy: &operatorv1alpha1.AlloyConfig{
						Enabled: false,
					},
				},
			},
		},
	}

	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	reconciler.addSidecarContainers(deploy, capsule)

	// Should have only 1 container when alloy is disabled
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container when alloy disabled, got %d", len(deploy.Spec.Template.Spec.Containers))
	}
}

func TestAddSidecarContainers_AlloyNil(t *testing.T) {
	reconciler := &DeploymentSubReconciler{}

	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-ns",
		},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			AccountId: "0.0.3",
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						Image: "test:latest",
					},
				},
			},
		},
	}

	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	reconciler.addSidecarContainers(deploy, capsule)

	// Should have only 1 container when alloy is nil
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container when alloy nil, got %d", len(deploy.Spec.Template.Spec.Containers))
	}
}
