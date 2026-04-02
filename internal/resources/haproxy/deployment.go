// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HAProxyCapsule]
}

func NewDeploymentSubReconciler(c client.Client, scheme *runtime.Scheme) *DeploymentSubReconciler {
	return &DeploymentSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HAProxyCapsule](c, scheme, "Deployment"),
	}
}

func (r *DeploymentSubReconciler) Reconcile(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildDeployment(haproxy)

	if err := controllerutil.SetControllerReference(haproxy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating HAProxy deployment", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating HAProxy deployment", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DeploymentSubReconciler) buildDeployment(haproxy *operatorv1alpha1.HAProxyCapsule) *appsv1.Deployment {
	replicas := int32(1)
	selectorLabels := CommonLabels(haproxy)
	resourceLabels := ResourceLabels(haproxy)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      ConfigVolumeName,
			MountPath: ConfigMountPath,
			SubPath:   ConfigFileName,
		},
	}

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

	// HAProxy 2.4+ auto-pairs tls.crt with tls.crt.key — keyToPath renames tls.key to make this work.
	// This is what makes the secret cert-manager compatible without any init containers.
	if haproxy.Spec.TLSSecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: TLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: haproxy.Spec.TLSSecretName,
					Items: []corev1.KeyToPath{
						{Key: "tls.crt", Path: "tls.crt"},
						{Key: "tls.key", Path: "tls.crt.key"},
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      TLSVolumeName,
			MountPath: TLSMountPath,
			ReadOnly:  true,
		})
	}

	hc := GetHealthCheck(haproxy)
	metricsPort := GetMetricsPort(haproxy)
	var startupProbe, livenessProbe, readinessProbe *corev1.Probe
	if hc.Enabled {
		startupProbe = buildStartupProbe(hc, metricsPort)
		livenessProbe = buildLivenessProbe(hc, metricsPort)
		readinessProbe = buildReadinessProbe(hc, metricsPort)
	}

	gracePeriod := haproxy.Spec.TerminationGracePeriodSeconds

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
					Labels: resourceLabels,
				},
				Spec: corev1.PodSpec{
					NodeSelector:                  haproxy.Spec.NodeSelector,
					Affinity:                      haproxy.Spec.Affinity,
					Tolerations:                   haproxy.Spec.Tolerations,
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers: []corev1.Container{
						{
							Name:  ContainerName,
							Image: GetImage(haproxy),
							Ports: []corev1.ContainerPort{
								{Name: "grpc", ContainerPort: GetGrpcPort(haproxy), Protocol: corev1.ProtocolTCP},
								{Name: "grpcs", ContainerPort: GetGrpcsPort(haproxy), Protocol: corev1.ProtocolTCP},
								{Name: "metrics", ContainerPort: GetMetricsPort(haproxy), Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts:   volumeMounts,
							Env:            haproxy.Spec.Env,
							StartupProbe:   startupProbe,
							LivenessProbe:  livenessProbe,
							ReadinessProbe: readinessProbe,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	return deploy
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
