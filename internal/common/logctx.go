// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type logFieldsKey struct{}

// LogFields are the reconcile-scoped fields stamped onto every log line within a
// single Reconcile. The "name"/"namespace" keys are reserved for the primary CR;
// log owned objects under a distinct key (e.g. "resource", "configMap").
type LogFields struct {
	Controller  string
	Kind        string
	Namespace   string
	Name        string
	ReconcileID string
	OperationID string
}

// WithLogFields returns a context carrying f, to be read back by LoggerFrom.
func WithLogFields(ctx context.Context, f LogFields) context.Context {
	return context.WithValue(ctx, logFieldsKey{}, f)
}

func logFieldsFrom(ctx context.Context) (LogFields, bool) {
	f, ok := ctx.Value(logFieldsKey{}).(LogFields)
	return f, ok
}

// OperationIDer lets ReconcileContext stamp operation_id automatically for CRs
// that carry a spec.operationId, without callers knowing which ones do.
type OperationIDer interface {
	GetOperationID() string
}

// ReconcileContext stamps the reconcile-scoped log fields onto ctx, to be called
// at the top of each top-level Reconcile. reconcileID comes from the context
// controller-runtime populated; operation_id only when obj is an OperationIDer.
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
