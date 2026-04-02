// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"
	"crypto/sha256"
	"fmt"

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

const configChecksumAnnotation = "operator.solo.hedera.com/config-checksum"

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

	desired, err := r.buildDeployment(haproxy)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(haproxy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.Deployment{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
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

func (r *DeploymentSubReconciler) buildDeployment(haproxy *operatorv1alpha1.HAProxyCapsule) (*appsv1.Deployment, error) {
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
	if gracePeriod == 0 {
		gracePeriod = 10
	}

	cfgStr, err := HAProxyConfig(haproxy)
	if err != nil {
		return nil, err
	}
	configChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte(cfgStr)))

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
					Annotations: map[string]string{configChecksumAnnotation: configChecksum},
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

	return deploy, nil
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

	// Check config checksum annotation so ConfigMap changes trigger a rollout
	desiredAnnotations := desired.Spec.Template.Annotations
	existingAnnotations := existing.Spec.Template.Annotations
	return desiredAnnotations[configChecksumAnnotation] != existingAnnotations[configChecksumAnnotation]
}
