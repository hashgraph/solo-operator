// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// DeploymentSubReconciler manages the Deployment for a ConsensusCapsule
type DeploymentSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewDeploymentSubReconciler creates a new DeploymentSubReconciler
func NewDeploymentSubReconciler(c client.Client, scheme *runtime.Scheme) *DeploymentSubReconciler {
	return &DeploymentSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "Deployment"),
	}
}

// Reconcile ensures the deployment exists and is up to date
func (r *DeploymentSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildDeployment(capsule)

	// Set owner reference
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating deployment", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update spec if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating deployment", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildDeployment builds the desired deployment
func (r *DeploymentSubReconciler) buildDeployment(capsule *operatorv1alpha1.ConsensusCapsule) *appsv1.Deployment {
	replicas := int32(1)
	labels := PodLabels(capsule)

	// Build volumes and volume mounts
	volumes, volumeMounts := r.buildVolumes(capsule)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(capsule),
					Hostname:           capsule.Name,
					Subdomain:          ConsensusSubdomain,
					Containers: []corev1.Container{
						r.buildConsensusNodeContainer(capsule, volumeMounts),
					},
					Volumes: volumes,
				},
			},
		},
	}

	// Add sidecar containers
	r.addSidecarContainers(deploy, capsule)

	// Apply pod properties
	r.applyPodProperties(deploy, capsule)

	// Ensure shared fsGroup when cheetah uploaders are present so that all
	// containers in the pod share GID ownership of the stream volumes.
	// Must run after applyPodProperties to fill the gap only when the user
	// has not already configured fsGroup themselves.
	r.applyCheetahFSGroup(deploy, capsule)

	return deploy
}

// buildConsensusNodeContainer builds the main consensus node container
func (r *DeploymentSubReconciler) buildConsensusNodeContainer(capsule *operatorv1alpha1.ConsensusCapsule, volumeMounts []corev1.VolumeMount) corev1.Container {
	cn := capsule.Spec.PodProperties.Containers.ConsensusNode

	containerName := cn.Name
	if containerName == "" {
		containerName = "consensus-node"
	}

	image := common.BuildImageOrDefault(cn.SoftwareVersion, "gcr.io/hedera-registry/consensus-node:latest")

	container := corev1.Container{
		Name:  containerName,
		Image: image,
		Env: []corev1.EnvVar{
			{Name: "CONSENSUS_NODE_ID", Value: fmt.Sprintf("%d", capsule.Spec.NodeId)},
			{Name: "JAVA_OPTS", Value: fmt.Sprintf("-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=%d", DebugPort)},
		},
		Ports: []corev1.ContainerPort{
			{Name: "gossip", ContainerPort: GossipPort, Protocol: corev1.ProtocolTCP},
			{Name: "grpc", ContainerPort: GrpcPort, Protocol: corev1.ProtocolTCP},
			{Name: "grpcs", ContainerPort: GrpcsPort, Protocol: corev1.ProtocolTCP},
			{Name: "prometheus", ContainerPort: PrometheusPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: volumeMounts,
	}

	if cn.ImagePullPolicy != "" {
		container.ImagePullPolicy = cn.ImagePullPolicy
	}

	// Apply resources if configured
	if cn.Resources != nil {
		container.Resources = *cn.Resources
	}

	// Add additional volume mounts
	if len(cn.AdditionalVolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, cn.AdditionalVolumeMounts...)
	}

	return container
}

// buildVolumes builds the volumes and volume mounts for the deployment
func (r *DeploymentSubReconciler) buildVolumes(capsule *operatorv1alpha1.ConsensusCapsule) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: "capsule-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(capsule)},
				},
			},
		},
		{
			Name: "consensus-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ClusterConfigMapName},
				},
			},
		},
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ClusterConfigMapName},
					Items: []corev1.KeyToPath{
						{Key: "application.properties", Path: "application.properties"},
						{Key: "api-permission.properties", Path: "api-permission.properties"},
						{Key: "bootstrap.properties", Path: "bootstrap.properties"},
						{Key: "genesis-throttles.json", Path: "genesis-throttles.json"},
					},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "capsule-config", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/log4j2.xml", SubPath: "log4j2.xml"},
		{Name: "consensus-config", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/config.txt", SubPath: "config.txt"},
		{Name: "consensus-config", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/settings.txt", SubPath: "settings.txt"},
		{Name: "config-data", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/data/config"},
	}

	// Add PVC-based volumes if configured
	if capsule.Spec.PersistentVolumeClaims != nil {
		pvcConfig := capsule.Spec.PersistentVolumeClaims

		if pvcConfig.Logs != nil {
			volumes = append(volumes, corev1.Volume{
				Name: VolumeTypeLogs,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(capsule, VolumeTypeLogs),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: VolumeTypeLogs, MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/output",
			})
		}

		if pvcConfig.Stats != nil {
			volumes = append(volumes, corev1.Volume{
				Name: VolumeTypeStats,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(capsule, VolumeTypeStats),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: VolumeTypeStats, MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/data/stats",
			})
		}

		if pvcConfig.States != nil {
			volumes = append(volumes, corev1.Volume{
				Name: VolumeTypeStates,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(capsule, VolumeTypeStates),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: VolumeTypeStates, MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/data/saved",
			})
		}

		if pvcConfig.Upgrade != nil {
			volumes = append(volumes, corev1.Volume{
				Name: VolumeTypeUpgrade,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(capsule, VolumeTypeUpgrade),
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: VolumeTypeUpgrade, MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade",
			})
		}

		if pvcConfig.Streams != nil {
			if pvcConfig.Streams.Block != nil {
				volumes = append(volumes, corev1.Volume{
					Name: VolumeTypeBlocks,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: PVCName(capsule, VolumeTypeBlocks),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: VolumeTypeBlocks, MountPath: "/opt/hgcapp/blockStreams",
				})
			}

			if pvcConfig.Streams.Record != nil {
				volumes = append(volumes, corev1.Volume{
					Name: VolumeTypeRecords,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: PVCName(capsule, VolumeTypeRecords),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: VolumeTypeRecords, MountPath: "/opt/hgcapp/recordStreams",
				})
			}

			if pvcConfig.Streams.Events != nil {
				volumes = append(volumes, corev1.Volume{
					Name: VolumeTypeEvents,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: PVCName(capsule, VolumeTypeEvents),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: VolumeTypeEvents, MountPath: "/opt/hgcapp/eventsStreams",
				})
			}
		}
	}

	// Add pod-level volumes if configured (overrides PVC volumes)
	if capsule.Spec.PodProperties.Volumes != nil {
		podVolumes := capsule.Spec.PodProperties.Volumes

		if podVolumes.Logs != nil {
			volumes = append(volumes, *podVolumes.Logs)
		}
		if podVolumes.Stats != nil {
			volumes = append(volumes, *podVolumes.Stats)
		}
		if podVolumes.States != nil {
			volumes = append(volumes, *podVolumes.States)
		}
		if podVolumes.Upgrade != nil {
			volumes = append(volumes, *podVolumes.Upgrade)
		}
		if podVolumes.Streams != nil {
			if podVolumes.Streams.Block != nil {
				volumes = append(volumes, *podVolumes.Streams.Block)
			}
			if podVolumes.Streams.Record != nil {
				volumes = append(volumes, *podVolumes.Streams.Record)
			}
			if podVolumes.Streams.Events != nil {
				volumes = append(volumes, *podVolumes.Streams.Events)
			}
		}
		if len(podVolumes.AdditionalVolumes) > 0 {
			volumes = append(volumes, podVolumes.AdditionalVolumes...)
		}
	}

	// Add secrets volumes if configured
	if capsule.Spec.Secrets != nil {
		if capsule.Spec.Secrets.HostPath != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "keys",
				VolumeSource: corev1.VolumeSource{
					HostPath: capsule.Spec.Secrets.HostPath,
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: "keys", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/data/keys",
			})
		} else if capsule.Spec.Secrets.SecretResources != nil {
			// Use a projected volume to combine the node's signing keys (Secret)
			// with all nodes' public keys (ConfigMap) into a single mount
			var projectedSources []corev1.VolumeProjection

			// Add node's own signing keys from Secret
			secretItems := []corev1.KeyToPath{}
			if capsule.Spec.Secrets.SecretResources.SigningPrivateKey.SecretName != "" {
				secretItems = append(secretItems, corev1.KeyToPath{
					Key:  capsule.Spec.Secrets.SecretResources.SigningPrivateKey.KeyName,
					Path: capsule.Spec.Secrets.SecretResources.SigningPrivateKey.KeyName,
				})
			}
			if capsule.Spec.Secrets.SecretResources.SigningPublicCertificate.SecretName != "" {
				secretItems = append(secretItems, corev1.KeyToPath{
					Key:  capsule.Spec.Secrets.SecretResources.SigningPublicCertificate.KeyName,
					Path: capsule.Spec.Secrets.SecretResources.SigningPublicCertificate.KeyName,
				})
			}
			if len(secretItems) > 0 && capsule.Spec.Secrets.SecretResources.SigningPrivateKey.SecretName != "" {
				projectedSources = append(projectedSources, corev1.VolumeProjection{
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: capsule.Spec.Secrets.SecretResources.SigningPrivateKey.SecretName,
						},
						Items: secretItems,
					},
				})
			}

			// Add all nodes' public keys from the genesis-created ConfigMap
			projectedSources = append(projectedSources, corev1.VolumeProjection{
				ConfigMap: &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: PublicKeysConfigMapName,
					},
					Optional: func() *bool { t := true; return &t }(),
				},
			})

			if len(projectedSources) > 0 {
				volumes = append(volumes, corev1.Volume{
					Name: "keys",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: projectedSources,
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: "keys", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/data/keys",
				})
			}
		}
	}

	// Add config folder if configured
	if capsule.Spec.ConfigFolder != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "config-folder",
			VolumeSource: corev1.VolumeSource{
				HostPath: capsule.Spec.ConfigFolder,
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "config-folder", MountPath: "/mnt/config",
		})
	}

	return volumes, volumeMounts
}

// addSidecarContainers adds sidecar containers to the deployment
func (r *DeploymentSubReconciler) addSidecarContainers(deploy *appsv1.Deployment, capsule *operatorv1alpha1.ConsensusCapsule) {
	containers := capsule.Spec.PodProperties.Containers

	accountId := capsule.Spec.AccountId
	cheetahEnabled := (containers.BlocksUploader != nil && containers.BlocksUploader.Enabled) ||
		(containers.RecordsUploader != nil && containers.RecordsUploader.Enabled) ||
		(containers.EventsUploader != nil && containers.EventsUploader.Enabled)

	// Add Cheetah ConfigMap volume and init container if any cheetah uploader is enabled
	if cheetahEnabled {
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "solo-cheetah-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: CheetahConfigMapName},
				},
			},
		})
	}

	// Block streams uploader
	if containers.BlocksUploader != nil && containers.BlocksUploader.Enabled {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildCheetahUploaderContainer(capsule, containers.BlocksUploader, "blockstream-uploader", "cheetah-block-streams.yaml", "/opt/hgcapp/blockStreams", "BLOCK_STREAMS",
				fmt.Sprintf("blockStreams/block-%s", accountId)))
	}

	// Record streams uploader
	if containers.RecordsUploader != nil && containers.RecordsUploader.Enabled {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildCheetahUploaderContainer(capsule, containers.RecordsUploader, "record-stream-uploader", "cheetah-record-streams.yaml", "/opt/hgcapp/recordStreams", "RECORD_STREAMS",
				fmt.Sprintf("recordstreams/record%s", accountId)))
	}

	// Events streams uploader
	if containers.EventsUploader != nil && containers.EventsUploader.Enabled {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildCheetahUploaderContainer(capsule, containers.EventsUploader, "event-stream-uploader", "cheetah-events-streams.yaml", "/opt/hgcapp/eventsStreams", "EVENTS_STREAMS",
				fmt.Sprintf("eventsStreams/events_%s", accountId)))
	}

	// Legacy backup uploader (rclone-based)
	if containers.BackupUploader != nil && containers.BackupUploader.Enabled {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildBackupUploaderContainer(containers.BackupUploader))
	}

	// OpenTelemetry collector
	if containers.OpenTelemetryCollector != nil && containers.OpenTelemetryCollector.Enabled {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildOtelCollectorContainer(capsule, containers.OpenTelemetryCollector))
	}
}

// buildCheetahUploaderContainer builds a cheetah uploader sidecar container.
// streamBucketPath is the stream-specific path component (e.g. "blockStreams/block-0.0.3").
func (r *DeploymentSubReconciler) buildCheetahUploaderContainer(capsule *operatorv1alpha1.ConsensusCapsule, uploader *operatorv1alpha1.CheetahUploader, containerName, configFile, mountPath, envPrefix, streamBucketPath string) corev1.Container {
	image := common.BuildImageOrDefault(uploader.SoftwareVersion, CheetahDefaultImage)

	name := uploader.Name
	if name == "" {
		name = containerName
	}

	// Build volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "solo-cheetah-config",
			MountPath: "/app/config/cheetah.yaml",
			SubPath:   configFile,
			ReadOnly:  true,
		},
		{
			Name:      getVolumeNameForPath(mountPath),
			MountPath: mountPath,
		},
	}

	// Build environment variables
	env := []corev1.EnvVar{}

	bucketPath := cheetahBucketPath(uploader.BucketPrefix, streamBucketPath)

	// GCS configuration
	if uploader.GCS != nil {
		if uploader.GCS.Enabled {
			env = append(env, corev1.EnvVar{Name: "GCS_ENABLED", Value: "true"})
		} else {
			env = append(env, corev1.EnvVar{Name: "GCS_ENABLED", Value: "false"})
		}
		if uploader.GCS.Bucket != "" {
			env = append(env, corev1.EnvVar{Name: "GCS_BUCKET_NAME", Value: uploader.GCS.Bucket})
		}
		if uploader.GCS.Region != "" {
			env = append(env, corev1.EnvVar{Name: "GCS_BUCKET_REGION", Value: uploader.GCS.Region})
		}
		if uploader.GCS.Endpoint != "" {
			env = append(env, corev1.EnvVar{Name: "GCS_ENDPOINT", Value: uploader.GCS.Endpoint})
		}
		if uploader.GCS.UseSsl {
			env = append(env, corev1.EnvVar{Name: "GCS_USE_SSL", Value: "true"})
		} else {
			env = append(env, corev1.EnvVar{Name: "GCS_USE_SSL", Value: "false"})
		}
		env = append(env, corev1.EnvVar{Name: fmt.Sprintf("GCS_%s_BUCKET_PREFIX", envPrefix), Value: bucketPath})
	}

	// S3 configuration
	if uploader.S3 != nil {
		if uploader.S3.Enabled {
			env = append(env, corev1.EnvVar{Name: "S3_ENABLED", Value: "true"})
		} else {
			env = append(env, corev1.EnvVar{Name: "S3_ENABLED", Value: "false"})
		}
		if uploader.S3.Bucket != "" {
			env = append(env, corev1.EnvVar{Name: "S3_BUCKET_NAME", Value: uploader.S3.Bucket})
		}
		if uploader.S3.Region != "" {
			env = append(env, corev1.EnvVar{Name: "S3_BUCKET_REGION", Value: uploader.S3.Region})
		}
		if uploader.S3.Endpoint != "" {
			env = append(env, corev1.EnvVar{Name: "S3_ENDPOINT", Value: uploader.S3.Endpoint})
		}
		if uploader.S3.UseSsl {
			env = append(env, corev1.EnvVar{Name: "S3_USE_SSL", Value: "true"})
		} else {
			env = append(env, corev1.EnvVar{Name: "S3_USE_SSL", Value: "false"})
		}
		env = append(env, corev1.EnvVar{Name: fmt.Sprintf("S3_%s_BUCKET_PREFIX", envPrefix), Value: bucketPath})
	}

	// Blob configuration (multicloud via go-cloud)
	if uploader.Blob != nil {
		if uploader.Blob.Enabled {
			env = append(env, corev1.EnvVar{Name: "BLOB_ENABLED", Value: "true"})
		} else {
			env = append(env, corev1.EnvVar{Name: "BLOB_ENABLED", Value: "false"})
		}
		if uploader.Blob.URL != "" {
			env = append(env, corev1.EnvVar{Name: "BLOB_URL", Value: uploader.Blob.URL})
		}
		env = append(env, corev1.EnvVar{Name: fmt.Sprintf("BLOB_%s_PREFIX", envPrefix), Value: bucketPath})
	}

	configCheckInterval := CheetahDefaultConfigCheckInterval
	if uploader.ConfigCheckInterval != "" {
		configCheckInterval = uploader.ConfigCheckInterval
	}

	container := corev1.Container{
		Name:  name,
		Image: image,
		Command: []string{
			"/app/bin/cheetah",
			"upload",
			"--config",
			"/app/config/cheetah.yaml",
			"--config-check-interval",
			configCheckInterval,
		},
		Env:          env,
		VolumeMounts: volumeMounts,
		Ports: []corev1.ContainerPort{
			{Name: "pprof", ContainerPort: CheetahPprofPort, Protocol: corev1.ProtocolTCP},
			{Name: "stats", ContainerPort: CheetahStatsPort, Protocol: corev1.ProtocolTCP},
		},
	}

	if uploader.ImagePullPolicy != "" {
		container.ImagePullPolicy = uploader.ImagePullPolicy
	}

	if uploader.Resources != nil {
		container.Resources = *uploader.Resources
	}

	// Add envFrom for secrets
	if uploader.EnvFromSecret != "" {
		container.EnvFrom = []corev1.EnvFromSource{
			{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: uploader.EnvFromSecret}}},
		}
	}

	return container
}

// applyCheetahFSGroup sets fsGroup on the pod security context when any cheetah uploader
// is enabled. This ensures all containers in the pod share GID ownership of stream volumes,
// so cheetah can delete files written by the consensus node without needing an init container
// or a hardcoded per-container RunAsUser.
// Only fills in fsGroup if the user has not already set one.
func (r *DeploymentSubReconciler) applyCheetahFSGroup(deploy *appsv1.Deployment, capsule *operatorv1alpha1.ConsensusCapsule) {
	containers := capsule.Spec.PodProperties.Containers
	cheetahEnabled := (containers.BlocksUploader != nil && containers.BlocksUploader.Enabled) ||
		(containers.RecordsUploader != nil && containers.RecordsUploader.Enabled) ||
		(containers.EventsUploader != nil && containers.EventsUploader.Enabled)
	if !cheetahEnabled {
		return
	}

	if deploy.Spec.Template.Spec.SecurityContext == nil {
		deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if deploy.Spec.Template.Spec.SecurityContext.FSGroup == nil {
		fsGroup := int64(2000)
		deploy.Spec.Template.Spec.SecurityContext.FSGroup = &fsGroup
	}
}

// cheetahBucketPath returns the full bucket object prefix for a stream.
// If the uploader has a BucketPrefix set it is prepended; otherwise the stream-specific path is used as-is.
func cheetahBucketPath(bucketPrefix, streamPath string) string {
	if bucketPrefix != "" {
		return bucketPrefix + "/" + streamPath
	}
	return streamPath
}

// getVolumeNameForPath returns the volume name for a given mount path
func getVolumeNameForPath(path string) string {
	switch path {
	case "/opt/hgcapp/blockStreams":
		return VolumeTypeBlocks
	case "/opt/hgcapp/recordStreams":
		return VolumeTypeRecords
	case "/opt/hgcapp/eventsStreams":
		return VolumeTypeEvents
	default:
		return "unknown"
	}
}

// buildBackupUploaderContainer builds the backup uploader container (rclone-based)
func (r *DeploymentSubReconciler) buildBackupUploaderContainer(config *operatorv1alpha1.StreamUploader) corev1.Container {
	image := common.BuildImageOrDefault(config.SoftwareVersion, "gcr.io/hedera-registry/hedera-backups:latest")

	name := config.Name
	if name == "" {
		name = "backup-uploader"
	}

	container := corev1.Container{
		Name:  name,
		Image: image,
	}

	if config.ImagePullPolicy != "" {
		container.ImagePullPolicy = config.ImagePullPolicy
	}

	if config.Resources != nil {
		container.Resources = *config.Resources
	}

	if len(config.AdditionalVolumeMounts) > 0 {
		container.VolumeMounts = config.AdditionalVolumeMounts
	}

	return container
}

// buildOtelCollectorContainer builds the OpenTelemetry collector sidecar container
func (r *DeploymentSubReconciler) buildOtelCollectorContainer(capsule *operatorv1alpha1.ConsensusCapsule, config *operatorv1alpha1.OpenTelemetryCollectorConfig) corev1.Container {
	containerName := config.Name
	if containerName == "" {
		containerName = "open-telemetry-collector"
	}

	image := config.Image
	if image == "" {
		image = "otel/opentelemetry-collector-contrib:latest"
	}

	container := corev1.Container{
		Name:  containerName,
		Image: image,
	}

	if config.ImagePullPolicy != "" {
		container.ImagePullPolicy = config.ImagePullPolicy
	}

	if config.Resources != nil {
		container.Resources = *config.Resources
	}

	if len(config.AdditionalVolumeMounts) > 0 {
		container.VolumeMounts = config.AdditionalVolumeMounts
	}

	return container
}

// applyPodProperties applies pod-level properties to the deployment
func (r *DeploymentSubReconciler) applyPodProperties(deploy *appsv1.Deployment, capsule *operatorv1alpha1.ConsensusCapsule) {
	podProps := capsule.Spec.PodProperties

	if podProps.Affinity != nil {
		deploy.Spec.Template.Spec.Affinity = podProps.Affinity
	}

	if podProps.NodeSelector != nil {
		deploy.Spec.Template.Spec.NodeSelector = podProps.NodeSelector
	}

	if len(podProps.Tolerations) > 0 {
		deploy.Spec.Template.Spec.Tolerations = podProps.Tolerations
	}

	if len(podProps.TopologySpreadConstraints) > 0 {
		deploy.Spec.Template.Spec.TopologySpreadConstraints = podProps.TopologySpreadConstraints
	}

	if len(podProps.ResourceClaims) > 0 {
		deploy.Spec.Template.Spec.ResourceClaims = podProps.ResourceClaims
	}

	if podProps.SecurityContext != nil {
		deploy.Spec.Template.Spec.SecurityContext = podProps.SecurityContext
	}

	if podProps.Priority != nil {
		deploy.Spec.Template.Spec.Priority = podProps.Priority
	}

	if podProps.PriorityClassName != "" {
		deploy.Spec.Template.Spec.PriorityClassName = podProps.PriorityClassName
	}

	if podProps.PreemptionPolicy != "" {
		policy := corev1.PreemptionPolicy(podProps.PreemptionPolicy)
		deploy.Spec.Template.Spec.PreemptionPolicy = &policy
	}

	if podProps.TerminationGracePeriodSeconds != nil {
		deploy.Spec.Template.Spec.TerminationGracePeriodSeconds = podProps.TerminationGracePeriodSeconds
	}
}

// needsUpdate checks if the deployment needs to be updated
func (r *DeploymentSubReconciler) needsUpdate(existing, desired *appsv1.Deployment) bool {
	// Compare replicas
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	// Compare strategy
	if existing.Spec.Strategy.Type != desired.Spec.Strategy.Type {
		return true
	}

	// Compare containers (simplified check)
	if len(existing.Spec.Template.Spec.Containers) != len(desired.Spec.Template.Spec.Containers) {
		return true
	}

	for i, container := range desired.Spec.Template.Spec.Containers {
		if i >= len(existing.Spec.Template.Spec.Containers) {
			return true
		}
		existingContainer := existing.Spec.Template.Spec.Containers[i]
		if container.Image != existingContainer.Image || container.Name != existingContainer.Name {
			return true
		}
	}

	// Compare volumes (simplified check)
	if len(existing.Spec.Template.Spec.Volumes) != len(desired.Spec.Template.Spec.Volumes) {
		return true
	}

	return false
}
