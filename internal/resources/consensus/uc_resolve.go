// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/uc"
)

// ResolveUCMode returns the UC deployment mode for a capsule by reading
// Orbit.Spec.Consensus.ProvisionerDaemonEnabled. The default (ProvisionerDaemonEnabled=false)
// is cluster-only. When true, a host-level provisioner daemon handles the execute phase
// so the UC runs in mainnet mode. NotFound is treated as cluster-only; other errors are
// propagated so the reconciler can requeue.
func ResolveUCMode(ctx context.Context, c client.Client, capsule *operatorv1alpha1.ConsensusCapsule) (uc.Mode, error) {
	logger := log.FromContext(ctx)

	if capsule.Spec.Orbit == "" {
		return uc.ModeClusterOnly, nil
	}

	orbit := &operatorv1alpha1.Orbit{}
	if err := c.Get(ctx, client.ObjectKey{Name: capsule.Spec.Orbit}, orbit); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Orbit not found, defaulting UC mode to cluster-only", "orbit", capsule.Spec.Orbit)
			return uc.ModeClusterOnly, nil
		}
		return uc.ModeClusterOnly, err
	}

	if orbit.Spec.Consensus.ProvisionerDaemonEnabled {
		return uc.ModeMainnet, nil
	}
	return uc.ModeClusterOnly, nil
}
