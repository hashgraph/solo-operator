// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/automa-saga/errx"

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
	comp operatorv1alpha1.ImageComponentSpec,
	current operatorv1alpha1.SelectedSource,
	timeout time.Duration,
	rng *rand.Rand,
	strategyName string,
) (FailoverResult, error) {
	spec := specForSource(comp, current)

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res := checker.Check(cctx, spec, comp.Deterministic)
	if res.State == StateValid {
		return FailoverResult{Selected: current}, nil
	}

	_, winner, ok := SelectComponent(ctx, checker, comp, timeout, rng, strategyName)
	if !ok {
		return FailoverResult{}, errx.WithHints(
			errx.WithReason(
				errNoRegistry.New("no valid registry available for component (original %s unreachable)", current.Image),
				ReasonNoValidRegistry,
			),
			"Confirm at least one registry in the component's failover set serves a valid image",
			"Check network connectivity and pull credentials to the configured registries",
		)
	}

	return FailoverResult{
		Selected:    SelectedSourceFromWinner(comp, winner),
		DidFailover: true,
		FailedImage: current.Image,
	}, nil
}

func specForSource(comp operatorv1alpha1.ImageComponentSpec, current operatorv1alpha1.SelectedSource) operatorv1alpha1.ImageRegistrySpec {
	if len(current.LayerHashes) > 0 {
		return operatorv1alpha1.ImageRegistrySpec{Image: current.Image, LayerHashes: current.LayerHashes}
	}
	return SpecForImage(comp, current.Image)
}

// SpecForImage returns the registry spec for image, or a bare spec if not found.
func SpecForImage(comp operatorv1alpha1.ImageComponentSpec, image string) operatorv1alpha1.ImageRegistrySpec {
	for _, r := range comp.Registries {
		if r.Image == image {
			return r
		}
	}
	return operatorv1alpha1.ImageRegistrySpec{Image: image}
}
