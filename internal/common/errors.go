// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"github.com/joomcode/errorx"

	"github.com/hashgraph/solo-operator/internal/errx"
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

// WarnOnConflict suppresses 409 Conflict errors since controller-runtime automatically requeues.
func WarnOnConflict(ctx context.Context, err error) error {
	if apierrors.IsConflict(err) {
		Warn(ctx, ReasonConflict).Err(err).Msg("Optimistic concurrency conflict on update; will retry")
		return nil
	}
	return err
}
