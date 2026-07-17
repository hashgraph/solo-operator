// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RequestContext stamps name/namespace/reconcileID before the primary object is fetched;
// early-exit log lines get object identity without manual .Str chains.
func RequestContext(ctx context.Context, req ctrl.Request) context.Context {
	f := LogFields{
		Namespace:   req.Namespace,
		Name:        req.Name,
		ReconcileID: string(ctrlcontroller.ReconcileIDFromContext(ctx)),
	}
	return WithLogFields(ctx, f)
}

type logFieldsKey struct{}

type LogFields struct {
	Controller  string
	Kind        string
	Namespace   string
	Name        string
	ReconcileID string
	OperationID string
}

func WithLogFields(ctx context.Context, f LogFields) context.Context {
	return context.WithValue(ctx, logFieldsKey{}, f)
}

func logFieldsFrom(ctx context.Context) (LogFields, bool) {
	f, ok := ctx.Value(logFieldsKey{}).(LogFields)
	return f, ok
}

// OperationIDer is implemented by CRs that carry a spec.operationId.
type OperationIDer interface {
	GetOperationID() string
}

func ReconcileContext(ctx context.Context, controller, kind string, obj metav1.Object) context.Context {
	f := LogFields{
		Controller:  controller,
		Kind:        kind,
		Namespace:   obj.GetNamespace(),
		Name:        obj.GetName(),
		ReconcileID: string(ctrlcontroller.ReconcileIDFromContext(ctx)),
	}
	if o, ok := obj.(OperationIDer); ok {
		f.OperationID = o.GetOperationID()
	}
	return WithLogFields(ctx, f)
}
