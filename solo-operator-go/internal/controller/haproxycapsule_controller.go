// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

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
)

const (
	// HAProxyCapsuleFinalizer is the finalizer for HAProxyCapsule resources
	HAProxyCapsuleFinalizer = "operator.solo.hedera.com/haproxycapsule-finalizer"
)

// HAProxyCapsuleReconciler reconciles a HAProxyCapsule object
type HAProxyCapsuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=haproxycapsules/finalizers,verbs=update

// Reconcile is the main reconciliation loop for HAProxyCapsule resources
func (r *HAProxyCapsuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting HAProxyCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished HAProxyCapsule reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	// Fetch the HAProxyCapsule instance
	haproxy := &operatorv1alpha1.HAProxyCapsule{}
	if err := r.Get(ctx, req.NamespacedName, haproxy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("HAProxyCapsule resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get HAProxyCapsule")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !haproxy.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, haproxy)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(haproxy, HAProxyCapsuleFinalizer) {
		controllerutil.AddFinalizer(haproxy, HAProxyCapsuleFinalizer)
		if err := r.Update(ctx, haproxy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile ConfigMap
	if err := r.reconcileConfigMap(ctx, haproxy); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	if err := r.reconcileDeployment(ctx, haproxy); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, haproxy); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.updateStatus(ctx, haproxy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of a HAProxyCapsule resource
func (r *HAProxyCapsuleReconciler) handleDeletion(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(haproxy, HAProxyCapsuleFinalizer) {
		logger.Info("Deleting HAProxyCapsule",
			"namespace", haproxy.Namespace,
			"name", haproxy.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(haproxy, HAProxyCapsuleFinalizer)
		if err := r.Update(ctx, haproxy); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileConfigMap ensures the HAProxy config map exists
func (r *HAProxyCapsuleReconciler) reconcileConfigMap(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) error {
	logger := log.FromContext(ctx)

	cmName := fmt.Sprintf("%s-config", haproxy.Name)

	// Default HAProxy configuration
	haproxyConfig := `
global
    log stdout format raw local0

defaults
    log global
    mode tcp
    option tcplog
    timeout connect 10s
    timeout client 30s
    timeout server 30s

frontend grpc_frontend
    bind *:50211
    default_backend consensus_nodes

frontend grpcs_frontend
    bind *:50212
    default_backend consensus_nodes_tls

backend consensus_nodes
    balance roundrobin
    server node1 consensus:50211 check

backend consensus_nodes_tls
    balance roundrobin
    server node1 consensus:50212 check

listen stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 10s
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: haproxy.Namespace,
			Labels:    r.commonLabels(haproxy),
		},
		Data: map[string]string{
			"haproxy.cfg": haproxyConfig,
		},
	}

	if err := controllerutil.SetControllerReference(haproxy, cm, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating HAProxy config map", "name", cm.Name)
			return r.Create(ctx, cm)
		}
		return err
	}

	existing.Data = cm.Data
	return r.Update(ctx, existing)
}

// reconcileDeployment ensures the HAProxy deployment exists
func (r *HAProxyCapsuleReconciler) reconcileDeployment(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) error {
	logger := log.FromContext(ctx)

	replicas := int32(1)
	labels := r.commonLabels(haproxy)
	cmName := fmt.Sprintf("%s-config", haproxy.Name)

	image := haproxy.Spec.Image
	if image == "" {
		image = "haproxy:2.8.10-alpine3.20"
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      haproxy.Name,
			Namespace: haproxy.Namespace,
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
							Name:  "haproxy",
							Image: image,
							Ports: []corev1.ContainerPort{
								{Name: "grpc", ContainerPort: 50211, Protocol: corev1.ProtocolTCP},
								{Name: "grpcs", ContainerPort: 50212, Protocol: corev1.ProtocolTCP},
								{Name: "stats", ContainerPort: 8404, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/usr/local/etc/haproxy/haproxy.cfg",
									SubPath:   "haproxy.cfg",
								},
							},
							Env: haproxy.Spec.Env,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add secret volume if specified
	if haproxy.Spec.SecretName != "" {
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "tls-secrets",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: haproxy.Spec.SecretName,
				},
			},
		})
		deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "tls-secrets",
				MountPath: "/etc/haproxy/certs",
				ReadOnly:  true,
			},
		)
	}

	if err := controllerutil.SetControllerReference(haproxy, deploy, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: deploy.Name, Namespace: deploy.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating HAProxy deployment", "name", deploy.Name)
			return r.Create(ctx, deploy)
		}
		return err
	}

	existing.Spec = deploy.Spec
	return r.Update(ctx, existing)
}

// reconcileService ensures the HAProxy service exists
func (r *HAProxyCapsuleReconciler) reconcileService(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) error {
	logger := log.FromContext(ctx)

	svcType := corev1.ServiceTypeLoadBalancer
	if haproxy.Spec.ServiceType != "" {
		svcType = corev1.ServiceType(haproxy.Spec.ServiceType)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      haproxy.Name,
			Namespace: haproxy.Namespace,
			Labels:    r.commonLabels(haproxy),
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: r.commonLabels(haproxy),
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: 50211, TargetPort: intstr.FromInt32(50211), Protocol: corev1.ProtocolTCP},
				{Name: "grpcs", Port: 50212, TargetPort: intstr.FromInt32(50212), Protocol: corev1.ProtocolTCP},
				{Name: "stats", Port: 8404, TargetPort: intstr.FromInt32(8404), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	// Apply custom service spec if provided
	if haproxy.Spec.ServiceSpec != nil {
		svc.Spec = *haproxy.Spec.ServiceSpec
	}

	if err := controllerutil.SetControllerReference(haproxy, svc, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Name: svc.Name, Namespace: svc.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating HAProxy service", "name", svc.Name)
			return r.Create(ctx, svc)
		}
		return err
	}

	existing.Spec.Selector = svc.Spec.Selector
	existing.Spec.Type = svc.Spec.Type
	existing.Spec.Ports = svc.Spec.Ports
	return r.Update(ctx, existing)
}

// updateStatus updates the HAProxyCapsule status
func (r *HAProxyCapsuleReconciler) updateStatus(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) error {
	// Check deployment status
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		haproxy.Status.Phase = operatorv1alpha1.PhasePending
	} else {
		if deploy.Status.ReadyReplicas > 0 {
			haproxy.Status.Phase = operatorv1alpha1.PhaseRunning
		} else {
			haproxy.Status.Phase = operatorv1alpha1.PhasePending
		}
	}

	haproxy.Status.ObservedGeneration = haproxy.Generation

	return r.Status().Update(ctx, haproxy)
}

// commonLabels returns the common labels for resources
func (r *HAProxyCapsuleReconciler) commonLabels(haproxy *operatorv1alpha1.HAProxyCapsule) map[string]string {
	return map[string]string{
		LabelManagedBy: "solo-operator",
		LabelPartOf:    "haproxy",
		"app":          haproxy.Name,
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *HAProxyCapsuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.HAProxyCapsule{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Named("haproxycapsule").
		Complete(r)
}
