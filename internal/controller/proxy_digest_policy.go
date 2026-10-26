// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"strings"

	"github.com/automa-saga/errx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// validateProxyDigestPolicy enforces the Orbit's requireDigestOnDeploy on a proxy image.
func validateProxyDigestPolicy(ctx context.Context, c client.Client, orbitName, kind, name, image string) error {
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errx.WithReason(errK8sAPI.Wrap(err, "get Orbit %s for digest policy", orbitName), ReasonGetFailed)
	}
	if !orbit.Spec.RequireDigestOnDeploy || strings.Contains(image, "@sha256:") {
		return nil
	}
	return reconcile.TerminalError(errx.WithHints(
		errx.WithReason(
			errImageDigestRequired.New("Orbit %q requires digest-pinned images on deploy, but %s %q uses unpinned image %q", orbitName, kind, name, image),
			ReasonImageDigestRequired),
		"Set spec.image to a reference ending in @sha256:<digest>, or set the Orbit's spec.requireDigestOnDeploy=false.",
	))
}
