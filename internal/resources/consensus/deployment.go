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

	// Build container image
	cn := capsule.Spec.PodProperties.Containers.ConsensusNode
	image := buildImage(cn.SoftwareVersion)

	// Build volumes and volume mounts
	volumes, volumeMounts := r.buildVolumes(capsule)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
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
						r.buildConsensusNodeContainer(capsule, image, volumeMounts),
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

	return deploy
}

// buildImage constructs the container image string
func buildImage(sv operatorv1alpha1.SoftwareVersion) string {
	if sv.ManifestHash != "" {
		return fmt.Sprintf("%s/%s@%s", sv.Repository, sv.ImageName, sv.ManifestHash)
	}
	return fmt.Sprintf("%s/%s:%s", sv.Repository, sv.ImageName, sv.ImageTag)
}

// buildConsensusNodeContainer builds the main consensus node container
func (r *DeploymentSubReconciler) buildConsensusNodeContainer(capsule *operatorv1alpha1.ConsensusCapsule, image string, volumeMounts []corev1.VolumeMount) corev1.Container {
	cn := capsule.Spec.PodProperties.Containers.ConsensusNode

	containerName := cn.Name
	if containerName == "" {
		containerName = "consensus-node"
	}

	container := corev1.Container{
		Name:  containerName,
		Image: image,
		Env: []corev1.EnvVar{
			{Name: "AUTO_START_CONSENSUS_SERVICE", Value: "false"},
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
		// ConfigMap for capsule-specific config (log4j2.xml)
		{
			Name: "capsule-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(capsule)},
				},
			},
		},
		// ConfigMap for consensus config (created by Orbit)
		{
			Name: "consensus-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ConsensusConfigMapName},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "capsule-config", MountPath: "/opt/hgcapp/services-hedera/HapiApp2.0/log4j2.xml", SubPath: "log4j2.xml"},
		{Name: "consensus-config", MountPath: "/mnt/cm/consensus-config"},
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
				Name: VolumeTypeLogs, MountPath: "/opt/hgcapp/output/logs",
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
				Name: VolumeTypeStats, MountPath: "/opt/hgcapp/output/stats",
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
				Name: VolumeTypeStates, MountPath: "/opt/hgcapp/output/states",
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
				Name: VolumeTypeUpgrade, MountPath: "/opt/hgcapp/output/upgrade",
			})
		}

		if pvcConfig.Streams != nil {
			if pvcConfig.Streams.Record != nil {
				volumes = append(volumes, corev1.Volume{
					Name: VolumeTypeRecordStreams,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: PVCName(capsule, VolumeTypeRecordStreams),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: VolumeTypeRecordStreams, MountPath: "/opt/hgcapp/output/recordStreams",
				})
			}

			if pvcConfig.Streams.Events != nil {
				volumes = append(volumes, corev1.Volume{
					Name: VolumeTypeEventsStreams,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: PVCName(capsule, VolumeTypeEventsStreams),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: VolumeTypeEventsStreams, MountPath: "/opt/hgcapp/output/eventsStreams",
				})
			}
		}
	}

	// Add pod-level volumes if configured
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
				Name: "keys", MountPath: "/opt/hgcapp/keys",
			})
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

	// Record stream uploader
	if containers.RecordStreamUploader != nil {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildStreamUploaderContainer("record-stream-uploader", containers.RecordStreamUploader))
	}

	// Record stream sidecar files uploader
	if containers.RecordStreamSidecarFilesUploader != nil {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildStreamUploaderContainer("record-stream-sidecar-uploader", containers.RecordStreamSidecarFilesUploader))
	}

	// Events stream uploader
	if containers.EventsStreamUploader != nil {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildStreamUploaderContainer("events-stream-uploader", containers.EventsStreamUploader))
	}

	// Backup uploader
	if containers.BackupUploader != nil {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildStreamUploaderContainer("backup-uploader", containers.BackupUploader))
	}

	// OpenTelemetry collector
	if containers.OpenTelemetryCollector != nil && containers.OpenTelemetryCollector.Enabled {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers,
			r.buildOtelCollectorContainer(capsule, containers.OpenTelemetryCollector))
	}
}

// buildStreamUploaderContainer builds a stream uploader sidecar container
func (r *DeploymentSubReconciler) buildStreamUploaderContainer(name string, config *operatorv1alpha1.StreamUploader) corev1.Container {
	container := corev1.Container{
		Name:  name,
		Image: buildImage(config.SoftwareVersion),
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

	container := corev1.Container{
		Name: containerName,
	}

	if config.SoftwareVersion != nil {
		container.Image = buildImage(*config.SoftwareVersion)
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
