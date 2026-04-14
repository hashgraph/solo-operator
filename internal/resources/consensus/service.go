// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"

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

// ServiceSubReconciler manages the Service for a ConsensusCapsule
type ServiceSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

// NewServiceSubReconciler creates a new ServiceSubReconciler
func NewServiceSubReconciler(c client.Client, scheme *runtime.Scheme) *ServiceSubReconciler {
	return &ServiceSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "Service"),
	}
}

// Reconcile ensures the service exists
func (r *ServiceSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildService(capsule)

	// Set owner reference
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if exists
	existing := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating service", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update spec (preserve ClusterIP and NodePort)
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating service", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec.Selector = desired.Spec.Selector
		existing.Spec.Type = desired.Spec.Type
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations

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
func (r *ServiceSubReconciler) buildService(capsule *operatorv1alpha1.ConsensusCapsule) *corev1.Service {
	svcType := corev1.ServiceTypeLoadBalancer
	if capsule.Spec.ServiceType != "" {
		svcType = corev1.ServiceType(capsule.Spec.ServiceType)
	}

	gossipPort := int32(GossipPort)
	if capsule.Spec.GossipPort != 0 {
		gossipPort = capsule.Spec.GossipPort
	}
	grpcPort := int32(GrpcPort)
	if capsule.Spec.GrpcPort != 0 {
		grpcPort = capsule.Spec.GrpcPort
	}

	labels := CommonLabels(capsule)
	labels[LabelSoloType] = "network-node-svc"
	labels[LabelSoloPromEndpt] = "active"

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        capsule.Name,
			Namespace:   capsule.Namespace,
			Labels:      labels,
			Annotations: capsule.Spec.ServiceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type: svcType,
			Selector: map[string]string{
				LabelManagedBy: ManagedByValue,
				LabelPartOf:    PartOfConsensusNode,
				LabelNodeId:    fmt.Sprintf("%d", capsule.Spec.NodeId),
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "gossip",
					Port:       gossipPort,
					TargetPort: intstr.FromInt32(gossipPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc",
					Port:       grpcPort,
					TargetPort: intstr.FromInt32(grpcPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpcs",
					Port:       GrpcsPort,
					TargetPort: intstr.FromInt32(GrpcsPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       PrometheusPort,
					TargetPort: intstr.FromInt32(PrometheusPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// needsUpdate checks if the service needs to be updated
func (r *ServiceSubReconciler) needsUpdate(existing, desired *corev1.Service) bool {
	// Check type
	if existing.Spec.Type != desired.Spec.Type {
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
	return false
}
