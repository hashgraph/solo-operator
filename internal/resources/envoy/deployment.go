// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"

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
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type DeploymentSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.EnvoyProxy]
}

func NewDeploymentSubReconciler(c client.Client, scheme *runtime.Scheme) *DeploymentSubReconciler {
	return &DeploymentSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.EnvoyProxy](c, scheme, "Deployment"),
	}
}

func (r *DeploymentSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildDeployment(envoy)

	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy deployment", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating Envoy deployment", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DeploymentSubReconciler) buildDeployment(envoy *operatorv1alpha1.EnvoyProxy) *appsv1.Deployment {
	replicas := GetReplicas(envoy)
	selectorLabels := CommonLabels(envoy)
	resourceLabels := ResourceLabels(envoy)
	proxyPort := GetProxyPort(envoy)
	metricsPort := GetMetricsPort(envoy)
	adminPort := GetAdminPort(envoy)

	gracePeriod := envoy.Spec.TerminationGracePeriodSeconds
	if gracePeriod == 0 {
		gracePeriod = 10
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    resourceLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: resourceLabels,
				},
				Spec: corev1.PodSpec{
					NodeSelector:                  envoy.Spec.NodeSelector,
					Affinity:                      envoy.Spec.Affinity,
					Tolerations:                   envoy.Spec.Tolerations,
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers: []corev1.Container{
						{
							Name:           ContainerName,
							Image:          GetImage(envoy),
							Args:           []string{"-c", "/etc/envoy/envoy.yaml"},
							Ports:          buildContainerPorts(proxyPort, adminPort, metricsPort, envoy.Spec.ExposeAdmin),
							VolumeMounts:   buildVolumeMounts(envoy),
							Env:            BuildEnv(envoy),
							StartupProbe:   buildStartupProbe(metricsPort, envoy),
							LivenessProbe:  buildLivenessProbe(metricsPort, envoy),
							ReadinessProbe: buildReadinessProbe(metricsPort, envoy),
						},
					},
					Volumes: buildVolumes(envoy),
				},
			},
		},
	}

	return deploy
}

func buildContainerPorts(proxyPort, adminPort, metricsPort int32, exposeAdmin bool) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{Name: "grpc", ContainerPort: proxyPort, Protocol: corev1.ProtocolTCP},
		{Name: "metrics", ContainerPort: metricsPort, Protocol: corev1.ProtocolTCP},
	}
	if exposeAdmin {
		ports = append(ports, corev1.ContainerPort{Name: "admin", ContainerPort: adminPort, Protocol: corev1.ProtocolTCP})
	}
	return ports
}

func buildVolumeMounts(envoy *operatorv1alpha1.EnvoyProxy) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{Name: ConfigVolumeName, MountPath: ConfigMountPath, ReadOnly: true},
	}
	if envoy.Spec.TLSSecretName != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSMountPath,
			ReadOnly:  true,
		})
	}
	return mounts
}

func buildVolumes(envoy *operatorv1alpha1.EnvoyProxy) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: ConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(envoy)},
				},
			},
		},
	}
	if envoy.Spec.TLSSecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: TLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: envoy.Spec.TLSSecretName,
					Items: []corev1.KeyToPath{
						{Key: "tls.crt", Path: "tls.crt"},
						{Key: "tls.key", Path: "tls.key"},
					},
				},
			},
		})
	}
	return volumes
}

// buildStartupProbe returns a startup probe when the operator manages the config (spec.Files is empty).
// Custom configs (spec.Files set) may not expose /healthz, so probes are omitted.
func buildStartupProbe(metricsPort int32, envoy *operatorv1alpha1.EnvoyProxy) *corev1.Probe {
	if len(envoy.Spec.Files) > 0 {
		return nil
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.FromInt32(metricsPort),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    24, // 2 min window
	}
}

func buildLivenessProbe(metricsPort int32, envoy *operatorv1alpha1.EnvoyProxy) *corev1.Probe {
	if len(envoy.Spec.Files) > 0 {
		return nil
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.FromInt32(metricsPort),
			},
		},
		PeriodSeconds:    10,
		FailureThreshold: 3,
	}
}

func buildReadinessProbe(metricsPort int32, envoy *operatorv1alpha1.EnvoyProxy) *corev1.Probe {
	if len(envoy.Spec.Files) > 0 {
		return nil
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.FromInt32(metricsPort),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
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

	// Check pod template labels (captures AdditionalLabels changes)
	for k, v := range desired.Spec.Template.Labels {
		if existing.Spec.Template.Labels[k] != v {
			return true
		}
	}

	return false
}
