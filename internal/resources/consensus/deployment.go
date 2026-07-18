// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/errx"
	"github.com/hashgraph/solo-operator/internal/resources/apipermissionproperties"
	"github.com/hashgraph/solo-operator/internal/resources/applicationoverrideproperties"
	"github.com/hashgraph/solo-operator/internal/resources/applicationproperties"
	"github.com/hashgraph/solo-operator/internal/resources/bootstrapproperties"
	"github.com/hashgraph/solo-operator/internal/resources/feeschedules"
	"github.com/hashgraph/solo-operator/internal/resources/log4j2config"
	"github.com/hashgraph/solo-operator/internal/resources/nodeproperties"
	"github.com/hashgraph/solo-operator/internal/resources/nodesettings"
	"github.com/hashgraph/solo-operator/internal/resources/simplefeesschedules"
	"github.com/hashgraph/solo-operator/internal/resources/throttlesconfig"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
	"github.com/hashgraph/solo-operator/internal/uc"
)

// StatefulSetSubReconciler manages the StatefulSet for a ConsensusCapsule.
type StatefulSetSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
	Recorder events.EventRecorder
}

// NewStatefulSetSubReconciler creates a new StatefulSetSubReconciler.
func NewStatefulSetSubReconciler(c client.Client, scheme *runtime.Scheme, recorder events.EventRecorder) *StatefulSetSubReconciler {
	return &StatefulSetSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "StatefulSet"),
		Recorder:          recorder,
	}
}

// Reconcile ensures the StatefulSet exists and is up to date.
func (r *StatefulSetSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
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
			common.Info(ctx, common.ReasonStatefulSetCreated).Str("statefulSet", desired.Name).Msg("Creating StatefulSet")
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		common.Debug(ctx, common.ReasonStatefulSetUpdated).Str("statefulSet", desired.Name).Msg("Updating StatefulSet")
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildStatefulSet builds the desired StatefulSet purely from the capsule spec; it
// never reads the existing StatefulSet, so an omitted sidecar section is removed.
func (r *StatefulSetSubReconciler) buildStatefulSet(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (*appsv1.StatefulSet, error) {
	replicas := int32(1)
	if capsule.Spec.Offline {
		replicas = 0
	}
	selectorLabels := SelectorLabels(capsule)
	podLabels := PodLabels(capsule)

	volumes, volumeMounts := r.buildVolumes(capsule)

	consensusContainer, err := r.buildConsensusNodeContainer(capsule, volumeMounts)
	if err != nil {
		return nil, err
	}

	revisionHistoryLimit := int32(10)
	terminationGracePeriod := int64(30)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: capsule.Name,
			// RollingUpdate terminates the pod and starts a new one on spec change (single replica = same as Recreate).
			// Switch to OnDeleteStatefulSetStrategyType to require manual pod deletion before updates are applied.
			UpdateStrategy:       appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType},
			PodManagementPolicy:  appsv1.OrderedReadyPodManagement,
			RevisionHistoryLimit: &revisionHistoryLimit,
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: "Retain",
				WhenScaled:  "Retain",
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            ServiceAccountName(capsule),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					SchedulerName:                 corev1.DefaultSchedulerName,
					InitContainers:                []corev1.Container{r.buildGenesisInitContainer(capsule)},
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

	ucMode := uc.ModeClusterOnly
	if containers := capsule.Spec.PodProperties.Containers; containers.UC != nil && containers.UC.Enabled {
		ucMode, err = ResolveUCMode(ctx, r.Client, capsule)
		if err != nil {
			return nil, err
		}
	}

	if err := r.addSidecarContainers(ctx, sts, capsule, resolvedAlloy, ucMode); err != nil {
		return nil, err
	}

	r.applyPodProperties(sts, capsule)
	r.applyCheetahFSGroup(sts, capsule)
	// DeprecatedServiceAccount must mirror ServiceAccountName (set after applyPodProperties
	// which may override ServiceAccountName).
	sts.Spec.Template.Spec.DeprecatedServiceAccount = sts.Spec.Template.Spec.ServiceAccountName

	return sts, nil
}

// buildGenesisInitContainer copies genesis-network.json from the optional ConfigMap into
// the writable data-config-dir so the platform can archive it after the first round.
//
// Two sentinels detect whether this is a fresh cluster:
//  1. Platform state directory (/data/saved/SavedStateDir) — present on non-fresh nodes
//     (written after completing at least one round); exit immediately if present.
//  2. Dot-file (/data/saved/.genesis-init-done) — written after successful copy; guards
//     pod restarts before the platform creates its own state directory.
//
// If neither sentinel is present the cluster is fresh: we wait up to 10 minutes for the
// genesis-network.json ConfigMap to be mounted (the genesis NetworkOperation may still be
// in progress). Exits non-zero on timeout so the pod is restarted rather than starting
// without genesis configuration.
func (r *StatefulSetSubReconciler) buildGenesisInitContainer(capsule *operatorv1alpha1.ConsensusCapsule) corev1.Container {
	const (
		stateDir     = "/data/saved/" + SavedStateDir
		sentinelFile = "/data/saved/.genesis-init-done"
		genesisFile  = "/tmp/genesis-config/" + GenesisNetworkFileName
		destDir      = HapiBase + "/data/config/"
	)
	// Three sentinels in priority order: checkpoint CM (survives pod restarts),
	// platform state dir (written after round 1), dot-file (written after copy).
	script := `[ "$GENESIS_INITIALIZED" = "true" ] && { echo "genesis-init: checkpoint set, skipping"; exit 0; }
[ -d ` + stateDir + ` ] && exit 0
[ -f ` + sentinelFile + ` ] && exit 0
echo "genesis-init: fresh cluster, waiting for genesis-network.json (up to 10m)"
i=0
while [ ! -f ` + genesisFile + ` ]; do
  i=$((i+1))
  if [ $i -ge 60 ]; then
    echo "genesis-init: timed out waiting for genesis-network.json"
    exit 1
  fi
  sleep 10
done
cp ` + genesisFile + ` ` + destDir + ` && touch ` + sentinelFile + `
echo "genesis-init: done"`

	optional := true
	return corev1.Container{
		Name:    "genesis-init",
		Image:   GenesisInitImage,
		Command: []string{"sh", "-c", script},
		Env: []corev1.EnvVar{
			{
				Name: "GENESIS_INITIALIZED",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: CheckpointConfigMapName(capsule),
						},
						Key:      CheckpointKeyGenesisInitialized,
						Optional: &optional,
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "genesis-config", MountPath: "/tmp/genesis-config"},
			{Name: "data-config-dir", MountPath: HapiBase + "/data/config"},
			{Name: VolumeTypeSaved, MountPath: "/data/saved"},
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          corev1.PullIfNotPresent,
	}
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
		return corev1.Container{}, errx.WithHints(
			errx.WithReason(errConfigInvalid.New("consensus node image is required"), ReasonImageRequired),
			"Set spec.podProperties.containers.consensusNode.softwareVersion",
		)
	}

	env := []corev1.EnvVar{
		{Name: "CONSENSUS_NODE_ID", Value: fmt.Sprintf("%d", capsule.Spec.NodeId)},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "status.podIP"},
			},
		},
	}

	if cn.JavaHeapMin != "" {
		env = append(env, corev1.EnvVar{Name: "JAVA_HEAP_MIN", Value: cn.JavaHeapMin})
	}

	if cn.JavaHeapMax != "" {
		env = append(env, corev1.EnvVar{Name: "JAVA_HEAP_MAX", Value: cn.JavaHeapMax})
	}

	if cn.JavaOpts != "" {
		env = append(env, corev1.EnvVar{Name: "JAVA_OPTS", Value: cn.JavaOpts})
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
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          common.PullPolicyOrDefault(cn.SoftwareVersion, corev1.PullIfNotPresent),
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
	pvc := capsule.Spec.PersistentVolumeClaims

	// ── ConfigMap volumes ────────────────────────────────────────────────────
	defaultMode := int32(420) // 0644
	optionalTrue := true
	volumes := []corev1.Volume{
		{
			Name: "log4j2-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: log4j2config.ConfigMapNameForRef(capsule.Spec.Log4j2ConfigRef)},
					DefaultMode:          &defaultMode,
				},
			},
		},
		{
			Name: "settings-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: nodesettings.ConfigMapNameForRef(capsule.Spec.SettingsConfigRef)},
					DefaultMode:          &defaultMode,
				},
			},
		},
		{
			Name: "app-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: applicationproperties.ConfigMapNameForRef(capsule.Spec.ApplicationPropertiesRef)},
					DefaultMode:          &defaultMode,
				},
			},
		},
		{
			Name: "genesis-properties-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: DataConfigMapName(capsule)},
					DefaultMode:          &defaultMode,
				},
			},
		},
		// genesis-config is optional: absent until NetworkGenesis runs.
		{
			Name: "genesis-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: GenesisConfigMapName},
					DefaultMode:          &defaultMode,
					Optional:             &optionalTrue,
				},
			},
		},
		// data-config-dir is a writable emptyDir for data/config so genesis-network.json
		// can be archived (renamed) by the platform after the first round post-genesis.
		{
			Name:         "data-config-dir",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		// log4j2.xml is owned by the Log4j2Config CR's ConfigMap; takes effect on next pod restart.
		{Name: "log4j2-config", MountPath: HapiBase + "/log4j2.xml", SubPath: "log4j2.xml", ReadOnly: true},
		// settings.txt is owned by the NodeSettings CR's ConfigMap; takes effect on next pod restart.
		{Name: "settings-config", MountPath: HapiBase + "/settings.txt", SubPath: "settings.txt", ReadOnly: true},
		// data-config-dir is the writable data/config directory; individual files overlaid via subPath.
		{Name: "data-config-dir", MountPath: HapiBase + "/data/config"},
		{Name: "app-config", MountPath: HapiBase + "/data/config/application.properties", SubPath: "application.properties"},
		{Name: "genesis-properties-config", MountPath: HapiBase + "/data/config/" + GenesisPropertiesFileName, SubPath: GenesisPropertiesFileName},
	}

	// ── Optional config CR volumes (only mounted when the ref is set) ─────────
	optionalConfigs := []struct {
		ref        string
		volumeName string
		cmNameFn   func(string) string
		mountPath  string
		subPath    string
	}{
		{capsule.Spec.ApiPermissionPropertiesRef, "api-permission-config", apipermissionproperties.ConfigMapNameForRef, HapiBase + "/data/config/api-permission.properties", "api-permission.properties"},
		{capsule.Spec.NodePropertiesRef, "node-properties-config", nodeproperties.ConfigMapNameForRef, HapiBase + "/data/config/node.properties", "node.properties"},
		{capsule.Spec.ThrottlesConfigRef, "throttles-config", throttlesconfig.ConfigMapNameForRef, HapiBase + "/data/config/throttles.json", "throttles.json"},
		{capsule.Spec.FeeSchedulesRef, "fee-schedules-config", feeschedules.ConfigMapNameForRef, HapiBase + "/data/config/feeSchedules.json", "feeSchedules.json"},
		{capsule.Spec.SimpleFeesSchedulesRef, "simple-fee-schedules-config", simplefeesschedules.ConfigMapNameForRef, HapiBase + "/data/config/simpleFeesSchedules.json", "simpleFeesSchedules.json"},
		{capsule.Spec.ApplicationOverridePropertiesRef, "app-override-config", applicationoverrideproperties.ConfigMapNameForRef, HapiBase + "/data/config/application-override.properties", "application-override.properties"},
		{capsule.Spec.BootstrapPropertiesRef, "bootstrap-config", bootstrapproperties.ConfigMapNameForRef, HapiBase + "/data/config/bootstrap.properties", "bootstrap.properties"},
	}
	for _, om := range optionalConfigs {
		if om.ref == "" {
			continue
		}
		cmName := om.cmNameFn(om.ref)
		volumes = append(volumes, corev1.Volume{
			Name: om.volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
					DefaultMode:          &defaultMode,
					Optional:             &optionalTrue,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      om.volumeName,
			MountPath: om.mountPath,
			SubPath:   om.subPath,
			ReadOnly:  true,
		})
	}

	// block-nodes.json — only when BlockNodes is configured
	if capsule.Spec.BlockNodes != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "block-nodes-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: BlockNodesConfigMapName(capsule)},
					DefaultMode:          &defaultMode,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "block-nodes-config",
			MountPath: HapiBase + "/data/config/block-nodes.json",
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
		{VolumeTypeLogs, HapiBase + "/output", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Logs })},
		{VolumeTypeStats, HapiBase + "/data/stats", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Stats })},
		{VolumeTypeSaved, HapiBase + "/data/saved", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Saved })},
		{VolumeTypeState, HapiBase + "/state", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.State })},
		{VolumeTypeUpgrade, HapiBase + "/data/upgrade", pvcSpec(func(c *operatorv1alpha1.ConsensusPVCConfig) *corev1.PersistentVolumeClaimSpec { return c.Upgrade })},
		{VolumeTypeBlocks, PathBlockStreams, streamsSpec(func(s *operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec { return s.Block })},
		{VolumeTypeRecords, PathRecordStreams, streamsSpec(func(s *operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec { return s.Record })},
		{VolumeTypeEvents, PathEventsStreams, streamsSpec(func(s *operatorv1alpha1.ConsensusPVCStreams) *corev1.PersistentVolumeClaimSpec { return s.Events })},
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
					SecretName:  capsule.Spec.HapiAppSecretsName,
					DefaultMode: &defaultMode,
				},
			},
		})
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "hapi-app-secrets",
				MountPath: HapiBase + "/hedera.crt",
				SubPath:   "hedera.crt",
			},
			corev1.VolumeMount{
				Name:      "hapi-app-secrets",
				MountPath: HapiBase + "/hedera.key",
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
				Name: "keys", MountPath: HapiBase + "/data/keys",
			})
		} else if capsule.Spec.Secrets.SecretResources != nil {
			var projectedSources []corev1.VolumeProjection

			if capsule.Spec.Secrets.SecretResources.SigningPrivateKey.SecretName != "" {
				projectedSources = append(projectedSources, corev1.VolumeProjection{
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: capsule.Spec.Secrets.SecretResources.SigningPrivateKey.SecretName,
						},
						Items: []corev1.KeyToPath{{
							Key:  capsule.Spec.Secrets.SecretResources.SigningPrivateKey.KeyName,
							Path: fmt.Sprintf("s-private-%s.pem", capsule.Name),
						}},
					},
				})
			}

			// public signing certs for all nodes (optional: absent until NetworkGenesis runs)
			publicKeysOptional := true
			projectedSources = append(projectedSources, corev1.VolumeProjection{
				ConfigMap: &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: PublicKeysConfigMapName},
					Optional:             &publicKeysOptional,
				},
			})
			volumes = append(volumes, corev1.Volume{
				Name: "keys",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources:     projectedSources,
						DefaultMode: &defaultMode,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name: "keys", MountPath: HapiBase + "/data/keys",
			})

			sr := capsule.Spec.Secrets.SecretResources
			if sr.GrpcTlsPrivateKey.SecretName != "" {
				volumes = append(volumes, corev1.Volume{
					Name: "grpc-tls-key",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  sr.GrpcTlsPrivateKey.SecretName,
							Items:       []corev1.KeyToPath{{Key: sr.GrpcTlsPrivateKey.KeyName, Path: "hedera.key"}},
							DefaultMode: &defaultMode,
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: "grpc-tls-key", MountPath: HapiBase + "/hedera.key", SubPath: "hedera.key",
				})
			}
			if sr.GrpcTlsPublicCertificate.SecretName != "" {
				volumes = append(volumes, corev1.Volume{
					Name: "grpc-tls-cert",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  sr.GrpcTlsPublicCertificate.SecretName,
							Items:       []corev1.KeyToPath{{Key: sr.GrpcTlsPublicCertificate.KeyName, Path: "hedera.crt"}},
							DefaultMode: &defaultMode,
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name: "grpc-tls-cert", MountPath: HapiBase + "/hedera.crt", SubPath: "hedera.crt",
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
	// replaceOrAppend replaces an existing volume with the same name or appends it,
	// preventing duplicate volume names that the Kubernetes API would reject.
	replaceOrAppend := func(vols []corev1.Volume, v corev1.Volume) []corev1.Volume {
		for i, existing := range vols {
			if existing.Name == v.Name {
				vols[i] = v
				return vols
			}
		}
		return append(vols, v)
	}

	if capsule.Spec.PodProperties.Volumes != nil {
		podVols := capsule.Spec.PodProperties.Volumes
		for _, v := range []*corev1.Volume{
			podVols.Logs, podVols.Stats, podVols.Saved, podVols.Upgrade,
		} {
			if v != nil {
				volumes = replaceOrAppend(volumes, *v)
			}
		}
		if podVols.Streams != nil {
			for _, v := range []*corev1.Volume{
				podVols.Streams.Block, podVols.Streams.Record, podVols.Streams.Events,
			} {
				if v != nil {
					volumes = replaceOrAppend(volumes, *v)
				}
			}
		}
		for _, v := range podVols.AdditionalVolumes {
			volumes = replaceOrAppend(volumes, v)
		}
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

func (r *StatefulSetSubReconciler) event(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, eventtype string, reason errx.Reason, msg string) {
	common.Eventf(ctx, r.Recorder, capsule, eventtype, reason, "StatefulSet", "%s", msg)
}

// addSidecarContainers adds optional sidecar containers to the StatefulSet pod template.
func (r *StatefulSetSubReconciler) addSidecarContainers(ctx context.Context, sts *appsv1.StatefulSet, capsule *operatorv1alpha1.ConsensusCapsule, resolvedAlloy *operatorv1alpha1.AlloyConfig, ucMode uc.Mode) error {
	containers := capsule.Spec.PodProperties.Containers

	accountId := capsule.Spec.AccountId
	cheetahEnabled := (containers.BlocksUploader != nil && containers.BlocksUploader.Enabled) ||
		(containers.RecordsUploader != nil && containers.RecordsUploader.Enabled) ||
		(containers.EventsUploader != nil && containers.EventsUploader.Enabled)

	if cheetahEnabled {
		dm := int32(420)
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "solo-cheetah-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: CheetahConfigMapName(capsule)},
					DefaultMode:          &dm,
				},
			},
		})
	}

	type cheetahSpec struct {
		uploader      *operatorv1alpha1.CheetahUploader
		eventLabel    string
		containerName string
		configFile    string
		mountPath     string
		envPrefix     string
		bucketPath    string
	}
	uploaders := []cheetahSpec{
		{containers.BlocksUploader, "blockStreamUploader", "blockstream-uploader", "cheetah-block-streams.yaml", PathBlockStreams, "BLOCK_STREAMS", fmt.Sprintf("blockStreams/block-%s", accountId)},
		{containers.RecordsUploader, "recordStreamUploader", "record-stream-uploader", "cheetah-record-streams.yaml", PathRecordStreams, "RECORD_STREAMS", fmt.Sprintf("recordstreams/record%s", accountId)},
		{containers.EventsUploader, "eventStreamUploader", "event-stream-uploader", "cheetah-events-streams.yaml", PathEventsStreams, "EVENTS_STREAMS", fmt.Sprintf("eventsStreams/events_%s", accountId)},
	}
	for _, u := range uploaders {
		switch {
		case u.uploader == nil:
			r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventComponentSkipped,
				u.eventLabel+": omitted from spec; not present in desired StatefulSet")
		case !u.uploader.Enabled:
			r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventSidecarDisabled,
				u.eventLabel+": enabled=false; container removed from StatefulSet")
		default:
			c, err := r.buildCheetahUploaderContainer(u.uploader, u.containerName, u.configFile, u.mountPath, u.envPrefix, u.bucketPath)
			if err != nil {
				return fmt.Errorf("%s: %w", u.containerName, err)
			}
			sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, c)
		}
	}

	switch {
	case containers.BackupUploader == nil:
		r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventComponentSkipped,
			"backupUploader: omitted from spec; not present in desired StatefulSet")
	case !containers.BackupUploader.Enabled:
		r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventSidecarDisabled,
			"backupUploader: enabled=false; container removed from StatefulSet")
	default:
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers,
			r.buildBackupUploaderContainer(containers.BackupUploader))
	}

	switch {
	case containers.UC == nil:
		r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventComponentSkipped,
			"uc: omitted from spec; not present in desired StatefulSet")
	case !containers.UC.Enabled:
		r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventSidecarDisabled,
			"uc: enabled=false; container removed from StatefulSet")
	default:
		var ucDataSpec *corev1.PersistentVolumeClaimSpec
		if capsule.Spec.PersistentVolumeClaims != nil {
			ucDataSpec = capsule.Spec.PersistentVolumeClaims.UCData
		}
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
			volumeOrEmptyDir(VolumeTypeUCData, PVCName(capsule, VolumeTypeUCData), ucDataSpec),
		)
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers,
			r.buildUcContainer(capsule, containers.UC, ucMode))
	}

	if resolvedAlloy == nil || !resolvedAlloy.Enabled {
		r.event(ctx, capsule, corev1.EventTypeNormal, operatorv1alpha1.EventComponentSkipped,
			"alloy: omitted from spec; not present in desired StatefulSet")
	} else {
		dm := int32(420)
		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "alloy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: AlloyConfigMapName(capsule)},
					DefaultMode:          &dm,
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
		return corev1.Container{}, errx.WithHints(
			errx.WithReason(errConfigInvalid.New("blob storage is enabled (blob.enabled=true) but blob.url is not set"), ReasonBlobURLRequired),
			"Set blob.url on the uploader, or disable blob storage (blob.enabled=false)",
		)
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
		Env:                      env,
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          common.PullPolicyOrDefault(uploader.SoftwareVersion, corev1.PullIfNotPresent),
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

func (r *StatefulSetSubReconciler) buildUcContainer(capsule *operatorv1alpha1.ConsensusCapsule, config *operatorv1alpha1.UcSidecar, mode uc.Mode) corev1.Container {
	image := common.BuildImageOrDefault(config.SoftwareVersion, UCDefaultImage())

	name := config.Name
	if name == "" {
		name = "uc"
	}

	volumeMounts := []corev1.VolumeMount{
		// shared upgrade tree — the UC polls current/ and reads previous/ as fallback
		{Name: VolumeTypeUpgrade, MountPath: HapiBase + "/data/upgrade"},
		// single root for all UC runtime state; UC creates crs/, events/, backup/, uc.sock under here
		{Name: VolumeTypeUCData, MountPath: UCDataPath},
	}
	volumeMounts = append(volumeMounts, config.AdditionalVolumeMounts...)

	env := []corev1.EnvVar{
		{Name: "UC_NODE_ID", Value: fmt.Sprintf("%d", capsule.Spec.NodeId)},
		{Name: "UC_ORBIT", Value: capsule.Spec.Orbit},
		{Name: "UC_MODE", Value: string(mode)},
		{
			Name: "UC_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		},
		{
			// The platform-status sensor patches this pod's platform-status condition
			// (pods/status) and derives the owning ConsensusCapsule name from the pod
			// name (StatefulSet pod "<capsule>-<ordinal>").
			Name: "UC_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
	}

	container := corev1.Container{
		Name:                     name,
		Image:                    image,
		Env:                      env,
		VolumeMounts:             volumeMounts,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          common.PullPolicyOrDefault(config.SoftwareVersion, corev1.PullIfNotPresent),
	}

	if config.Resources != nil {
		container.Resources = *config.Resources
	}

	return container
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
// Path constants are defined alongside the volume type constants in common.go.
func getVolumeNameForPath(path string) string {
	switch path {
	case PathBlockStreams:
		return VolumeTypeBlocks
	case PathRecordStreams:
		return VolumeTypeRecords
	case PathEventsStreams:
		return VolumeTypeEvents
	default:
		return "unknown"
	}
}

// buildBackupUploaderContainer builds the backup uploader container (rclone-based).
// The backup.sh script inside the image requires BACKUP_UPLOADER_BUCKET_1 (rclone path,
// e.g. "backups:my-bucket") and RCLONE_CONFIG_BACKUPS_* env vars that configure the
// rclone "backups" remote. Credentials are injected from config.Config.EnvFromSecret.
func (r *StatefulSetSubReconciler) buildBackupUploaderContainer(config *operatorv1alpha1.StreamUploader) corev1.Container {
	image := common.BuildImageOrDefault(config.SoftwareVersion, "gcr.io/hedera-registry/hedera-backups:latest")

	name := config.Name
	if name == "" {
		name = "backup-uploader"
	}

	container := corev1.Container{
		Name:                     name,
		Image:                    image,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          common.PullPolicyOrDefault(config.SoftwareVersion, corev1.PullIfNotPresent),
	}

	if config.Resources != nil {
		container.Resources = *config.Resources
	}

	if len(config.AdditionalVolumeMounts) > 0 {
		container.VolumeMounts = config.AdditionalVolumeMounts
	}

	if cfg := config.Config; cfg != nil {
		var env []corev1.EnvVar

		// BACKUP_UPLOADER_BUCKET_1 is the rclone destination path: "backups:<bucketName>"
		if cfg.BucketName != "" {
			env = append(env, corev1.EnvVar{
				Name:  "BACKUP_UPLOADER_BUCKET_1",
				Value: "backups:" + cfg.BucketName,
			})
		}

		if cfg.S3 != nil && cfg.S3.Enabled {
			env = append(env, corev1.EnvVar{Name: "RCLONE_CONFIG_BACKUPS_TYPE", Value: "s3"})
			if cfg.S3.URL != "" {
				env = append(env, corev1.EnvVar{Name: "RCLONE_CONFIG_BACKUPS_ENDPOINT", Value: cfg.S3.URL})
			}
		} else if cfg.GCS != nil && cfg.GCS.Enabled {
			env = append(env, corev1.EnvVar{Name: "RCLONE_CONFIG_BACKUPS_TYPE", Value: "gcs"})
		}

		if len(env) > 0 {
			container.Env = env
		}

		// Credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, etc.)
		if cfg.EnvFromSecret != "" {
			container.EnvFrom = []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cfg.EnvFromSecret},
				}},
			}
		}
	}

	return container
}

// buildAlloyContainer builds the Grafana Alloy telemetry sidecar container.
func (r *StatefulSetSubReconciler) buildAlloyContainer(capsule *operatorv1alpha1.ConsensusCapsule, config *operatorv1alpha1.AlloyConfig) corev1.Container {
	containerName := config.Name
	if containerName == "" {
		containerName = "alloy"
	}

	image := common.BuildImageOrDefault(config.SoftwareVersion, AlloyDefaultImage)

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
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		ImagePullPolicy:          common.PullPolicyOrDefault(config.SoftwareVersion, corev1.PullIfNotPresent),
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
	if podProps.ServiceAccount != nil && podProps.ServiceAccount.Name != "" {
		sts.Spec.Template.Spec.ServiceAccountName = podProps.ServiceAccount.Name
	}
}

func (r *StatefulSetSubReconciler) needsUpdate(existing, desired *appsv1.StatefulSet) bool {
	return !apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) ||
		!apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels)
}
