// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/uc"
)

func TestBuildAlloyContainer_Defaults(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}
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
	if container.Image != AlloyDefaultImage {
		t.Errorf("expected image %q, got %q", AlloyDefaultImage, container.Image)
	}
	// Should have args to run the config
	if len(container.Args) != 2 || container.Args[0] != "run" {
		t.Errorf("expected args [run /etc/alloy/config.alloy], got %v", container.Args)
	}
	// Should always have the alloy-config volume mount
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount (alloy-config), got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != "alloy-config" {
		t.Errorf("expected volume mount name 'alloy-config', got %q", container.VolumeMounts[0].Name)
	}
}

func TestBuildAlloyContainer_CustomNameAndImage(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Name:    "my-alloy",
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
			Repository: "grafana",
			ImageName:  "alloy",
			ImageTag:   "v1.8.1",
		},
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
	reconciler := &StatefulSetSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
			ImagePullPolicy: operatorv1alpha1.ImagePullPolicyAlways,
		},
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	if container.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("expected ImagePullPolicy 'Always', got %q", container.ImagePullPolicy)
	}
}

func TestBuildAlloyContainer_Resources(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}
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

func TestBuildAlloyContainer_AdditionalVolumeMounts(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		AdditionalVolumeMounts: []corev1.VolumeMount{
			{Name: "custom-vol", MountPath: "/custom"},
		},
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	// 1 (alloy-config) + 1 (additional)
	if len(container.VolumeMounts) != 2 {
		t.Fatalf("expected 2 volume mounts, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != "alloy-config" {
		t.Errorf("expected first volume mount name 'alloy-config', got %q", container.VolumeMounts[0].Name)
	}
	if container.VolumeMounts[1].Name != "custom-vol" {
		t.Errorf("expected second volume mount name 'custom-vol', got %q", container.VolumeMounts[1].Name)
	}
}

func TestBuildAlloyContainer_LokiEnabled_WithLogsPVC_MountsLogs(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			PersistentVolumeClaims: &operatorv1alpha1.ConsensusPVCConfig{
				Logs: &corev1.PersistentVolumeClaimSpec{},
			},
		},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled:  true,
					Endpoint: "http://loki:3100/loki/api/v1/push",
				},
			},
		},
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	// alloy-config + logs
	if len(container.VolumeMounts) != 2 {
		t.Fatalf("expected 2 volume mounts, got %d", len(container.VolumeMounts))
	}
	found := false
	for _, vm := range container.VolumeMounts {
		if vm.Name == VolumeTypeLogs && vm.ReadOnly {
			found = true
		}
	}
	if !found {
		t.Error("expected logs volume mount with ReadOnly=true when Loki is enabled and logs PVC exists")
	}
}

func TestBuildAlloyContainer_LokiEnabled_NoLogsPVC_SkipsLogsMount(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Namespace: "test-ns"},
	}
	config := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled: true,
				},
			},
		},
	}

	container := reconciler.buildAlloyContainer(capsule, config)

	// Only alloy-config, no logs mount since PVC not configured
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount (alloy-config only), got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != "alloy-config" {
		t.Errorf("expected only alloy-config mount, got %q", container.VolumeMounts[0].Name)
	}
}

func TestAddSidecarContainers_Alloy(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}

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
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							ImageName: "test",
							ImageTag:  "latest",
						},
					},
					Alloy: &operatorv1alpha1.AlloyConfig{
						Enabled: true,
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "grafana",
							ImageName:  "alloy",
							ImageTag:   "v1.8.1",
						},
					},
				},
			},
		},
	}

	deploy := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	if err := reconciler.addSidecarContainers(deploy, capsule, capsule.Spec.PodProperties.Containers.Alloy, uc.ModeClusterOnly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	// Should have alloy-config volume added to the deployment
	foundVol := false
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == "alloy-config" {
			foundVol = true
			if v.ConfigMap.Name != "test-node-alloy-config" {
				t.Errorf("expected ConfigMap name 'test-node-alloy-config', got %q", v.ConfigMap.Name)
			}
		}
	}
	if !foundVol {
		t.Error("expected alloy-config volume to be added to deployment")
	}
}

func TestAddSidecarContainers_AlloyDisabled(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}

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
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							ImageName: "test",
							ImageTag:  "latest",
						},
					},
					Alloy: &operatorv1alpha1.AlloyConfig{
						Enabled: false,
					},
				},
			},
		},
	}

	deploy := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	if err := reconciler.addSidecarContainers(deploy, capsule, capsule.Spec.PodProperties.Containers.Alloy, uc.ModeClusterOnly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have only 1 container when alloy is disabled
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container when alloy disabled, got %d", len(deploy.Spec.Template.Spec.Containers))
	}
}

func TestAddSidecarContainers_AlloyNil(t *testing.T) {
	reconciler := &StatefulSetSubReconciler{}

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
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							ImageName: "test",
							ImageTag:  "latest",
						},
					},
				},
			},
		},
	}

	deploy := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "consensus-node"},
					},
				},
			},
		},
	}

	if err := reconciler.addSidecarContainers(deploy, capsule, nil, uc.ModeClusterOnly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have only 1 container when alloy is nil
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("expected 1 container when alloy nil, got %d", len(deploy.Spec.Template.Spec.Containers))
	}
}

// --- Alloy River config generation tests ---

func TestBuildAlloyRiverConfig_MinimalDefaults(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{Enabled: true}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, `level = "info"`) {
		t.Error("expected default log level 'info'")
	}
	// No exporters enabled — should not have loki or prometheus blocks
	if strings.Contains(cfg, "loki.write") {
		t.Error("unexpected loki.write block when Loki not enabled")
	}
	if strings.Contains(cfg, "prometheus.remote_write") {
		t.Error("unexpected prometheus.remote_write block when Prometheus not enabled")
	}
}

func TestBuildAlloyRiverConfig_LokiEnabled(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			AppName:     "my-app",
			Environment: "production",
			LogLevel:    "DEBUG",
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled:  true,
					Endpoint: "http://loki.monitoring:3100/loki/api/v1/push",
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, `level = "debug"`) {
		t.Error("expected log level 'debug'")
	}
	if !strings.Contains(cfg, "local.file_match") {
		t.Error("expected local.file_match block for log discovery")
	}
	if !strings.Contains(cfg, "loki.source.file") {
		t.Error("expected loki.source.file block")
	}
	if !strings.Contains(cfg, "loki.write") {
		t.Error("expected loki.write block")
	}
	if !strings.Contains(cfg, "http://loki.monitoring:3100/loki/api/v1/push") {
		t.Error("expected custom Loki endpoint")
	}
	if !strings.Contains(cfg, `"my-app"`) {
		t.Error("expected custom app name in labels")
	}
	if !strings.Contains(cfg, `"production"`) {
		t.Error("expected custom environment in labels")
	}
}

func TestBuildAlloyRiverConfig_LokiWithAuth(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled: true,
					Auth: &operatorv1alpha1.AlloyLokiAuth{
						Enabled: true,
						Secret:  &corev1.LocalObjectReference{Name: "loki-credentials"},
					},
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, "basic_auth") {
		t.Error("expected basic_auth block when auth is enabled")
	}
	if !strings.Contains(cfg, "LOKI_USERNAME") {
		t.Error("expected LOKI_USERNAME env reference")
	}
}

func TestBuildAlloyRiverConfig_PrometheusEnabled(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
					Enabled:  true,
					Endpoint: "http://mimir:9090/api/v1/write",
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, "prometheus.scrape") {
		t.Error("expected prometheus.scrape block")
	}
	if !strings.Contains(cfg, "prometheus.remote_write") {
		t.Error("expected prometheus.remote_write block")
	}
	if !strings.Contains(cfg, "http://mimir:9090/api/v1/write") {
		t.Error("expected custom Prometheus endpoint")
	}
}

func TestBuildAlloyRiverConfig_AllExportersEnabled(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled: true,
				},
				PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
					Enabled: true,
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, "loki.write") {
		t.Error("expected loki.write block")
	}
	if !strings.Contains(cfg, "prometheus.remote_write") {
		t.Error("expected prometheus.remote_write block")
	}
}

func TestBuildAlloyRiverConfig_CustomLabels(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Labels: map[string]string{
				"cluster": "us-east-1",
				"team":    "platform",
			},
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled: true,
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, `"cluster" = "us-east-1"`) {
		t.Error("expected custom label 'cluster' in config")
	}
	if !strings.Contains(cfg, `"team" = "platform"`) {
		t.Error("expected custom label 'team' in config")
	}
	// Built-in labels should still be present
	if !strings.Contains(cfg, `"app"`) {
		t.Error("expected built-in 'app' label")
	}
}

func TestBuildAlloyRiverConfig_CustomScrapeInterval(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
					Enabled:        true,
					ScrapeInterval: "30s",
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, `scrape_interval = "30s"`) {
		t.Error("expected custom scrape interval '30s'")
	}
}

func TestBuildAlloyRiverConfig_DefaultScrapeInterval(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test-ns"},
	}
	alloy := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			Exporters: &operatorv1alpha1.AlloyExporters{
				PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
					Enabled: true,
				},
			},
		},
	}

	cfg := BuildAlloyRiverConfig(capsule, alloy)

	if !strings.Contains(cfg, `scrape_interval = "15s"`) {
		t.Error("expected default scrape interval '15s'")
	}
}

func TestAlloyConfigMapName(t *testing.T) {
	capsule := &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0"},
	}
	if AlloyConfigMapName(capsule) != "node0-alloy-config" {
		t.Errorf("expected 'node0-alloy-config', got %q", AlloyConfigMapName(capsule))
	}
}
