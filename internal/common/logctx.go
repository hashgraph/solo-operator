// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type logFieldsKey struct{}

// LogFields are the reconcile-scoped fields stamped onto every log event
// produced via LoggerFrom(ctx) within a single Reconcile. OperationID is
// optional — only the NetworkUpgrade/NetworkGenesis CRs carry a spec.operationId.
//
// The "name"/"namespace" keys are reserved for the primary CR. When logging an
// owned/related object, use a distinct key (e.g. "resource", "configMap") so it
// does not clobber the inherited primary "name".
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

// OperationIDer is implemented by CRs that carry a spec.operationId (the
// NetworkUpgrade/NetworkGenesis family). ReconcileContext stamps operation_id
// automatically when obj implements it, so callers need not know which CRs have one.
type OperationIDer interface {
	GetOperationID() string
}

// ReconcileContext stamps the reconcile-scoped log fields onto ctx in one call,
// to be invoked at the top of each top-level Reconcile. reconcileID is read from
// the context controller-runtime populated, via the public ReconcileIDFromContext.
// operation_id is populated only when obj implements OperationIDer.
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
