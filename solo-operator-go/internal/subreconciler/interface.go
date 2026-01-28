// SPDX-License-Identifier: Apache-2.0

package subreconciler

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SubReconciler defines the interface for dependent resource reconcilers.
// Each subreconciler is responsible for managing a specific dependent resource.
type SubReconciler[T client.Object] interface {
	// Name returns the name of this subreconciler for logging purposes
	Name() string

	// ShouldReconcile returns true if this subreconciler should run
	ShouldReconcile(ctx context.Context, primary T) bool

	// Reconcile performs the reconciliation for the dependent resource.
	// Returns a Result indicating whether to requeue and any error.
	Reconcile(ctx context.Context, primary T) (ctrl.Result, error)
}

// BaseSubReconciler provides common functionality for subreconcilers
type BaseSubReconciler[T client.Object] struct {
	Client client.Client
	Scheme *runtime.Scheme
	name   string
}

// NewBaseSubReconciler creates a new BaseSubReconciler
func NewBaseSubReconciler[T client.Object](c client.Client, scheme *runtime.Scheme, name string) *BaseSubReconciler[T] {
	return &BaseSubReconciler[T]{
		Client: c,
		Scheme: scheme,
		name:   name,
	}
}

// Name returns the name of this subreconciler
func (b *BaseSubReconciler[T]) Name() string {
	return b.name
}

// ShouldReconcile returns true by default; override in implementations
func (b *BaseSubReconciler[T]) ShouldReconcile(ctx context.Context, primary T) bool {
	return true
}

// SetOwnerReference sets the owner reference on the dependent resource
func (b *BaseSubReconciler[T]) SetOwnerReference(owner, dependent metav1.Object) error {
	ownerObj, ok := owner.(client.Object)
	if !ok {
		return nil
	}
	dependentObj, ok := dependent.(client.Object)
	if !ok {
		return nil
	}

	return controllerutil.SetControllerReference(ownerObj, dependentObj, b.Scheme)
}

// Chain represents a chain of subreconcilers to execute in order
type Chain[T client.Object] struct {
	subreconcilers []SubReconciler[T]
}

// NewChain creates a new Chain with the given subreconcilers
func NewChain[T client.Object](subreconcilers ...SubReconciler[T]) *Chain[T] {
	return &Chain[T]{
		subreconcilers: subreconcilers,
	}
}

// Reconcile executes all subreconcilers in order.
// Stops on the first error or requeue request.
func (c *Chain[T]) Reconcile(ctx context.Context, primary T) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	for _, sr := range c.subreconcilers {
		if !sr.ShouldReconcile(ctx, primary) {
			logger.V(1).Info("Skipping subreconciler", "name", sr.Name())
			continue
		}

		logger.V(1).Info("Running subreconciler", "name", sr.Name())
		result, err := sr.Reconcile(ctx, primary)
		if err != nil {
			logger.Error(err, "Subreconciler failed", "name", sr.Name())
			return result, err
		}
		if result.Requeue || result.RequeueAfter > 0 {
			logger.V(1).Info("Subreconciler requested requeue", "name", sr.Name())
			return result, nil
		}
	}

	return ctrl.Result{}, nil
}

// ResourceReconciler is a subreconciler that creates or updates a specific resource
type ResourceReconciler[T client.Object, R client.Object] struct {
	*BaseSubReconciler[T]

	// Builder builds the desired resource state
	Builder func(ctx context.Context, primary T) (R, error)

	// Updater updates an existing resource (optional)
	Updater func(ctx context.Context, primary T, existing R) error

	// ShouldReconcileFunc determines if this reconciler should run (optional)
	ShouldReconcileFunc func(ctx context.Context, primary T) bool
}

// NewResourceReconciler creates a new ResourceReconciler
func NewResourceReconciler[T client.Object, R client.Object](
	c client.Client,
	scheme *runtime.Scheme,
	name string,
	builder func(ctx context.Context, primary T) (R, error),
) *ResourceReconciler[T, R] {
	return &ResourceReconciler[T, R]{
		BaseSubReconciler: NewBaseSubReconciler[T](c, scheme, name),
		Builder:           builder,
	}
}

// WithUpdater sets the updater function
func (r *ResourceReconciler[T, R]) WithUpdater(updater func(ctx context.Context, primary T, existing R) error) *ResourceReconciler[T, R] {
	r.Updater = updater
	return r
}

// WithShouldReconcile sets the ShouldReconcile function
func (r *ResourceReconciler[T, R]) WithShouldReconcile(fn func(ctx context.Context, primary T) bool) *ResourceReconciler[T, R] {
	r.ShouldReconcileFunc = fn
	return r
}

// ShouldReconcile returns whether this reconciler should run
func (r *ResourceReconciler[T, R]) ShouldReconcile(ctx context.Context, primary T) bool {
	if r.ShouldReconcileFunc != nil {
		return r.ShouldReconcileFunc(ctx, primary)
	}
	return true
}

// Reconcile creates or updates the resource
func (r *ResourceReconciler[T, R]) Reconcile(ctx context.Context, primary T) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Build the desired resource
	desired, err := r.Builder(ctx, primary)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set owner reference
	if err := r.SetOwnerReference(primary, desired); err != nil {
		return ctrl.Result{}, err
	}

	// Try to get existing resource
	existing := desired.DeepCopyObject().(R)
	key := client.ObjectKeyFromObject(desired)

	err = r.Client.Get(ctx, key, existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Resource doesn't exist, create it
		logger.Info("Creating resource", "kind", desired.GetObjectKind().GroupVersionKind().Kind, "name", desired.GetName())
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Resource exists, update if needed
	if r.Updater != nil {
		if err := r.Updater(ctx, primary, existing); err != nil {
			return ctrl.Result{}, err
		}

		logger.V(1).Info("Updating resource", "kind", existing.GetObjectKind().GroupVersionKind().Kind, "name", existing.GetName())
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
