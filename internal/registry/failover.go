// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

type FailoverResult struct {
	Selected    operatorv1alpha1.SelectedSource
	DidFailover bool
	FailedImage string
}

func RevalidateAndFailover(
	ctx context.Context,
	checker ImageChecker,
	comp operatorv1alpha1.UpgradeComponentSpec,
	current operatorv1alpha1.SelectedSource,
	timeout time.Duration,
	rng *rand.Rand,
	strategyName string,
) (FailoverResult, error) {
	spec := operatorv1alpha1.UpgradeRegistrySpec{Image: current.Image}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res := checker.Check(cctx, spec, comp.Deterministic)
	if res.State == StateValid {
		return FailoverResult{Selected: current}, nil
	}

	strategy := NewStrategy(strategyName, rng)
	results := CheckAll(ctx, checker, comp, timeout, strategy)

	winner, ok := SelectValid(results, rng)
	if !ok {
		return FailoverResult{}, fmt.Errorf("no valid registry available for component (original %s unreachable)", current.Image)
	}

	return FailoverResult{
		Selected: operatorv1alpha1.SelectedSource{
			Image:               winner.Image,
			Digest:              winner.Digest,
			LayerHashesVerified: winner.LayerHashesVerified,
		},
		DidFailover: true,
		FailedImage: current.Image,
	}, nil
}
