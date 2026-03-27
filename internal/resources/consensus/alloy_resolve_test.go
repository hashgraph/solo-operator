// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// --- mergeAlloyConfig tests ---

func TestMergeAlloyConfig_CapsuleOverridesImage(t *testing.T) {
	orbit := &operatorv1alpha1.OrbitAlloyDefaults{
		Enabled: true,
		Image:   "grafana/alloy:v1.10.0",
	}
	capsule := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Image:   "grafana/alloy:v1.14.2",
	}

	merged := mergeAlloyConfig(orbit, capsule)

	if merged.Image != "grafana/alloy:v1.14.2" {
		t.Errorf("expected capsule image, got %q", merged.Image)
	}
}

func TestMergeAlloyConfig_InheritsImageFromOrbit(t *testing.T) {
	orbit := &operatorv1alpha1.OrbitAlloyDefaults{
		Enabled: true,
		Image:   "grafana/alloy:v1.10.0",
	}
	capsule := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		// No image set
	}

	merged := mergeAlloyConfig(orbit, capsule)

	if merged.Image != "grafana/alloy:v1.10.0" {
		t.Errorf("expected orbit image, got %q", merged.Image)
	}
}

func TestMergeAlloyConfig_InheritsResourcesFromOrbit(t *testing.T) {
	orbit := &operatorv1alpha1.OrbitAlloyDefaults{
		Enabled: true,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
	}
	capsule := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
	}

	merged := mergeAlloyConfig(orbit, capsule)

	if merged.Resources == nil {
		t.Fatal("expected inherited resources from orbit")
	}
	if merged.Resources.Requests.Cpu().String() != "100m" {
		t.Errorf("expected CPU request '100m', got %q", merged.Resources.Requests.Cpu().String())
	}
}

func TestMergeAlloyConfig_CapsuleResourcesOverrideOrbit(t *testing.T) {
	orbit := &operatorv1alpha1.OrbitAlloyDefaults{
		Enabled: true,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
	}
	capsule := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("200m"),
			},
		},
	}

	merged := mergeAlloyConfig(orbit, capsule)

	if merged.Resources.Requests.Cpu().String() != "200m" {
		t.Errorf("expected capsule CPU request '200m', got %q", merged.Resources.Requests.Cpu().String())
	}
}

// --- mergeAlloyCollectorConfig tests ---

func TestMergeCollectorConfig_NilOrbit(t *testing.T) {
	capsuleCfg := &operatorv1alpha1.AlloyCollectorConfig{
		AppName: "my-app",
	}
	merged := mergeAlloyCollectorConfig(nil, capsuleCfg)

	if merged.AppName != "my-app" {
		t.Errorf("expected capsule appName, got %q", merged.AppName)
	}
}

func TestMergeCollectorConfig_NilCapsule(t *testing.T) {
	orbitCfg := &operatorv1alpha1.AlloyCollectorConfig{
		AppName:     "orbit-app",
		Environment: "staging",
	}
	merged := mergeAlloyCollectorConfig(orbitCfg, nil)

	if merged.AppName != "orbit-app" {
		t.Errorf("expected orbit appName, got %q", merged.AppName)
	}
	if merged.Environment != "staging" {
		t.Errorf("expected orbit environment, got %q", merged.Environment)
	}
}

func TestMergeCollectorConfig_ScalarFieldsMerge(t *testing.T) {
	orbitCfg := &operatorv1alpha1.AlloyCollectorConfig{
		AppName:     "orbit-app",
		Environment: "production",
		LogLevel:    "WARN",
	}
	capsuleCfg := &operatorv1alpha1.AlloyCollectorConfig{
		// AppName not set — should inherit from orbit
		Environment: "staging", // overrides orbit
		// LogLevel not set — should inherit from orbit
	}

	merged := mergeAlloyCollectorConfig(orbitCfg, capsuleCfg)

	if merged.AppName != "orbit-app" {
		t.Errorf("expected inherited appName 'orbit-app', got %q", merged.AppName)
	}
	if merged.Environment != "staging" {
		t.Errorf("expected capsule environment 'staging', got %q", merged.Environment)
	}
	if merged.LogLevel != "WARN" {
		t.Errorf("expected inherited logLevel 'WARN', got %q", merged.LogLevel)
	}
}

// --- mergeAlloyExporters tests ---

func TestMergeExporters_InheritsLokiFromOrbit(t *testing.T) {
	orbitExp := &operatorv1alpha1.AlloyExporters{
		Loki: &operatorv1alpha1.AlloyLokiExporter{
			Enabled:  true,
			Endpoint: "http://loki.observability:3100/loki/api/v1/push",
		},
	}
	capsuleExp := &operatorv1alpha1.AlloyExporters{
		// No Loki — should inherit from orbit
		PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
			Enabled:  true,
			Endpoint: "http://prom:9090/api/v1/write",
		},
	}

	merged := mergeAlloyExporters(orbitExp, capsuleExp)

	if merged.Loki == nil || !merged.Loki.Enabled {
		t.Error("expected Loki inherited from orbit")
	}
	if merged.Loki.Endpoint != "http://loki.observability:3100/loki/api/v1/push" {
		t.Errorf("expected orbit Loki endpoint, got %q", merged.Loki.Endpoint)
	}
	if merged.PrometheusRemote == nil || !merged.PrometheusRemote.Enabled {
		t.Error("expected capsule Prometheus preserved")
	}
}

func TestMergeExporters_CapsuleLokiOverridesOrbit(t *testing.T) {
	orbitExp := &operatorv1alpha1.AlloyExporters{
		Loki: &operatorv1alpha1.AlloyLokiExporter{
			Enabled:  true,
			Endpoint: "http://orbit-loki:3100/loki/api/v1/push",
		},
	}
	capsuleExp := &operatorv1alpha1.AlloyExporters{
		Loki: &operatorv1alpha1.AlloyLokiExporter{
			Enabled:  true,
			Endpoint: "http://capsule-loki:3100/loki/api/v1/push",
		},
	}

	merged := mergeAlloyExporters(orbitExp, capsuleExp)

	if merged.Loki.Endpoint != "http://capsule-loki:3100/loki/api/v1/push" {
		t.Errorf("expected capsule Loki endpoint, got %q", merged.Loki.Endpoint)
	}
}

func TestMergeExporters_InheritsPrometheusFromOrbit(t *testing.T) {
	orbitExp := &operatorv1alpha1.AlloyExporters{
		PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
			Enabled:  true,
			Endpoint: "http://prom:9090/api/v1/write",
		},
	}
	capsuleExp := &operatorv1alpha1.AlloyExporters{
		Loki: &operatorv1alpha1.AlloyLokiExporter{
			Enabled: true,
		},
		// No Prometheus — should inherit from orbit
	}

	merged := mergeAlloyExporters(orbitExp, capsuleExp)

	if merged.PrometheusRemote == nil || !merged.PrometheusRemote.Enabled {
		t.Error("expected Prometheus inherited from orbit")
	}
}

func TestMergeExporters_NilOrbit(t *testing.T) {
	capsuleExp := &operatorv1alpha1.AlloyExporters{
		Loki: &operatorv1alpha1.AlloyLokiExporter{
			Enabled: true,
		},
	}

	merged := mergeAlloyExporters(nil, capsuleExp)

	if merged.Loki == nil || !merged.Loki.Enabled {
		t.Error("expected capsule Loki preserved")
	}
}

func TestMergeExporters_NilCapsule(t *testing.T) {
	orbitExp := &operatorv1alpha1.AlloyExporters{
		Loki: &operatorv1alpha1.AlloyLokiExporter{
			Enabled:  true,
			Endpoint: "http://loki:3100/loki/api/v1/push",
		},
	}

	merged := mergeAlloyExporters(orbitExp, nil)

	if merged.Loki == nil || !merged.Loki.Enabled {
		t.Error("expected orbit Loki inherited")
	}
}

// --- alloyConfigFromOrbitDefaults tests ---

func TestAlloyConfigFromOrbitDefaults(t *testing.T) {
	defaults := &operatorv1alpha1.OrbitAlloyDefaults{
		Enabled:         true,
		Image:           "grafana/alloy:v1.14.2",
		ImagePullPolicy: corev1.PullAlways,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			AppName:     "hedera-node",
			Environment: "production",
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled:  true,
					Endpoint: "http://loki:3100/loki/api/v1/push",
				},
			},
		},
	}

	result := alloyConfigFromOrbitDefaults(defaults)

	if !result.Enabled {
		t.Error("expected enabled")
	}
	if result.Image != "grafana/alloy:v1.14.2" {
		t.Errorf("expected orbit image, got %q", result.Image)
	}
	if result.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("expected PullAlways, got %v", result.ImagePullPolicy)
	}
	if result.Config == nil || result.Config.Exporters == nil || result.Config.Exporters.Loki == nil {
		t.Fatal("expected Loki config from orbit")
	}
	if result.Config.Exporters.Loki.Endpoint != "http://loki:3100/loki/api/v1/push" {
		t.Errorf("expected orbit Loki endpoint, got %q", result.Config.Exporters.Loki.Endpoint)
	}
}

// --- Full merge scenario tests ---

func TestMergeAlloyConfig_FullScenario_OrbitLokiCapsulePrometheus(t *testing.T) {
	orbit := &operatorv1alpha1.OrbitAlloyDefaults{
		Enabled: true,
		Image:   "grafana/alloy:v1.14.2",
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			AppName:     "hedera-node",
			Environment: "production",
			LogLevel:    "INFO",
			Exporters: &operatorv1alpha1.AlloyExporters{
				Loki: &operatorv1alpha1.AlloyLokiExporter{
					Enabled:  true,
					Endpoint: "http://loki.observability:3100/loki/api/v1/push",
				},
			},
		},
	}
	capsule := &operatorv1alpha1.AlloyConfig{
		Enabled: true,
		Config: &operatorv1alpha1.AlloyCollectorConfig{
			LogLevel: "DEBUG", // override orbit's INFO
			Exporters: &operatorv1alpha1.AlloyExporters{
				// No Loki — should inherit orbit's
				PrometheusRemote: &operatorv1alpha1.AlloyPrometheusRemoteExporter{
					Enabled:  true,
					Endpoint: "http://prom:9090/api/v1/write",
				},
			},
		},
	}

	merged := mergeAlloyConfig(orbit, capsule)

	// Image from orbit (capsule empty)
	if merged.Image != "grafana/alloy:v1.14.2" {
		t.Errorf("expected orbit image, got %q", merged.Image)
	}
	// AppName from orbit (capsule empty)
	if merged.Config.AppName != "hedera-node" {
		t.Errorf("expected orbit appName, got %q", merged.Config.AppName)
	}
	// Environment from orbit (capsule empty)
	if merged.Config.Environment != "production" {
		t.Errorf("expected orbit environment, got %q", merged.Config.Environment)
	}
	// LogLevel from capsule (overrides orbit)
	if merged.Config.LogLevel != "DEBUG" {
		t.Errorf("expected capsule logLevel 'DEBUG', got %q", merged.Config.LogLevel)
	}
	// Loki from orbit (capsule didn't define it)
	if merged.Config.Exporters.Loki == nil || !merged.Config.Exporters.Loki.Enabled {
		t.Error("expected Loki inherited from orbit")
	}
	if merged.Config.Exporters.Loki.Endpoint != "http://loki.observability:3100/loki/api/v1/push" {
		t.Errorf("expected orbit Loki endpoint, got %q", merged.Config.Exporters.Loki.Endpoint)
	}
	// Prometheus from capsule
	if merged.Config.Exporters.PrometheusRemote == nil || !merged.Config.Exporters.PrometheusRemote.Enabled {
		t.Error("expected Prometheus from capsule")
	}
}

// --- Labels merge tests ---

func TestMergeLabels_BothNil(t *testing.T) {
	result := mergeLabels(nil, nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestMergeLabels_OrbitOnly(t *testing.T) {
	result := mergeLabels(map[string]string{"cluster": "us-east-1"}, nil)
	if result["cluster"] != "us-east-1" {
		t.Errorf("expected orbit label, got %v", result)
	}
}

func TestMergeLabels_CapsuleOverridesOrbit(t *testing.T) {
	orbit := map[string]string{"cluster": "us-east-1", "team": "infra"}
	capsule := map[string]string{"cluster": "eu-west-1", "env": "staging"}

	result := mergeLabels(orbit, capsule)

	if result["cluster"] != "eu-west-1" {
		t.Errorf("expected capsule override for 'cluster', got %q", result["cluster"])
	}
	if result["team"] != "infra" {
		t.Errorf("expected orbit label 'team' preserved, got %q", result["team"])
	}
	if result["env"] != "staging" {
		t.Errorf("expected capsule label 'env', got %q", result["env"])
	}
}

func TestMergeCollectorConfig_LabelsMerge(t *testing.T) {
	orbitCfg := &operatorv1alpha1.AlloyCollectorConfig{
		Labels: map[string]string{"cluster": "us-east-1", "team": "infra"},
	}
	capsuleCfg := &operatorv1alpha1.AlloyCollectorConfig{
		Labels: map[string]string{"cluster": "eu-west-1"},
	}

	merged := mergeAlloyCollectorConfig(orbitCfg, capsuleCfg)

	if merged.Labels["cluster"] != "eu-west-1" {
		t.Errorf("expected capsule label override, got %q", merged.Labels["cluster"])
	}
	if merged.Labels["team"] != "infra" {
		t.Errorf("expected orbit label inherited, got %q", merged.Labels["team"])
	}
}
