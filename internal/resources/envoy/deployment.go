// SPDX-License-Identifier: Apache-2.0

package envoy

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

// DeploymentSubReconciler manages the Deployment for an EnvoyProxy
type DeploymentSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.EnvoyProxy]
}

// NewDeploymentSubReconciler creates a new DeploymentSubReconciler
func NewDeploymentSubReconciler(c client.Client, scheme *runtime.Scheme) *DeploymentSubReconciler {
	return &DeploymentSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.EnvoyProxy](c, scheme, "Deployment"),
	}
}

// Reconcile ensures the deployment exists and is up to date
func (r *DeploymentSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildDeployment(envoy)

	// Set owner reference
	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
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

	// Update spec if needed
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

// buildDeployment builds the desired deployment
func (r *DeploymentSubReconciler) buildDeployment(envoy *operatorv1alpha1.EnvoyProxy) *appsv1.Deployment {
	replicas := GetReplicas(envoy)
	labels := CommonLabels(envoy)
	proxyPort := GetProxyPort(envoy)
	adminPort := GetAdminPort(envoy)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  ContainerName,
							Image: GetImage(envoy),
							Args:  []string{"-c", "/etc/envoy/envoy.yaml"},
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: proxyPort, Protocol: corev1.ProtocolTCP},
								{Name: "admin", ContainerPort: adminPort, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      ConfigVolumeName,
									MountPath: ConfigMountPath,
									ReadOnly:  true,
								},
								{
									Name:      AdminVolumeName,
									MountPath: AdminMountPath,
								},
							},
							Env: BuildEnv(envoy),
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(adminPort),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(adminPort),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: ConfigVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(envoy)},
								},
							},
						},
						{
							Name: AdminVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return deploy
}

// needsUpdate checks if the deployment needs to be updated
func (r *DeploymentSubReconciler) needsUpdate(existing, desired *appsv1.Deployment) bool {
	// Compare replicas
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != *desired.Spec.Replicas {
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

	// Compare volumes
	if len(existing.Spec.Template.Spec.Volumes) != len(desired.Spec.Template.Spec.Volumes) {
		return true
	}

	return false
}
