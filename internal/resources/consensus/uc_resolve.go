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

// ResolveOrbit fetches the Orbit that owns a capsule. It returns (nil, nil) when the
// capsule has no Orbit reference or the Orbit is not found, letting callers apply a safe
// default; other Get errors are propagated so the reconciler can requeue. Fetch once per
// reconcile and derive every Orbit-scoped setting from the result — this avoids redundant
// reads and rules out two resolvers observing different Orbit states.
func ResolveOrbit(ctx context.Context, c client.Client, capsule *operatorv1alpha1.ConsensusCapsule) (*operatorv1alpha1.Orbit, error) {
	if capsule.Spec.Orbit == "" {
		return nil, nil
	}
	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Spec.Orbit}, orbit); err != nil {
		if k8serrors.IsNotFound(err) {
			common.Debug(ctx, ReasonOrbitNotFound).Str("orbit", capsule.Spec.Orbit).Msg("Orbit not found, applying defaults")
			return nil, nil
		}
		return nil, errx.WithReason(errConfigInvalid.Wrap(err, "get Orbit %s", capsule.Spec.Orbit), ReasonOrbitRequired)
	}
	return orbit, nil
}

// ResolveUCMode returns the UC deployment mode for a capsule. The default (provisioner
// daemon disabled) is cluster-only; when enabled, a host-level provisioner daemon handles
// the execute phase so the UC runs in mainnet mode.
func ResolveUCMode(ctx context.Context, c client.Client, capsule *operatorv1alpha1.ConsensusCapsule) (uc.Mode, error) {
	orbit, err := ResolveOrbit(ctx, c, capsule)
	if err != nil {
		return uc.ModeClusterOnly, err
	}
	return ucModeFromOrbit(capsule, orbit)
}

// ucModeFromOrbit derives the UC mode from an already-resolved Orbit (nil = empty ref or
// NotFound). Unlike the node-name gate, which has a safe default for a missing Orbit, UC
// requires UC_ORBIT: an empty spec.orbit is terminal, while a merely-not-found Orbit
// (orbit == nil with a non-empty ref) defaults to cluster-only.
func ucModeFromOrbit(capsule *operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit) (uc.Mode, error) {
	if capsule.Spec.Orbit == "" {
		return uc.ModeClusterOnly, errx.WithHints(
			errx.WithReason(errConfigInvalid.New("spec.orbit is empty"), ReasonOrbitRequired),
			"Set spec.orbit to the name of the Orbit CR that owns this ConsensusCapsule",
		)
	}
	if orbit != nil && orbit.Spec.Consensus != nil && orbit.Spec.Consensus.ProvisionerDaemonEnabled {
		return uc.ModeMainnet, nil
	}
	return uc.ModeClusterOnly, nil
}
