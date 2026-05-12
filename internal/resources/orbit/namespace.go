// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// missingNamespaceRequeue is how long we wait before re-checking a namespace
// that hasn't been created out-of-band yet.
const missingNamespaceRequeue = 10 * time.Second

// ErrNamespaceConflict is returned when the target namespace is already claimed
// by a different Orbit (per the operator.solo.hedera.com/orbit label).
var ErrNamespaceConflict = errors.New("namespace already claimed by another Orbit")

type NamespaceSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.Orbit]
}

func NewNamespaceSubReconciler(c client.Client, scheme *runtime.Scheme) *NamespaceSubReconciler {
	return &NamespaceSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.Orbit](c, scheme, "OrbitNamespace"),
	}
}

func (r *NamespaceSubReconciler) Reconcile(ctx context.Context, orbit *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	existing := &corev1.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: orbit.Spec.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if !orbit.Spec.CreateNamespace {
			logger.Info("Waiting for namespace to be created out-of-band",
				"namespace", orbit.Spec.Namespace)
			return ctrl.Result{RequeueAfter: missingNamespaceRequeue}, nil
		}
		logger.Info("Creating namespace", "namespace", orbit.Spec.Namespace)
		// Cluster-scoped owner refs aren't valid; we clean the namespace up via the finalizer.
		if err := r.Client.Create(ctx, r.buildNamespace(orbit)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if owner, ok := existing.Labels[LabelOrbit]; ok && owner != orbit.Name {
		return ctrl.Result{}, fmt.Errorf("%w: namespace %q claimed by Orbit %q",
			ErrNamespaceConflict, orbit.Spec.Namespace, owner)
	}

	desiredLabels := StandardLabels(orbit)
	if !labelsMatch(existing.Labels, desiredLabels) {
		logger.V(1).Info("Updating namespace labels", "namespace", orbit.Spec.Namespace)
		if existing.Labels == nil {
			existing.Labels = make(map[string]string)
		}
		for k, v := range desiredLabels {
			existing.Labels[k] = v
		}
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NamespaceSubReconciler) buildNamespace(orbit *operatorv1alpha1.Orbit) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   orbit.Spec.Namespace,
			Labels: StandardLabels(orbit),
		},
	}
}

func labelsMatch(existing, desired map[string]string) bool {
	for k, v := range desired {
		if existing[k] != v {
			return false
		}
	}
	return true
}
