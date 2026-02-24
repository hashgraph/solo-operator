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

	cheetah := capsule.Spec.PodProperties.Containers.Cheetah
	if cheetah == nil || !cheetah.Enabled {
		return ctrl.Result{}, nil
	}

	desired := r.buildConfigMap(capsule, cheetah)
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

// buildConfigMap builds the Cheetah ConfigMap
func (r *CheetahConfigMapSubReconciler) buildConfigMap(capsule *operatorv1alpha1.ConsensusCapsule, cheetah *operatorv1alpha1.CheetahConfig) *corev1.ConfigMap {
	config := r.buildConfig(cheetah)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CheetahConfigMapName,
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Data: map[string]string{
			"cheetah.yaml": config,
		},
	}
}

// buildConfig builds the Cheetah YAML configuration string
func (r *CheetahConfigMapSubReconciler) buildConfig(cheetah *operatorv1alpha1.CheetahConfig) string {
	logLevel := "Info"
	if cheetah.LogLevel != "" {
		logLevel = cheetah.LogLevel
	}

	config := fmt.Sprintf(`log:
  fileLogging: false
  level: %s
  directory: /app/data/logs
  fileName: cheetah.log
  maxSize: 1
  maxBackups: 10
  maxAge: 30
pipelines:
`, logLevel)

	config += r.buildPipeline("record-stream-uploader", "/opt/hgcapp/recordStreams", ".rcd_sig", []string{".rcd_sig", ".rcd"}, "record-streams", cheetah)
	config += r.buildPipeline("events-stream-uploader", "/opt/hgcapp/eventsStreams", ".evts_sig", []string{".evts_sig", ".evts"}, "events-streams", cheetah)
	config += r.buildPipeline("record-stream-sidecar-files-uploader", "/opt/hgcapp/recordStreams/sidecar", ".rcd", []string{".rcd"}, "record-streams/sidecar", cheetah)

	return config
}

// buildPipeline builds a single pipeline configuration
func (r *CheetahConfigMapSubReconciler) buildPipeline(name, path, pattern string, extensions []string, prefix string, cheetah *operatorv1alpha1.CheetahConfig) string {
	extList := ""
	for i, ext := range extensions {
		if i > 0 {
			extList += ", "
		}
		extList += fmt.Sprintf(`"%s"`, ext)
	}

	s3Enabled := "false"
	s3Bucket := "cheetah"
	s3Region := "us-east-1"
	s3Endpoint := ""
	s3UseSsl := "false"

	if cheetah.S3 != nil {
		if cheetah.S3.Enabled {
			s3Enabled = "true"
		}
		if cheetah.S3.Bucket != "" {
			s3Bucket = cheetah.S3.Bucket
		}
		if cheetah.S3.Region != "" {
			s3Region = cheetah.S3.Region
		}
		if cheetah.S3.Endpoint != "" {
			s3Endpoint = cheetah.S3.Endpoint
		}
		if cheetah.S3.UseSsl {
			s3UseSsl = "true"
		}
	}

	gcsEnabled := "false"
	gcsBucket := "cheetah"
	gcsRegion := "us-east-1"
	gcsEndpoint := "storage.googleapis.com"
	gcsUseSsl := "true"

	if cheetah.GCS != nil {
		if cheetah.GCS.Enabled {
			gcsEnabled = "true"
		}
		if cheetah.GCS.Bucket != "" {
			gcsBucket = cheetah.GCS.Bucket
		}
		if cheetah.GCS.Region != "" {
			gcsRegion = cheetah.GCS.Region
		}
		if cheetah.GCS.Endpoint != "" {
			gcsEndpoint = cheetah.GCS.Endpoint
		}
		if !cheetah.GCS.UseSsl {
			gcsUseSsl = "false"
		}
	}

	return fmt.Sprintf(`  - name: %s
    scanner:
      path: %s
      pattern: "%s"
      recursive: true
      interval: 100ms
    processor:
      maxProcessors: 10
      fileExtensions: [%s]
      retry:
        limit: 5
      storage:
        s3:
          enabled: %s
          bucket: %s
          region: %s
          prefix: cheetah/streams/%s
          endpoint: %s
          accessKey: S3_ACCESS_KEY
          secretKey: S3_SECRET_KEY
          useSsl: %s
        gcs:
          enabled: %s
          bucket: %s
          region: %s
          prefix: cheetah/streams/%s
          endpoint: %s
          accessKey: GCS_ACCESS_KEY
          secretKey: GCS_SECRET_KEY
          useSsl: %s
`, name, path, pattern, extList, s3Enabled, s3Bucket, s3Region, prefix, s3Endpoint, s3UseSsl,
		gcsEnabled, gcsBucket, gcsRegion, prefix, gcsEndpoint, gcsUseSsl)
}

// needsUpdate checks if the config needs to be updated
func (r *CheetahConfigMapSubReconciler) needsUpdate(existing, desired map[string]string) bool {
	return existing["cheetah.yaml"] != desired["cheetah.yaml"]
}
