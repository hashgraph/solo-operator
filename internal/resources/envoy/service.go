// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"

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

// ServiceSubReconciler manages the Service for an EnvoyProxy
type ServiceSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.EnvoyProxy]
}

// NewServiceSubReconciler creates a new ServiceSubReconciler
func NewServiceSubReconciler(c client.Client, scheme *runtime.Scheme) *ServiceSubReconciler {
	return &ServiceSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.EnvoyProxy](c, scheme, "Service"),
	}
}

// Reconcile ensures the service exists and is up to date
func (r *ServiceSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildService(envoy)

	// Set owner reference
	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy service", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update spec if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating Envoy service", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec.Selector = desired.Spec.Selector
		existing.Spec.Type = desired.Spec.Type
		existing.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
		existing.Labels = desired.Labels
		for k, v := range desired.Annotations {
			if existing.Annotations == nil {
				existing.Annotations = make(map[string]string)
			}
			existing.Annotations[k] = v
		}

		// Preserve NodePorts when updating ports
		for i, port := range desired.Spec.Ports {
			if i < len(existing.Spec.Ports) {
				port.NodePort = existing.Spec.Ports[i].NodePort
				existing.Spec.Ports[i] = port
			} else {
				existing.Spec.Ports = append(existing.Spec.Ports, port)
			}
		}
		if len(desired.Spec.Ports) < len(existing.Spec.Ports) {
			existing.Spec.Ports = existing.Spec.Ports[:len(desired.Spec.Ports)]
		}

		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildService builds the desired service
func (r *ServiceSubReconciler) buildService(envoy *operatorv1alpha1.EnvoyProxy) *corev1.Service {
	selectorLabels := CommonLabels(envoy)
	resourceLabels := ResourceLabels(envoy)
	proxyPort := GetProxyPort(envoy)
	metricsPort := GetMetricsPort(envoy)

	// Map ProxyServiceType to a valid K8s ServiceType.
	// Ingress uses a ClusterIP service; the Ingress resource is managed by IngressSubReconciler.
	svcType := corev1.ServiceTypeLoadBalancer
	switch envoy.Spec.ServiceType {
	case operatorv1alpha1.ProxyServiceTypeClusterIP, operatorv1alpha1.ProxyServiceTypeIngress:
		svcType = corev1.ServiceTypeClusterIP
	case operatorv1alpha1.ProxyServiceTypeLoadBalancer:
		svcType = corev1.ServiceTypeLoadBalancer
	}

	// ExternalTrafficPolicy: default Local (only meaningful for non-ClusterIP types)
	extPolicy := envoy.Spec.ExternalTrafficPolicy
	if extPolicy == "" {
		extPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        envoy.Name,
			Namespace:   envoy.Namespace,
			Labels:      resourceLabels,
			Annotations: envoy.Spec.ServiceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: proxyPort, TargetPort: intstr.FromInt32(proxyPort), Protocol: corev1.ProtocolTCP},
				{Name: "metrics", Port: metricsPort, TargetPort: intstr.FromInt32(metricsPort), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	// Apply ExternalTrafficPolicy only for non-ClusterIP service types
	if svcType != corev1.ServiceTypeClusterIP {
		svc.Spec.ExternalTrafficPolicy = extPolicy
	}

	// Apply custom service spec if provided (overrides defaults)
	if envoy.Spec.ServiceSpec != nil {
		svc.Spec = *envoy.Spec.ServiceSpec
		if svc.Spec.Selector == nil {
			svc.Spec.Selector = selectorLabels
		}
	}

	return svc
}

// needsUpdate checks if the service needs to be updated
func (r *ServiceSubReconciler) needsUpdate(existing, desired *corev1.Service) bool {
	// Check type
	if existing.Spec.Type != desired.Spec.Type {
		return true
	}

	// Check ExternalTrafficPolicy
	if existing.Spec.ExternalTrafficPolicy != desired.Spec.ExternalTrafficPolicy {
		return true
	}

	// Check selector
	for k, v := range desired.Spec.Selector {
		if existing.Spec.Selector[k] != v {
			return true
		}
	}

	// Check ports
	if len(existing.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}
	for i, port := range desired.Spec.Ports {
		if i >= len(existing.Spec.Ports) {
			return true
		}
		existingPort := existing.Spec.Ports[i]
		if port.Name != existingPort.Name ||
			port.Port != existingPort.Port ||
			port.Protocol != existingPort.Protocol {
			return true
		}
	}

	// Check annotations (only desired keys; extra cloud-added annotations are preserved)
	for k, v := range desired.Annotations {
		if existing.Annotations[k] != v {
			return true
		}
	}

	return false
}
