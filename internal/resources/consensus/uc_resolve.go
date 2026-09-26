// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"

	"github.com/automa-saga/errx"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/uc"
)

// ResolveUCMode returns the UC deployment mode for a capsule by reading the Orbit's
// resolved provisioner-daemon setting. The default (disabled) is cluster-only. When
// enabled, a host-level provisioner daemon handles the execute phase
// so the UC runs in mainnet mode. An empty Orbit reference is a terminal error when UC is
// enabled because UC_ORBIT is required. NotFound is treated as cluster-only; other errors
// are propagated so the reconciler can requeue.
func ResolveUCMode(ctx context.Context, c client.Client, capsule *operatorv1alpha1.ConsensusCapsule) (uc.Mode, error) {
	if capsule.Spec.Orbit == "" {
		return uc.ModeClusterOnly, errx.WithHints(
			errx.WithReason(errConfigInvalid.New("spec.orbit is empty"), ReasonOrbitRequired),
			"Set spec.orbit to the name of the Orbit CR that owns this ConsensusCapsule",
		)
	}

	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Spec.Orbit}, orbit); err != nil {
		if k8serrors.IsNotFound(err) {
			common.Debug(ctx, ReasonUCModeResolved).Str("orbit", capsule.Spec.Orbit).Msg("Orbit not found, defaulting UC mode to cluster-only")
			return uc.ModeClusterOnly, nil
		}
		return uc.ModeClusterOnly, errx.WithReason(errConfigInvalid.Wrap(err, "get Orbit %s", capsule.Spec.Orbit), ReasonOrbitRequired)
	}

	if orbit.Spec.ProvisionerDaemonEnabled {
		return uc.ModeMainnet, nil
	}
	return uc.ModeClusterOnly, nil
}

// ResolveNodeNameZeroBased returns the Orbit's resolved nodeNameZeroBased feature gate for
// a capsule. When the Orbit is unset or not found the gate defaults to false (node<id+1>),
// matching the pre-feature-gate behavior.
func ResolveNodeNameZeroBased(ctx context.Context, c client.Client, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	if capsule.Spec.Orbit == "" {
		return false, nil
	}
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Spec.Orbit}, orbit); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, errx.WithReason(errConfigInvalid.Wrap(err, "get Orbit %s", capsule.Spec.Orbit), ReasonOrbitRequired)
	}
	return orbit.IsNodeNameZeroBased(), nil
}
