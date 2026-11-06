// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

const ConfigChecksumAnnotation = "operator.solo.hiero.org/config-checksum"

type DeploymentSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HAProxyCapsule]
}

func NewDeploymentSubReconciler(c client.Client, scheme *runtime.Scheme) *DeploymentSubReconciler {
	return &DeploymentSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HAProxyCapsule](c, scheme, "Deployment"),
	}
}

func (r *DeploymentSubReconciler) Reconcile(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	var checksum string
	if !HotReloadEnabled(haproxy) {
		var err error
		if checksum, err = restartChecksum(ctx, r.Client, haproxy); err != nil {
			return ctrl.Result{}, err
		}
	}
	desired := r.buildDeployment(haproxy, checksum)

	if err := controllerutil.SetControllerReference(haproxy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, common.ReasonDeploymentCreated).Msg("Creating HAProxy deployment")
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		common.Debug(ctx, common.ReasonDeploymentUpdated).Msg("Updating HAProxy deployment")
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
	}

	return ctrl.Result{}, nil
}

func (r *DeploymentSubReconciler) buildDeployment(haproxy *operatorv1alpha1.HAProxyCapsule, checksum string) *appsv1.Deployment {
	replicas := int32(1)
	selectorLabels := CommonLabels(haproxy)
	resourceLabels := ResourceLabels(haproxy)

	volumes, volumeMounts := buildVolumes(haproxy)

	hc := GetHealthCheck(haproxy)
	metricsPort := GetMetricsPort(haproxy)
	var startupProbe, livenessProbe, readinessProbe *corev1.Probe
	if hc.Enabled {
		startupProbe = buildStartupProbe(hc, metricsPort)
		livenessProbe = buildLivenessProbe(hc, metricsPort)
		readinessProbe = buildReadinessProbe(hc, metricsPort)
	}

	gracePeriod := haproxy.Spec.TerminationGracePeriodSeconds
	if gracePeriod == 0 {
		gracePeriod = 10
	}

	containers := []corev1.Container{
		{
			Name:    ContainerName,
			Image:   GetImage(haproxy),
			Command: []string{"haproxy"},
			Args:    []string{"-W", "-db", "-f", ConfigFilePath},
			Ports: []corev1.ContainerPort{
				{Name: "grpc", ContainerPort: GetGrpcPort(haproxy), Protocol: corev1.ProtocolTCP},
				{Name: "grpcs", ContainerPort: GetGrpcsPort(haproxy), Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: GetMetricsPort(haproxy), Protocol: corev1.ProtocolTCP},
			},
			VolumeMounts:    volumeMounts,
			Env:             haproxy.Spec.Env,
			StartupProbe:    startupProbe,
			LivenessProbe:   livenessProbe,
			ReadinessProbe:  readinessProbe,
			SecurityContext: common.RestrictedSecurityContext(),
		},
	}
	var podAnnotations map[string]string
	if HotReloadEnabled(haproxy) {
		containers[0].Args = append(containers[0].Args, "-S", fmt.Sprintf("ipv4@%s:%d", MasterCLIHost, MasterCLIPort))
		containers = append(containers, buildReloaderContainer(haproxy, volumeMounts))
	} else {
		podAnnotations = map[string]string{ConfigChecksumAnnotation: checksum}
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      haproxy.Name,
			Namespace: haproxy.Namespace,
			Labels:    resourceLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      resourceLabels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					NodeSelector:                  haproxy.Spec.NodeSelector,
					Affinity:                      haproxy.Spec.Affinity,
					Tolerations:                   haproxy.Spec.Tolerations,
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers:                    containers,
					Volumes:                       volumes,
				},
			},
		},
	}

	return deploy
}

// No subPath mounts: kubelet only propagates updates to directory mounts.
func buildVolumes(haproxy *operatorv1alpha1.HAProxyCapsule) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: ConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(haproxy)},
				},
			},
		},
	}
	mounts := []corev1.VolumeMount{
		{Name: ConfigVolumeName, MountPath: ConfigMountPath, ReadOnly: true},
	}

	for _, v := range resolve(haproxy).volumes {
		projections := make([]corev1.VolumeProjection, 0, len(v.sources))
		for _, src := range v.sources {
			projections = append(projections, src.projection())
		}
		volumes = append(volumes, corev1.Volume{
			Name:         v.name,
			VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: projections}},
		})
		mounts = append(mounts, corev1.VolumeMount{Name: v.name, MountPath: v.mountPath, ReadOnly: true})
	}

	return volumes, mounts
}

func buildReloaderContainer(haproxy *operatorv1alpha1.HAProxyCapsule, mounts []corev1.VolumeMount) corev1.Container {
	watchDirs := make([]string, 0, len(mounts))
	for _, m := range mounts {
		watchDirs = append(watchDirs, m.MountPath)
	}
	return corev1.Container{
		Name:    ReloaderContainerName,
		Image:   GetReloaderImage(haproxy),
		Command: []string{"/bin/sh", "-c", reloaderScript},
		Env: []corev1.EnvVar{
			{Name: "WATCH_DIRS", Value: strings.Join(watchDirs, " ")},
			{Name: "RELOAD_INTERVAL_SECONDS", Value: strconv.Itoa(int(GetReloadInterval(haproxy)))},
			{Name: "MASTER_CLI_HOST", Value: MasterCLIHost},
			{Name: "MASTER_CLI_PORT", Value: strconv.Itoa(int(MasterCLIPort))},
		},
		VolumeMounts:    mounts,
		SecurityContext: common.RestrictedSecurityContext(),
	}
}

// statsProbeHandler returns an httpGet probe on the HAProxy stats endpoint.
// The stats listener uses mode http so it responds with 200 when HAProxy is running.
func statsProbeHandler(metricsPort int32) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/stats",
			Port: intstr.FromInt32(metricsPort),
		},
	}
}

func buildStartupProbe(hc operatorv1alpha1.HAProxyHealthCheckSpec, metricsPort int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        statsProbeHandler(metricsPort),
		InitialDelaySeconds: hc.StartupInitialDelaySeconds,
		PeriodSeconds:       hc.StartupPeriodSeconds,
		FailureThreshold:    hc.StartupFailureThreshold,
		SuccessThreshold:    1,
		TimeoutSeconds:      hc.TimeoutSeconds,
	}
}

func buildLivenessProbe(hc operatorv1alpha1.HAProxyHealthCheckSpec, metricsPort int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:     statsProbeHandler(metricsPort),
		PeriodSeconds:    hc.PeriodSeconds,
		FailureThreshold: hc.FailureThreshold,
		SuccessThreshold: 1,
		TimeoutSeconds:   hc.TimeoutSeconds,
	}
}

func buildReadinessProbe(hc operatorv1alpha1.HAProxyHealthCheckSpec, metricsPort int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:     statsProbeHandler(metricsPort),
		PeriodSeconds:    hc.PeriodSeconds,
		FailureThreshold: hc.FailureThreshold,
		SuccessThreshold: 1,
		TimeoutSeconds:   hc.TimeoutSeconds,
	}
}

func (r *DeploymentSubReconciler) needsUpdate(existing, desired *appsv1.Deployment) bool {
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	ep := existing.Spec.Template.Spec
	dp := desired.Spec.Template.Spec

	if !equality.Semantic.DeepEqual(ep.NodeSelector, dp.NodeSelector) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.Affinity, dp.Affinity) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.Tolerations, dp.Tolerations) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.TerminationGracePeriodSeconds, dp.TerminationGracePeriodSeconds) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.Volumes, dp.Volumes) {
		return true
	}

	if len(ep.Containers) != len(dp.Containers) {
		return true
	}
	for i := range dp.Containers {
		ec := ep.Containers[i]
		dc := dp.Containers[i]
		if ec.Name != dc.Name || ec.Image != dc.Image {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.Command, dc.Command) || !equality.Semantic.DeepEqual(ec.Args, dc.Args) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.Ports, dc.Ports) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.Env, dc.Env) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.VolumeMounts, dc.VolumeMounts) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.StartupProbe, dc.StartupProbe) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.LivenessProbe, dc.LivenessProbe) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.ReadinessProbe, dc.ReadinessProbe) {
			return true
		}
	}

	// Check pod template labels bidirectionally (captures AdditionalLabels additions and removals)
	if len(existing.Spec.Template.Labels) != len(desired.Spec.Template.Labels) {
		return true
	}
	for k, v := range desired.Spec.Template.Labels {
		if existing.Spec.Template.Labels[k] != v {
			return true
		}
	}

	// Check config checksum annotation so ConfigMap changes trigger a rollout
	desiredAnnotations := desired.Spec.Template.Annotations
	existingAnnotations := existing.Spec.Template.Annotations
	return desiredAnnotations[ConfigChecksumAnnotation] != existingAnnotations[ConfigChecksumAnnotation]
}
