// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// RegistryState is the outcome of checking one registry entry.
type RegistryState string

const (
	StateValid        RegistryState = "Valid"
	StateUnreachable  RegistryState = "Unreachable"
	StateHashMismatch RegistryState = "HashMismatch"
	StateNoHashes     RegistryState = "NoHashes"
	StateDenied       RegistryState = "Denied"
)

type RegistryResult struct {
	Image  string
	Digest operatorv1alpha1.Sha256Digest
	State  RegistryState
	// true when expected layer hashes were configured and matched the image
	LayerHashesVerified bool
	// recorded, never propagated, so one failure can't abort sibling checks
	Err error
}

type ImageChecker interface {
	Check(ctx context.Context, spec operatorv1alpha1.UpgradeRegistrySpec, deterministic operatorv1alpha1.UpgradeDeterministicSpec) RegistryResult
}
