// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"sync"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// FakeOutcome records the canned response for a single Image string.
type FakeOutcome struct {
	Digest              string
	LayerHashesVerified bool
	Err                 error
}

// FakeValidator drives unit/integration tests with deterministic outcomes.
// Lookup is by image reference; unknown images return ErrUnreachable.
type FakeValidator struct {
	mu       sync.Mutex
	Outcomes map[string]FakeOutcome
}

// NewFakeValidator constructs an empty FakeValidator.
func NewFakeValidator() *FakeValidator {
	return &FakeValidator{Outcomes: map[string]FakeOutcome{}}
}

// Set installs (or replaces) the outcome for an image reference.
func (f *FakeValidator) Set(image string, o FakeOutcome) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Outcomes[image] = o
}

// Validate implements Validator.
func (f *FakeValidator) Validate(
	_ context.Context,
	_ *operatorv1alpha1.UpgradeComponentSpec,
	reg *operatorv1alpha1.UpgradeRegistrySpec,
) (digest string, layerHashesVerified bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if reg == nil {
		return "", false, ErrUnreachable
	}
	o, ok := f.Outcomes[reg.Image]
	if !ok {
		return "", false, ErrUnreachable
	}
	return o.Digest, o.LayerHashesVerified, o.Err
}
