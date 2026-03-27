// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// ResolveAlloyConfig returns the effective AlloyConfig for a capsule by merging
// orbit-level defaults with capsule-level overrides. Capsule fields take precedence.
//
// Resolution order:
//  1. If the capsule has an explicit alloy config, it overrides orbit defaults field-by-field.
//  2. If the capsule has no alloy config, orbit defaults are used as-is.
//  3. If neither orbit nor capsule define alloy config, nil is returned.
func ResolveAlloyConfig(ctx context.Context, c client.Client, capsule *operatorv1alpha1.ConsensusCapsule) (*operatorv1alpha1.AlloyConfig, error) {
	logger := log.FromContext(ctx)

	capsuleAlloy := capsule.Spec.PodProperties.Containers.Alloy

	// Fetch orbit defaults. Only swallow NotFound (orbit may not exist yet);
	// propagate transient errors so the reconciler requeues.
	orbitDefaults, err := fetchOrbitAlloyDefaults(ctx, c, capsule.Spec.Orbit)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("Orbit not found, using capsule alloy config only", "orbit", capsule.Spec.Orbit)
			return capsuleAlloy, nil
		}
		return nil, err
	}

	// No orbit defaults — return capsule config as-is
	if orbitDefaults == nil {
		return capsuleAlloy, nil
	}

	// Orbit has defaults but capsule has no alloy config — build from orbit
	if capsuleAlloy == nil {
		return alloyConfigFromOrbitDefaults(orbitDefaults), nil
	}

	// Both exist — merge with capsule overriding orbit
	return mergeAlloyConfig(orbitDefaults, capsuleAlloy), nil
}

// fetchOrbitAlloyDefaults fetches the Orbit CR and returns its alloy defaults, or nil if not set.
func fetchOrbitAlloyDefaults(ctx context.Context, c client.Client, orbitName string) (*operatorv1alpha1.OrbitAlloyDefaults, error) {
	if orbitName == "" {
		return nil, nil
	}

	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		return nil, err
	}

	if orbit.Spec.Telemetry == nil || orbit.Spec.Telemetry.Alloy == nil {
		return nil, nil
	}

	return orbit.Spec.Telemetry.Alloy, nil
}

// alloyConfigFromOrbitDefaults creates an AlloyConfig from orbit-level defaults.
func alloyConfigFromOrbitDefaults(defaults *operatorv1alpha1.OrbitAlloyDefaults) *operatorv1alpha1.AlloyConfig {
	return &operatorv1alpha1.AlloyConfig{
		Enabled:         defaults.Enabled,
		Image:           defaults.Image,
		ImagePullPolicy: defaults.ImagePullPolicy,
		Resources:       defaults.Resources,
		Config:          defaults.Config,
	}
}

// mergeAlloyConfig merges orbit-level defaults with capsule-level overrides.
// Capsule fields take precedence when set. Exporters, processors, and receivers
// are merged at the section level (e.g., if capsule defines Loki, the entire
// Loki config comes from the capsule; if it doesn't, orbit's Loki is used).
func mergeAlloyConfig(orbitDefaults *operatorv1alpha1.OrbitAlloyDefaults, capsule *operatorv1alpha1.AlloyConfig) *operatorv1alpha1.AlloyConfig {
	merged := *capsule // shallow copy

	// Container-level fields: capsule wins if set, otherwise orbit
	if merged.Image == "" && orbitDefaults.Image != "" {
		merged.Image = orbitDefaults.Image
	}
	if merged.ImagePullPolicy == "" && orbitDefaults.ImagePullPolicy != "" {
		merged.ImagePullPolicy = orbitDefaults.ImagePullPolicy
	}
	if merged.Resources == nil && orbitDefaults.Resources != nil {
		merged.Resources = orbitDefaults.Resources
	}

	// Merge collector config
	merged.Config = mergeAlloyCollectorConfig(orbitDefaults.Config, capsule.Config)

	return &merged
}

// mergeAlloyCollectorConfig merges orbit and capsule AlloyCollectorConfig.
// Scalar fields (AppName, Environment, LogLevel) are merged field-by-field.
// Struct fields (Exporters, Processors, Receivers) are merged at section level.
func mergeAlloyCollectorConfig(orbitCfg, capsuleCfg *operatorv1alpha1.AlloyCollectorConfig) *operatorv1alpha1.AlloyCollectorConfig {
	if orbitCfg == nil {
		return capsuleCfg
	}
	if capsuleCfg == nil {
		// Copy orbit config so we don't share the pointer
		cfg := *orbitCfg
		return &cfg
	}

	merged := *capsuleCfg // shallow copy

	// Scalar fields: capsule wins if non-empty
	if merged.AppName == "" && orbitCfg.AppName != "" {
		merged.AppName = orbitCfg.AppName
	}
	if merged.Environment == "" && orbitCfg.Environment != "" {
		merged.Environment = orbitCfg.Environment
	}
	if merged.LogLevel == "" && orbitCfg.LogLevel != "" {
		merged.LogLevel = orbitCfg.LogLevel
	}

	// Labels: orbit provides base, capsule labels override/extend
	merged.Labels = mergeLabels(orbitCfg.Labels, capsuleCfg.Labels)

	// Section-level merge for exporters
	merged.Exporters = mergeAlloyExporters(orbitCfg.Exporters, capsuleCfg.Exporters)

	// Section-level: capsule wins if non-nil, otherwise orbit
	if merged.Processors == nil && orbitCfg.Processors != nil {
		merged.Processors = orbitCfg.Processors
	}
	if merged.Receivers == nil && orbitCfg.Receivers != nil {
		merged.Receivers = orbitCfg.Receivers
	}

	return &merged
}

// mergeAlloyExporters merges exporter configs at the individual exporter level.
// Each exporter (Loki, Prometheus) is independently inherited from orbit if
// the capsule doesn't define it.
func mergeAlloyExporters(orbitExp, capsuleExp *operatorv1alpha1.AlloyExporters) *operatorv1alpha1.AlloyExporters {
	if orbitExp == nil {
		return capsuleExp
	}
	if capsuleExp == nil {
		exp := *orbitExp
		return &exp
	}

	merged := *capsuleExp

	// If capsule doesn't define Loki, inherit from orbit
	if merged.Loki == nil && orbitExp.Loki != nil {
		merged.Loki = orbitExp.Loki
	}
	// If capsule doesn't define Prometheus, inherit from orbit
	if merged.PrometheusRemote == nil && orbitExp.PrometheusRemote != nil {
		merged.PrometheusRemote = orbitExp.PrometheusRemote
	}

	return &merged
}

// mergeLabels merges two label maps. Capsule labels override orbit labels for the same key.
func mergeLabels(orbitLabels, capsuleLabels map[string]string) map[string]string {
	if len(orbitLabels) == 0 && len(capsuleLabels) == 0 {
		return nil
	}
	merged := make(map[string]string, len(orbitLabels)+len(capsuleLabels))
	for k, v := range orbitLabels {
		merged[k] = v
	}
	for k, v := range capsuleLabels {
		merged[k] = v
	}
	return merged
}
