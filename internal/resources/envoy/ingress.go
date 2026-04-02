// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// IngressSubReconciler manages the Ingress for an EnvoyProxy when ServiceType is Ingress
type IngressSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.EnvoyProxy]
}

// NewIngressSubReconciler creates a new IngressSubReconciler
func NewIngressSubReconciler(c client.Client, scheme *runtime.Scheme) *IngressSubReconciler {
	return &IngressSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.EnvoyProxy](c, scheme, "Ingress"),
	}
}

// ShouldReconcile returns true only when ServiceType is Ingress
func (r *IngressSubReconciler) ShouldReconcile(_ context.Context, envoy *operatorv1alpha1.EnvoyProxy) bool {
	return envoy.Spec.ServiceType == operatorv1alpha1.ProxyServiceTypeIngress
}

// Reconcile ensures the Ingress exists and is up to date
func (r *IngressSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildIngress(envoy)
	if desired == nil {
		return ctrl.Result{}, nil
	}

	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &networkingv1.Ingress{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy ingress", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update spec if changed
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	logger.V(1).Info("Updating Envoy ingress", "name", desired.Name, "namespace", desired.Namespace)
	if err := r.Client.Update(ctx, existing); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// buildIngress builds the desired Ingress from the IngressSpec
func (r *IngressSubReconciler) buildIngress(envoy *operatorv1alpha1.EnvoyProxy) *networkingv1.Ingress {
	if envoy.Spec.IngressSpec == nil {
		return nil
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    CommonLabels(envoy),
		},
		Spec: *envoy.Spec.IngressSpec,
	}
}
