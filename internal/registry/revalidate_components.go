// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"math/rand/v2"
	"time"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

type FailoverEvent func(name operatorv1alpha1.ComponentName, result FailoverResult)
type InvalidEvent func(name operatorv1alpha1.ComponentName, err error)

func RevalidateComponents(
	ctx context.Context,
	checker ImageChecker,
	images operatorv1alpha1.UpgradeImagesSpec,
	selected operatorv1alpha1.SelectedSources,
	timeout time.Duration,
	rng *rand.Rand,
	strategyName string,
	skip map[operatorv1alpha1.ComponentName]bool,
	onFailover FailoverEvent,
	onInvalid InvalidEvent,
) (updated *operatorv1alpha1.SelectedSources, allSucceeded bool) {
	updated = &operatorv1alpha1.SelectedSources{}
	allSucceeded = true

	for _, name := range operatorv1alpha1.AllUpgradeComponents {
		if skip[name] {
			continue
		}
		comp, present := images.Component(name)
		if !present || (comp.Enabled != nil && !*comp.Enabled) {
			continue
		}
		current := selected.Get(name)
		if current == nil {
			common.Info(ctx, ReasonNoSelectedSource).Str("component", string(name)).Msg("Skipping component: no selectedSource in spec")
			continue
		}

		result, err := RevalidateAndFailover(ctx, checker, comp, *current, timeout, rng, strategyName)
		if err != nil {
			common.Warn(ctx, ReasonComponentInvalid).Str("component", string(name)).Err(err).Msg("Found no valid registry for component")
			if onInvalid != nil {
				onInvalid(name, err)
			}
			allSucceeded = false
			continue
		}
		if result.DidFailover && onFailover != nil {
			onFailover(name, result)
		}
		src := result.Selected
		updated.Set(name, &src)
	}
	return updated, allSucceeded
}
