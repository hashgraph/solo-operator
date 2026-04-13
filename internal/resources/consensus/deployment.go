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

// StatefulSetSubReconciler manages the StatefulSet for a ConsensusCapsule.
type StatefulSetSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewStatefulSetSubReconciler creates a new StatefulSetSubReconciler.
func NewStatefulSetSubReconciler(c client.Client, scheme *runtime.Scheme) *StatefulSetSubReconciler {
	return &StatefulSetSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "StatefulSet"),
	}
}

// Reconcile ensures the StatefulSet exists and is up to date.
func (r *StatefulSetSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired, err := r.buildStatefulSet(ctx, capsule)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating StatefulSet", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating StatefulSet", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildStatefulSet builds the desired StatefulSet.
func (r *StatefulSetSubReconciler) buildStatefulSet(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (*appsv1.StatefulSet, error) {
	replicas := int32(1)
	labels := PodLabels(capsule)

	volumes, volumeMounts := r.buildVolumes(capsule)

	consensusContainer, err := r.buildConsensusNodeContainer(capsule, volumeMounts)
	if err != nil {
		return nil, err
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: capsule.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName(capsule),
					Containers: []corev1.Container{
						consensusContainer,
					},
					Volumes: volumes,
				},
			},
		},
	}

	resolvedAlloy, err := ResolveAlloyConfig(ctx, r.Client, capsule)
	if err != nil {
		return nil, err
	}

	if err := r.addSidecarContainers(sts, capsule, resolvedAlloy); err != nil {
		return nil, err
	}

	r.applyPodProperties(sts, capsule)
	r.applyCheetahFSGroup(sts, capsule)

	return sts, nil
}

// buildConsensusNodeContainer builds the main consensus node container.
func (r *StatefulSetSubReconciler) buildConsensusNodeContainer(capsule *operatorv1alpha1.ConsensusCapsule, volumeMounts []corev1.VolumeMount) (corev1.Container, error) {
	cn := capsule.Spec.PodProperties.Containers.ConsensusNode

	containerName := cn.Name
	if containerName == "" {
		containerName = "consensus-node"
	}

	image := common.BuildImageOrDefault(cn.SoftwareVersion, "")
	if image == "" {
		return corev1.Container{}, fmt.Errorf("consensus node image is required: set spec.podProperties.containers.consensusNode.softwareVersion")
	}

	env := []corev1.EnvVar{
		{Name: "CONSENSUS_NODE_ID", Value: fmt.Sprintf("%d", capsule.Spec.NodeId)},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
	}

	if capsule.Spec.NodeAlias != "" {
		env = append(env, corev1.EnvVar{Name: "CONSENSUS_NODE_ALIAS", Value: capsule.Spec.NodeAlias})
	}

	if cn.JavaHeapMin != "" {
		env = append(env, corev1.EnvVar{Name: "JAVA_HEAP_MIN", Value: cn.JavaHeapMin})
	}

	if cn.JavaHeapMax != "" {
		env = append(env, corev1.EnvVar{Name: "JAVA_HEAP_MAX", Value: cn.JavaHeapMax})
	}

	container := corev1.Container{
		Name:  containerName,
		Image: image,
		Env:   env,
		Ports: []corev1.ContainerPort{
			{Name: "gossip", ContainerPort: GossipPort, Protocol: corev1.ProtocolTCP},
			{Name: "grpc", ContainerPort: GrpcPort, Protocol: corev1.ProtocolTCP},
			{Name: "grpcs", ContainerPort: GrpcsPort, Protocol: corev1.ProtocolTCP},
			{Name: "metrics", ContainerPort: PrometheusPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: volumeMounts,
	}

	if cn.ImagePullPolicy != "" {
		container.ImagePullPolicy = cn.ImagePullPolicy
	}

	if cn.SecurityContext != nil {
		container.SecurityContext = cn.SecurityContext
	}

	if cn.Resources != nil {
		container.Resources = *cn.Resources
	}

	if len(cn.AdditionalVolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, cn.AdditionalVolumeMounts...)
	}

	return container, nil
}

// buildVolumes builds all volumes and volume mounts for the StatefulSet pod.
//
// Volume strategy:
//   - ConfigMap-backed volumes: always created, pointing to operator-managed CMs.
//   - emptyDir-only volumes: always created as emptyDir (populated by image or ephemeral).
//   - PVC-optional volumes: created as PVC when configured, otherwise emptyDir fallback.
//   - Secret volumes: created only when the relevant spec field is set.
func (r *StatefulSetSubReconciler) buildVolumes(capsule *operatorv1alpha1.ConsensusCapsule) ([]corev1.Volume, []corev1.VolumeMount) {
	const hapiBase = "/opt/hgcapp/services-hedera/HapiApp2.0"

	pvc := capsule.Spec.PersistentVolumeClaims

	// ── ConfigMap volumes ────────────────────────────────────────────────────
	volumes := []corev1.Volume{
		{
			Name: "hapi-app-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: HapiAppConfigMapName(capsule)},
				},
			},
		},
		{
			Name: "data-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: DataConfigMapName(capsule)},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "hapi-app-config", MountPath: "/etc/network-node/config", ReadOnly: true},
		{Name: "data-config", MountPath: hapiBase + "/data/config"},
	}

	// genesis-network.json — namespace-scoped, optional (absent until genesis NetworkOperation runs)
	genesisOptional := true
	volumes = append(volumes, corev1.Volume{
		Name: "genesis-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: GenesisConfigMapName},
				Optional:             &genesisOptional,
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "genesis-config",
		MountPath: hapiBase + "/data/config/" + GenesisNetworkFileName,
		SubPath:   GenesisNetworkFileName,
		ReadOnly:  true,
	})

	// block-nodes.json — only when BlockNodes is configured
	if capsule.Spec.BlockNodes != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "block-nodes-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: BlockNodesConfigMapName(capsule)},
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "block-nodes-config",
			MountPath: hapiBase + "/data/config/block-nodes.json",
			SubPath:   "block-nodes.json",
		})
	}

	// ── PVC-optional volumes (emptyDir fallback) ─────────────────────────────
	pvcSpec := func(getter func(*operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec) *corev1.PersistentVolumeClaimSpec {
		if pvc == nil {
			return nil
		}
		return getter(pvc)
	}
	streamsSpec := func(getter func(*operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec) *corev1.PersistentVolumeClaimSpec {
		if pvc == nil || pvc.Streams == nil {
			return nil
		}
		return getter(pvc.Streams)
	}

	pvcVolumes := []struct {
		name      string
		mountPath string
		spec      *corev1.PersistentVolumeClaimSpec
	}{
		{VolumeTypeLogs, hapiBase + "/output", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Logs })},
		{VolumeTypeStats, hapiBase + "/data/stats", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Stats })},
		{VolumeTypeSaved, hapiBase + "/data/saved", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Saved })},
		{VolumeTypeState, hapiBase + "/state", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.State })},
		{VolumeTypeUpgrade, hapiBase + "/data/upgrade", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Upgrade })},
		{VolumeTypeBlocks, "/opt/hgcapp/blockStreams", streamsSpec(func(s *operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec { return s.Block })},
		{VolumeTypeRecords, "/opt/hgcapp/recordStreams", streamsSpec(func(s *operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec { return s.Record })},
		{VolumeTypeEvents, "/opt/hgcapp/eventsStreams", streamsSpec(func(s *operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec { return s.Events })},
	}

	for _, v := range pvcVolumes {
		volumes = append(volumes, volumeOrEmptyDir(v.name, PVCName(capsule, v.name), v.spec))
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: v.name, MountPath: v.mountPath})
	}

	// ── Secret volumes ───────────────────────────────────────────────────────

	// TLS cert + key (hedera.crt, hedera.key)
	if capsule.Spec.HapiAppSecretsName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "hapi-app-secrets",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: capsule.Spec.HapiAppSecretsName,
				},
			},
		})
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "hapi-app-secrets",
				MountPath: hapiBase + "/hedera.crt",
				SubPath:   "hedera.crt",
			},
			corev1.VolumeMount{
				Name:      "hapi-app-secrets",
				MountPath: hapiBase + "/hedera.key",
				SubPath:   "hedera.key",
			},
		)
	}

	// Node signing keys
	if capsule.Spec.Secrets != nil {
		if capsule.Spec.Secrets.HostPath != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "keys",
				VolumeSource: corev1.VolumeSource{
					HostPath: capsule.Spec.Secrets.HostPath,
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: "keys", MountPath: hapiBase + "/data/keys",
			})
		} else if capsule.Spec.Secrets.SecretResources != nil {
			var projectedSources []corev1.VolumeProjection

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
					Name: "keys", MountPath: hapiBase + "/data/keys",
				})
			}
		}
	}

	// ── Legacy hostPath config folder ────────────────────────────────────────
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

	// ── Pod-level volume overrides ───────────────────────────────────────────
	if capsule.Spec.PodProperties.Volumes != nil {
		podVols := capsule.Spec.PodProperties.Volumes
		for _, v := range []struct{ vol *corev1.Volume }{
			{podVols.Logs}, {podVols.Stats}, {podVols.Saved}, {podVols.Upgrade},
		} {
			if v.vol != nil {
				volumes = append(volumes, *v.vol)
			}
		}
		if podVols.Streams != nil {
			for _, v := range []struct{ vol *corev1.Volume }{
				{podVols.Streams.Block}, {podVols.Streams.Record}, {podVols.Streams.Events},
			} {
				if v.vol != nil {
					volumes = append(volumes, *v.vol)
				}
			}
		}
		volumes = append(volumes, podVols.AdditionalVolumes...)
	}

	return volumes, volumeMounts
}

// volumeOrEmptyDir returns a PVC-backed volume when spec is non-nil, otherwise an emptyDir volume.
func volumeOrEmptyDir(name, pvcName string, spec *corev1.PersistentVolumeClaimSpec) corev1.Volume {
	if spec != nil {
		return corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
			},
		}
	}
	return corev1.Volume{
		Name:         name,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}

// addSidecarContainers adds optional sidecar containers to the StatefulSet pod template.
func (r *StatefulSetSubReconciler) addSidecarContainers(sts *appsv1.StatefulSet, capsule *operatorv1alpha1.ConsensusCapsule, resolvedAlloy *operatorv1alpha1.AlloyConfig) error {
	containers := capsule.Spec.PodProperties.Containers

	accountId := capsule.Spec.AccountId
	cheetahEnabled := (containers.BlocksUploader != nil && containers.BlocksUploader.Enabled) ||
		(containers.RecordsUploader != nil && containers.RecordsUploader.Enabled) ||
		(containers.EventsUploader != nil && containers.EventsUploader.Enabled)

	if cheetahEnabled {
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "solo-cheetah-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: CheetahConfigMapName},
				},
			},
		})
	}

	if containers.BlocksUploader != nil && containers.BlocksUploader.Enabled {
		c, err := r.buildCheetahUploaderContainer(containers.BlocksUploader, "blockstream-uploader", "cheetah-block-streams.yaml", "/opt/hgcapp/blockStreams", "BLOCK_STREAMS",
			fmt.Sprintf("blockStreams/block-%s", accountId))
		if err != nil {
			return fmt.Errorf("blockstream-uploader: %w", err)
		}
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, c)
	}

	if containers.RecordsUploader != nil && containers.RecordsUploader.Enabled {
		c, err := r.buildCheetahUploaderContainer(containers.RecordsUploader, "record-stream-uploader", "cheetah-record-streams.yaml", "/opt/hgcapp/recordStreams", "RECORD_STREAMS",
			fmt.Sprintf("recordstreams/record%s", accountId))
		if err != nil {
			return fmt.Errorf("record-stream-uploader: %w", err)
		}
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, c)
	}

	if containers.EventsUploader != nil && containers.EventsUploader.Enabled {
		c, err := r.buildCheetahUploaderContainer(containers.EventsUploader, "event-stream-uploader", "cheetah-events-streams.yaml", "/opt/hgcapp/eventsStreams", "EVENTS_STREAMS",
			fmt.Sprintf("eventsStreams/events_%s", accountId))
		if err != nil {
			return fmt.Errorf("event-stream-uploader: %w", err)
		}
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, c)
	}

	if containers.BackupUploader != nil && containers.BackupUploader.Enabled {
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers,
			r.buildBackupUploaderContainer(containers.BackupUploader))
	}

	if resolvedAlloy != nil && resolvedAlloy.Enabled {
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "alloy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: AlloyConfigMapName(capsule)},
				},
			},
		})
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers,
			r.buildAlloyContainer(capsule, resolvedAlloy))
	}

	return nil
}

// buildCheetahUploaderContainer builds a cheetah uploader sidecar container.
func (r *StatefulSetSubReconciler) buildCheetahUploaderContainer(uploader *operatorv1alpha1.CheetahUploader, containerName, configFile, mountPath, envPrefix, streamBucketPath string) (corev1.Container, error) {
	if uploader.Blob != nil && uploader.Blob.Enabled && uploader.Blob.URL == "" {
		return corev1.Container{}, fmt.Errorf("blob storage is enabled (blob.enabled=true) but blob.url is not set")
	}

	image := common.BuildImageOrDefault(uploader.SoftwareVersion, CheetahDefaultImage)

	name := uploader.Name
	if name == "" {
		name = containerName
	}

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

	env := []corev1.EnvVar{}
	bucketPath := cheetahBucketPath(uploader.BucketPrefix, streamBucketPath)

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

	if uploader.EnvFromSecret != "" {
		container.EnvFrom = []corev1.EnvFromSource{
			{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: uploader.EnvFromSecret}}},
		}
	}

	return container, nil
}

// applyCheetahFSGroup sets fsGroup on the pod security context when any Cheetah uploader is enabled.
func (r *StatefulSetSubReconciler) applyCheetahFSGroup(sts *appsv1.StatefulSet, capsule *operatorv1alpha1.ConsensusCapsule) {
	containers := capsule.Spec.PodProperties.Containers
	cheetahEnabled := (containers.BlocksUploader != nil && containers.BlocksUploader.Enabled) ||
		(containers.RecordsUploader != nil && containers.RecordsUploader.Enabled) ||
		(containers.EventsUploader != nil && containers.EventsUploader.Enabled)
	if !cheetahEnabled {
		return
	}
	if sts.Spec.Template.Spec.SecurityContext == nil {
		sts.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if sts.Spec.Template.Spec.SecurityContext.FSGroup == nil {
		fsGroup := int64(2000)
		sts.Spec.Template.Spec.SecurityContext.FSGroup = &fsGroup
	}
}

// cheetahBucketPath returns the full bucket object prefix for a stream.
func cheetahBucketPath(bucketPrefix, streamPath string) string {
	if bucketPrefix != "" {
		return bucketPrefix + "/" + streamPath
	}
	return streamPath
}

// getVolumeNameForPath returns the volume name for a given Cheetah stream mount path.
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

// buildBackupUploaderContainer builds the backup uploader container (rclone-based).
func (r *StatefulSetSubReconciler) buildBackupUploaderContainer(config *operatorv1alpha1.StreamUploader) corev1.Container {
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

// buildAlloyContainer builds the Grafana Alloy telemetry sidecar container.
func (r *StatefulSetSubReconciler) buildAlloyContainer(capsule *operatorv1alpha1.ConsensusCapsule, config *operatorv1alpha1.AlloyConfig) corev1.Container {
	containerName := config.Name
	if containerName == "" {
		containerName = "alloy"
	}

	image := common.BuildImageOrDefault(config.SoftwareVersion, config.Image)
	if image == "" {
		image = AlloyDefaultImage
	}

	container := corev1.Container{
		Name:  containerName,
		Image: image,
		Args:  []string{"run", "/etc/alloy/config.alloy"},
		Ports: []corev1.ContainerPort{
			{Name: "alloy-metrics", ContainerPort: AlloyHTTPPort, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "alloy-config", MountPath: "/etc/alloy", ReadOnly: true},
		},
	}

	// Share the logs volume with the consensus node (read-only) for log scraping.
	lokiEnabled := config.Config != nil && config.Config.Exporters != nil &&
		config.Config.Exporters.Loki != nil && config.Config.Exporters.Loki.Enabled
	logsPVCExists := capsule.Spec.PersistentVolumeClaims != nil && capsule.Spec.PersistentVolumeClaims.Logs != nil
	if lokiEnabled && logsPVCExists {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      VolumeTypeLogs,
			MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/output",
			ReadOnly:  true,
		})
	}

	lokiAuth := config.Config != nil && config.Config.Exporters != nil &&
		config.Config.Exporters.Loki != nil && config.Config.Exporters.Loki.Auth != nil &&
		config.Config.Exporters.Loki.Auth.Enabled && config.Config.Exporters.Loki.Auth.Secret != nil
	if lokiAuth {
		secretName := config.Config.Exporters.Loki.Auth.Secret.Name
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name: "LOKI_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "username",
					},
				},
			},
			corev1.EnvVar{
				Name: "LOKI_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "password",
					},
				},
			},
		)
	}

	if config.ImagePullPolicy != "" {
		container.ImagePullPolicy = config.ImagePullPolicy
	}

	if config.Resources != nil {
		container.Resources = *config.Resources
	}

	if len(config.AdditionalVolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, config.AdditionalVolumeMounts...)
	}

	return container
}

// applyPodProperties applies pod-level properties to the StatefulSet pod template.
func (r *StatefulSetSubReconciler) applyPodProperties(sts *appsv1.StatefulSet, capsule *operatorv1alpha1.ConsensusCapsule) {
	podProps := capsule.Spec.PodProperties

	if podProps.Affinity != nil {
		sts.Spec.Template.Spec.Affinity = podProps.Affinity
	}
	if podProps.NodeSelector != nil {
		sts.Spec.Template.Spec.NodeSelector = podProps.NodeSelector
	}
	if len(podProps.Tolerations) > 0 {
		sts.Spec.Template.Spec.Tolerations = podProps.Tolerations
	}
	if len(podProps.TopologySpreadConstraints) > 0 {
		sts.Spec.Template.Spec.TopologySpreadConstraints = podProps.TopologySpreadConstraints
	}
	if len(podProps.ResourceClaims) > 0 {
		sts.Spec.Template.Spec.ResourceClaims = podProps.ResourceClaims
	}
	if podProps.SecurityContext != nil {
		sts.Spec.Template.Spec.SecurityContext = podProps.SecurityContext
	}
	if podProps.Priority != nil {
		sts.Spec.Template.Spec.Priority = podProps.Priority
	}
	if podProps.PriorityClassName != "" {
		sts.Spec.Template.Spec.PriorityClassName = podProps.PriorityClassName
	}
	if podProps.PreemptionPolicy != "" {
		policy := corev1.PreemptionPolicy(podProps.PreemptionPolicy)
		sts.Spec.Template.Spec.PreemptionPolicy = &policy
	}
	if podProps.TerminationGracePeriodSeconds != nil {
		sts.Spec.Template.Spec.TerminationGracePeriodSeconds = podProps.TerminationGracePeriodSeconds
	}
}

// needsUpdate checks if the StatefulSet needs to be updated.
func (r *StatefulSetSubReconciler) needsUpdate(existing, desired *appsv1.StatefulSet) bool {
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}
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
	return len(existing.Spec.Template.Spec.Volumes) != len(desired.Spec.Template.Spec.Volumes)
}
