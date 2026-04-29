// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// HeadlessServiceSubReconciler manages the consensus headless service for an Orbit
type HeadlessServiceSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.Orbit]
}

// NewHeadlessServiceSubReconciler creates a new HeadlessServiceSubReconciler
func NewHeadlessServiceSubReconciler(c client.Client, scheme *runtime.Scheme) *HeadlessServiceSubReconciler {
	return &HeadlessServiceSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.Orbit](c, scheme, "ConsensusHeadlessService"),
	}
}

// Reconcile ensures the consensus headless service exists
func (r *HeadlessServiceSubReconciler) Reconcile(ctx context.Context, orbit *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildService(orbit)

	// Check if exists
	existing := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating consensus headless service", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update spec if needed (preserve ClusterIP to avoid conflicts)
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating consensus headless service", "name", desired.Name, "namespace", desired.Namespace)
		existing.Spec.Selector = desired.Spec.Selector
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.PublishNotReadyAddresses = desired.Spec.PublishNotReadyAddresses
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildService builds the desired headless service
func (r *HeadlessServiceSubReconciler) buildService(orbit *operatorv1alpha1.Orbit) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ConsensusSubdomain,
			Namespace: orbit.Name,
			Labels:    StandardLabels(orbit),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				LabelManagedBy: ManagedByValue,
				LabelPartOf:    PartOfConsensusNode,
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "gossip",
					Port:     GossipPort,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "grpc",
					Port:     GrpcPort,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "grpcs",
					Port:     GrpcsPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

// needsUpdate checks if the service needs to be updated
func (r *HeadlessServiceSubReconciler) needsUpdate(existing, desired *corev1.Service) bool {
	// Check selector equality
	if !mapsEqual(existing.Spec.Selector, desired.Spec.Selector) {
		return true
	}
	if !portsEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return true
	}
	if existing.Spec.PublishNotReadyAddresses != desired.Spec.PublishNotReadyAddresses {
		return true
	}
	// Check labels
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}
	return false
}

// mapsEqual checks if two string maps are equal
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// portsEqual checks if two port slices are equal
func portsEqual(a, b []corev1.ServicePort) bool {
	if len(a) != len(b) {
		return false
	}
	for i, port := range a {
		if port.Name != b[i].Name || port.Port != b[i].Port || port.Protocol != b[i].Protocol {
			return false
		}
	}
	return true
}
