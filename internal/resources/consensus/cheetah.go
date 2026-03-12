// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"

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

const CheetahConfigMapName = "solo-cheetah-config"

// CheetahConfigMapSubReconciler manages the Cheetah ConfigMap
type CheetahConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewCheetahConfigMapSubReconciler creates a new CheetahConfigMapSubReconciler
func NewCheetahConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *CheetahConfigMapSubReconciler {
	return &CheetahConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "CheetahConfig"),
	}
}

// Reconcile ensures the Cheetah ConfigMap exists
func (r *CheetahConfigMapSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	records := capsule.Spec.PodProperties.Containers.RecordsUploader
	events := capsule.Spec.PodProperties.Containers.EventsUploader
	blocks := capsule.Spec.PodProperties.Containers.BlocksUploader

	if (records == nil || !records.Enabled) && (events == nil || !events.Enabled) && (blocks == nil || !blocks.Enabled) {
		return ctrl.Result{}, nil
	}

	desired := r.buildConfigMap(capsule, records, events, blocks)
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Cheetah ConfigMap", "name", desired.Name)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing.Data, desired.Data) {
		logger.V(1).Info("Updating Cheetah ConfigMap", "name", desired.Name)
		existing.Data = desired.Data
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildConfigMap builds the Cheetah ConfigMap with separate config files
func (r *CheetahConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, records, events, blocks *operatorv1alpha1.CheetahUploader) *corev1.ConfigMap {
	data := make(map[string]string)

	if blocks != nil && blocks.Enabled {
		data["cheetah-block-streams.yaml"] = r.buildSingleConfig("block-streams-uploader", "/opt/hgcapp/blockStreams", ".mf", []string{".blk.gz"}, "blockStreams", blocks)
	}

	if records != nil && records.Enabled {
		data["cheetah-record-streams.yaml"] = r.buildRecordConfig(records)
	}

	if events != nil && events.Enabled {
		data["cheetah-events-streams.yaml"] = r.buildSingleConfig("events-streams-uploader", "/opt/hgcapp/eventsStreams", ".evts_sig", []string{".evts", ".evts_sig"}, "eventsStreams", events)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CheetahConfigMapName,
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: data,
	}
}

// buildSingleConfig builds a single cheetah config file
func (r *CheetahConfigMapSubReconciler) buildSingleConfig(name, directory, pattern string, patterns []string, prefix string, uploader *operatorv1alpha1.CheetahUploader) string {
	logLevel := "Debug"
	if uploader.LogLevel != "" {
		logLevel = uploader.LogLevel
	}

	patternList := ""
	for i, p := range patterns {
		if i > 0 {
			patternList += ", "
		}
		patternList += fmt.Sprintf(`"%s"`, p)
	}

	return fmt.Sprintf(`log:
  fileLogging: false
  level: %s
  directory: /app/logs
  fileName: cheetah.log
  maxSize: 1
  maxBackups: 10
  maxAge: 30
  
pipelines:
  - name: %s
    enabled: true
    stopOnError: "false"
    scanner:
      directory: %s
      pattern: "%s"
      interval: 100ms
      batchSize: 1000
    processor:
      maxProcessors: "30"
      fileMatcherConfigs:
        - matcherType: basic
          patterns: [ %s ]
      flushDelay: 150ms
      markerCheckConfig:
        checkInterval: 150ms
        minSize: 925
        maxAttempts: 5
      retry:
        limit: 5
      storage:
        s3:
          enabled: false
          bucket: S3_BUCKET_NAME
          prefix: S3_%s_BUCKET_PREFIX
          endpoint: S3_ENDPOINT
          region: S3_BUCKET_REGION
          accessKey: S3_ACCESS_KEY
          secretKey: S3_SECRET_KEY
          useSsl: true
        gcs:
          enabled: false
          bucket: GCS_BUCKET_NAME
          prefix: GCS_%s_BUCKET_PREFIX
          endpoint: GCS_ENDPOINT
          region: GCS_BUCKET_REGION
          accessKey: GCS_ACCESS_KEY
          secretKey: GCS_SECRET_KEY
          useSsl: true
`, logLevel, name, directory, pattern, patternList, prefix, prefix)
}

// buildRecordConfig builds record streams config with sidecar support
func (r *CheetahConfigMapSubReconciler) buildRecordConfig(uploader *operatorv1alpha1.CheetahUploader) string {
	logLevel := "Debug"
	if uploader.LogLevel != "" {
		logLevel = uploader.LogLevel
	}

	return fmt.Sprintf(`log:
  fileLogging: false
  level: %s
  directory: /app/logs
  fileName: cheetah.log
  maxSize: 1
  maxBackups: 10
  maxAge: 30
  
pipelines:
  - name: record-streams-uploader
    enabled: true
    stopOnError: "false"
    scanner:
      directory: /opt/hgcapp/recordStreams
      pattern: ".rcd_sig"
      interval: 100ms
      batchSize: 1000
    processor:
      maxProcessors: "30"
      fileMatcherConfigs:
        - matcherType: basic
          patterns: [ ".rcd.gz", ".rcd_sig" ]
        - matcherType: sequential
          patterns: [ "sidecar/{{.markerName}}_##.rcd.gz" ]
      flushDelay: 150ms
      markerCheckConfig:
        checkInterval: 150ms
        minSize: 925
        maxAttempts: 5
      retry:
        limit: 5
      storage:
        s3:
          enabled: false
          bucket: S3_BUCKET_NAME
          prefix: S3_RECORD_STREAMS_BUCKET_PREFIX
          endpoint: S3_ENDPOINT
          region: S3_BUCKET_REGION
          accessKey: S3_ACCESS_KEY
          secretKey: S3_SECRET_KEY
          useSsl: true
        gcs:
          enabled: false
          bucket: GCS_BUCKET_NAME
          prefix: GCS_RECORD_STREAMS_BUCKET_PREFIX
          endpoint: GCS_ENDPOINT
          region: GCS_BUCKET_REGION
          accessKey: GCS_ACCESS_KEY
          secretKey: GCS_SECRET_KEY
          useSsl: true
`, logLevel)
}

// needsUpdate checks if the config needs to be updated
func (r *CheetahConfigMapSubReconciler) needsUpdate(existing, desired map[string]string) bool {
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
