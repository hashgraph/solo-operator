// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// All errors here are terminal — the operator must fix the offending content.
var (
	errNS = errorx.NewNamespace("common")

	errValidation  = errNS.NewType("validation_failed")
	errCertInvalid = errNS.NewType("cert_invalid")
)

const (
	ReasonContentEmpty errx.Reason = "ContentEmpty"
	ReasonNotKeyValue  errx.Reason = "NotKeyValueFormat"
	ReasonInvalidJSON  errx.Reason = "InvalidJSON"
	ReasonNoPEMBlock   errx.Reason = "NoPEMBlock"
	ReasonInvalidCert  errx.Reason = "InvalidCertificate"
)

// WarnOnConflict logs a warning on 409 Conflict but returns the error so the
// caller (or its parent reconciler) can requeue. Swallowing the error to nil
// would prevent requeue: controller-runtime only requeues on a non-nil error
// or an explicit Result.Requeue.
func WarnOnConflict(ctx context.Context, err error) error {
	if apierrors.IsConflict(err) {
		Warn(ctx, ReasonConflict).Err(err).Msg("Optimistic concurrency conflict on update; will retry")
	}
	return err
}
