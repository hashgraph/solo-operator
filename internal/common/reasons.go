// SPDX-License-Identifier: Apache-2.0

package common

import "github.com/automa-saga/errx"

// Generic owned-resource lifecycle reason codes, shared across every resource
// sub-reconciler. The controller/kind fields from ReconcileContext disambiguate
// the subsystem. Domain-specific reasons stay in their own package's reasons.go.
const (
	ReasonConfigMapCreated errx.Reason = "ConfigMapCreated"
	ReasonConfigMapUpdated errx.Reason = "ConfigMapUpdated"

	ReasonServiceCreated errx.Reason = "ServiceCreated"
	ReasonServiceUpdated errx.Reason = "ServiceUpdated"

	ReasonDeploymentCreated errx.Reason = "DeploymentCreated"
	ReasonDeploymentUpdated errx.Reason = "DeploymentUpdated"

	ReasonStatefulSetCreated errx.Reason = "StatefulSetCreated"
	ReasonStatefulSetUpdated errx.Reason = "StatefulSetUpdated"

	ReasonIngressCreated errx.Reason = "IngressCreated"
	ReasonIngressUpdated errx.Reason = "IngressUpdated"
	ReasonIngressDeleted errx.Reason = "IngressDeleted"

	ReasonPVCCreated errx.Reason = "PVCCreated"

	ReasonRoleCreated errx.Reason = "RoleCreated"
	ReasonRoleUpdated errx.Reason = "RoleUpdated"

	ReasonRoleBindingCreated errx.Reason = "RoleBindingCreated"
	ReasonRoleBindingUpdated errx.Reason = "RoleBindingUpdated"

	ReasonServiceAccountCreated errx.Reason = "ServiceAccountCreated"
	ReasonServiceAccountUpdated errx.Reason = "ServiceAccountUpdated"

	// ReasonInvalidContent is the fallback for a config reconciler's error log
	// when the wrapped error carries no reason of its own.
	ReasonInvalidContent errx.Reason = "InvalidContent"

	ReasonConflict          errx.Reason = "Conflict"
	ReasonOwnershipConflict errx.Reason = "OwnershipConflict"
)
