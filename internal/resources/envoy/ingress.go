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

func (r *IngressSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildIngress(envoy)

	existing := &networkingv1.Ingress{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, existing)
	notFound := apierrors.IsNotFound(err)
	if err != nil && !notFound {
		return ctrl.Result{}, err
	}

	if desired == nil {
		if !notFound {
			logger.Info("Deleting Envoy ingress", "name", existing.Name, "namespace", existing.Namespace)
			if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if notFound {
		logger.Info("Creating Envoy ingress", "name", desired.Name, "namespace", desired.Namespace)
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	logger.V(1).Info("Updating Envoy ingress", "name", desired.Name, "namespace", desired.Namespace)
	if err := r.Client.Update(ctx, existing); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// buildIngress builds the desired Ingress from the IngressSpec, or returns nil when not desired.
func (r *IngressSubReconciler) buildIngress(envoy *operatorv1alpha1.EnvoyProxy) *networkingv1.Ingress {
	if envoy.Spec.ServiceType != operatorv1alpha1.ProxyServiceTypeIngress || envoy.Spec.IngressSpec == nil {
		return nil
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    ResourceLabels(envoy),
		},
		Spec: *envoy.Spec.IngressSpec,
	}
}
