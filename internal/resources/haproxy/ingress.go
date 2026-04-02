// SPDX-License-Identifier: Apache-2.0

package haproxy

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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

// IngressSubReconciler manages the Ingress for an HAProxyCapsule when ServiceType is Ingress
type IngressSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.HAProxyCapsule]
}

// NewIngressSubReconciler creates a new IngressSubReconciler
func NewIngressSubReconciler(c client.Client, scheme *runtime.Scheme) *IngressSubReconciler {
	return &IngressSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.HAProxyCapsule](c, scheme, "Ingress"),
	}
}

func (r *IngressSubReconciler) Reconcile(ctx context.Context, haproxy *operatorv1alpha1.HAProxyCapsule) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildIngress(haproxy)

	existing := &networkingv1.Ingress{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: haproxy.Name, Namespace: haproxy.Namespace}, existing)
	notFound := apierrors.IsNotFound(err)
	if err != nil && !notFound {
		return ctrl.Result{}, err
	}

	if desired == nil {
		if !notFound {
			if !metav1.IsControlledBy(existing, haproxy) {
				return ctrl.Result{}, nil
			}
			logger.Info("Deleting HAProxy ingress", "name", existing.Name, "namespace", existing.Namespace)
			if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if err := controllerutil.SetControllerReference(haproxy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if notFound {
		logger.Info("Creating HAProxy ingress", "name", desired.Name, "namespace", desired.Namespace)
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if r.needsUpdate(existing, desired) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		logger.V(1).Info("Updating HAProxy ingress", "name", desired.Name, "namespace", desired.Namespace)
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *IngressSubReconciler) needsUpdate(existing, desired *networkingv1.Ingress) bool {
	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return true
	}
	if len(existing.Labels) != len(desired.Labels) {
		return true
	}
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}
	return false
}

// buildIngress builds the desired Ingress from the IngressSpec, or returns nil when not desired.
func (r *IngressSubReconciler) buildIngress(haproxy *operatorv1alpha1.HAProxyCapsule) *networkingv1.Ingress {
	if haproxy.Spec.ServiceType != operatorv1alpha1.ProxyServiceTypeIngress || haproxy.Spec.IngressSpec == nil {
		return nil
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      haproxy.Name,
			Namespace: haproxy.Namespace,
			Labels:    ResourceLabels(haproxy),
		},
		Spec: *haproxy.Spec.IngressSpec,
	}
}
