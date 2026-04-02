// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"
	"sort"
	"strings"

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

// managedAnnotationsAnnotation records which annotation keys were last written by the operator.
// This enables deletion of annotations removed from spec.serviceAnnotations.
const managedAnnotationsAnnotation = "operator.solo.hedera.com/managed-annotations"

// managedAnnotationsKey extracts the sorted annotation keys stored in the marker annotation.
func managedAnnotationsKeys(svc *corev1.Service) []string {
	val := svc.Annotations[managedAnnotationsAnnotation]
	if val == "" {
		return nil
	}
	return strings.Split(val, ",")
}

// setManagedAnnotations writes desired annotations into existing and removes any previously
// managed keys that are no longer in desired.
func setManagedAnnotations(existing *corev1.Service, desired map[string]string) {
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	// Remove previously managed keys no longer in desired
	for _, k := range managedAnnotationsKeys(existing) {
		if _, still := desired[k]; !still {
			delete(existing.Annotations, k)
		}
	}
	// Upsert desired keys
	for k, v := range desired {
		existing.Annotations[k] = v
	}
	// Record the new managed key set
	keys := make([]string, 0, len(desired))
	for k := range desired {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	existing.Annotations[managedAnnotationsAnnotation] = strings.Join(keys, ",")
}

// ServiceSubReconciler manages the Service for an HAProxyCapsule
type ServiceSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HAProxyCapsule]
}

// NewServiceSubReconciler creates a new ServiceSubReconciler
func NewServiceSubReconciler(c client.Client, scheme *runtime.Scheme) *ServiceSubReconciler {
	return &ServiceSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HAProxyCapsule](c, scheme, "Service"),
	}
}

// Reconcile ensures the service exists and is up to date
func (r *ServiceSubReconciler) Reconcile(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildService(haproxy)

	// Set owner reference
	if err := controllerutil.SetControllerReference(haproxy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating HAProxy service", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update spec if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating HAProxy service", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec.Selector = desired.Spec.Selector
		existing.Spec.Type = desired.Spec.Type
		existing.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
		existing.Labels = desired.Labels
		setManagedAnnotations(existing, haproxy.Spec.ServiceAnnotations)

		// Preserve NodePorts when updating ports
		for i, port := range desired.Spec.Ports {
			if i < len(existing.Spec.Ports) {
				port.NodePort = existing.Spec.Ports[i].NodePort
				existing.Spec.Ports[i] = port
			} else {
				existing.Spec.Ports = append(existing.Spec.Ports, port)
			}
		}
		// Trim extra ports if desired has fewer
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
func (r *ServiceSubReconciler) buildService(haproxy *operatorv1alpha1.HAProxyCapsule) *corev1.Service {
	selectorLabels := CommonLabels(haproxy)
	resourceLabels := ResourceLabels(haproxy)

	// Map ProxyServiceType to a valid K8s ServiceType.
	// Ingress uses a ClusterIP service; the Ingress resource is managed by IngressSubReconciler.
	svcType := corev1.ServiceTypeLoadBalancer
	switch haproxy.Spec.ServiceType {
	case operatorv1alpha1.ProxyServiceTypeClusterIP, operatorv1alpha1.ProxyServiceTypeIngress:
		svcType = corev1.ServiceTypeClusterIP
	case operatorv1alpha1.ProxyServiceTypeLoadBalancer:
		svcType = corev1.ServiceTypeLoadBalancer
	}

	// ExternalTrafficPolicy: default Local (only meaningful for non-ClusterIP types)
	extPolicy := haproxy.Spec.ExternalTrafficPolicy
	if extPolicy == "" {
		extPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
	}

	// Build annotations: user annotations + managed-keys marker
	annotations := make(map[string]string, len(haproxy.Spec.ServiceAnnotations)+1)
	for k, v := range haproxy.Spec.ServiceAnnotations {
		annotations[k] = v
	}
	managedKeys := make([]string, 0, len(haproxy.Spec.ServiceAnnotations))
	for k := range haproxy.Spec.ServiceAnnotations {
		managedKeys = append(managedKeys, k)
	}
	sort.Strings(managedKeys)
	annotations[managedAnnotationsAnnotation] = strings.Join(managedKeys, ",")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        haproxy.Name,
			Namespace:   haproxy.Namespace,
			Labels:      resourceLabels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: GetGrpcPort(haproxy), TargetPort: intstr.FromInt32(GetGrpcPort(haproxy)), Protocol: corev1.ProtocolTCP},
				{Name: "grpcs", Port: GetGrpcsPort(haproxy), TargetPort: intstr.FromInt32(GetGrpcsPort(haproxy)), Protocol: corev1.ProtocolTCP},
				{Name: "metrics", Port: GetMetricsPort(haproxy), TargetPort: intstr.FromInt32(GetMetricsPort(haproxy)), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	// Apply ExternalTrafficPolicy only for non-ClusterIP service types
	if svcType != corev1.ServiceTypeClusterIP {
		svc.Spec.ExternalTrafficPolicy = extPolicy
	}

	// Apply custom service spec if provided (overrides default)
	if haproxy.Spec.ServiceSpec != nil {
		svc.Spec = *haproxy.Spec.ServiceSpec
		// Ensure selector is set to our labels
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

	// Check ports (ignoring NodePort which is preserved)
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

	// Check annotations: compare desired keys (including managed-marker for deletion detection)
	for k, v := range desired.Annotations {
		if existing.Annotations[k] != v {
			return true
		}
	}

	// Check labels (only desired keys; extra labels are preserved)
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}

	return false
}
