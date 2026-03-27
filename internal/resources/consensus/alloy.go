// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// AlloyConfigMapSubReconciler manages the Alloy ConfigMap
type AlloyConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewAlloyConfigMapSubReconciler creates a new AlloyConfigMapSubReconciler
func NewAlloyConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *AlloyConfigMapSubReconciler {
	return &AlloyConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "AlloyConfig"),
	}
}

// AlloyConfigMapName returns the Alloy ConfigMap name for a capsule
func AlloyConfigMapName(capsule *operatorv1alpha1.ConsensusCapsule) string {
	return capsule.Name + "-alloy-config"
}

// Reconcile ensures the Alloy ConfigMap exists
func (r *AlloyConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	alloy, err := ResolveAlloyConfig(ctx, r.Client, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}
	if alloy == nil || !alloy.Enabled {
		return ctrl.Result{}, nil
	}

	desired := r.buildConfigMap(capsule, alloy)
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Alloy ConfigMap", "name", desired.Name)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing.Data, desired.Data) {
		logger.V(1).Info("Updating Alloy ConfigMap", "name", desired.Name)
		existing.Data = desired.Data
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// needsUpdate checks if the ConfigMap data has changed
func (r *AlloyConfigMapSubReconciler) needsUpdate(existing, desired map[string]string) bool {
	if len(existing) != len(desired) {
		return true
	}
	for k, v := range desired {
		if existing[k] != v {
			return true
		}
	}
	return false
}

// buildConfigMap builds the Alloy ConfigMap
func (r *AlloyConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, alloy *operatorv1alpha1.AlloyConfig) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AlloyConfigMapName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: map[string]string{
			"config.alloy": BuildAlloyRiverConfig(capsule, alloy),
		},
	}
}

// BuildAlloyRiverConfig generates the Alloy River configuration from CR fields
func BuildAlloyRiverConfig(capsule *operatorv1alpha1.ConsensusCapsule, alloy *operatorv1alpha1.AlloyConfig) string {
	var b strings.Builder

	cfg := alloy.Config

	appName := AlloyDefaultAppName
	environment := AlloyDefaultEnvironment
	logLevel := AlloyDefaultLogLevel

	if cfg != nil {
		if cfg.AppName != "" {
			appName = cfg.AppName
		}
		if cfg.Environment != "" {
			environment = cfg.Environment
		}
		if cfg.LogLevel != "" {
			logLevel = strings.ToLower(cfg.LogLevel)
		}
	}

	// Logging configuration
	fmt.Fprintf(&b, "logging {\n  level = %q\n}\n\n", logLevel)

	// Labels block — reused across components
	// Built-in labels are always present; user-defined labels from config are appended.
	nodeLabels := fmt.Sprintf(`{"app" = %q, "environment" = %q, "node" = %q`, appName, environment, capsule.Name)
	if cfg != nil && len(cfg.Labels) > 0 {
		// Sort keys for deterministic output
		keys := make([]string, 0, len(cfg.Labels))
		for k := range cfg.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			nodeLabels += fmt.Sprintf(`, %q = %q`, k, cfg.Labels[k])
		}
	}
	nodeLabels += "}"

	// --- Receivers ---

	// Log file discovery — enabled when Loki exporter is active
	lokiEnabled := cfg != nil && cfg.Exporters != nil && cfg.Exporters.Loki != nil && cfg.Exporters.Loki.Enabled
	if lokiEnabled {
		b.WriteString("// Log file discovery\n")
		b.WriteString("local.file_match \"hedera_logs\" {\n")
		b.WriteString("  path_targets = [{__path__ = \"/opt/hgcapp/services-hedera/HapiApp2.0/output/hgcaa.log\"}]\n")
		b.WriteString("}\n\n")

		b.WriteString("loki.source.file \"hedera_logs\" {\n")
		b.WriteString("  targets    = local.file_match.hedera_logs.targets\n")
		b.WriteString("  forward_to = [loki.process.add_labels.receiver]\n")
		b.WriteString("}\n\n")

		b.WriteString("loki.process \"add_labels\" {\n")
		b.WriteString("  stage.static_labels {\n")
		fmt.Fprintf(&b, "    values = %s\n", nodeLabels)
		b.WriteString("  }\n")
		b.WriteString("  forward_to = [loki.write.default.receiver]\n")
		b.WriteString("}\n\n")

		endpoint := "http://loki:3100/loki/api/v1/push"
		if cfg.Exporters.Loki.Endpoint != "" {
			endpoint = cfg.Exporters.Loki.Endpoint
		}

		b.WriteString("loki.write \"default\" {\n")
		fmt.Fprintf(&b, "  endpoint {\n    url = %q\n", endpoint)

		if cfg.Exporters.Loki.Auth != nil && cfg.Exporters.Loki.Auth.Enabled && cfg.Exporters.Loki.Auth.Secret != nil {
			b.WriteString("    basic_auth {\n")
			b.WriteString("      username = env(\"LOKI_USERNAME\")\n")
			b.WriteString("      password = env(\"LOKI_PASSWORD\")\n")
			b.WriteString("    }\n")
		}

		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}

	// --- Prometheus remote-write ---
	// The consensus node always exposes metrics at :9999 for direct scraping.
	// When enabled, Alloy scrapes :9999 and remote-writes to the configured endpoint.
	prometheusEnabled := cfg != nil && cfg.Exporters != nil && cfg.Exporters.PrometheusRemote != nil && cfg.Exporters.PrometheusRemote.Enabled
	if prometheusEnabled {
		scrapeInterval := "15s"
		if cfg.Exporters.PrometheusRemote.ScrapeInterval != "" {
			scrapeInterval = cfg.Exporters.PrometheusRemote.ScrapeInterval
		}

		b.WriteString("// Scrape consensus node metrics and remote-write\n")
		b.WriteString("prometheus.scrape \"hedera_metrics\" {\n")
		fmt.Fprintf(&b, "  targets = [{__address__ = \"localhost:%d\"}]\n", PrometheusPort)
		b.WriteString("  forward_to = [prometheus.remote_write.default.receiver]\n")
		fmt.Fprintf(&b, "  scrape_interval = %q\n", scrapeInterval)
		b.WriteString("}\n\n")

		endpoint := "http://prometheus:9090/api/v1/write"
		if cfg.Exporters.PrometheusRemote.Endpoint != "" {
			endpoint = cfg.Exporters.PrometheusRemote.Endpoint
		}

		b.WriteString("prometheus.remote_write \"default\" {\n")
		b.WriteString("  endpoint {\n")
		fmt.Fprintf(&b, "    url = %q\n", endpoint)
		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}

	return b.String()
}
