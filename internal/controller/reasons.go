// SPDX-License-Identifier: Apache-2.0

package controller

import "github.com/automa-saga/errx"

// Log-event reason codes for internal/controller. Generic K8s lifecycle codes
// live in internal/common; error/condition reasons live in errors.go.
const (
	// Reconcile lifecycle.
	ReasonReconcileStart   errx.Reason = "ReconcileStart"
	ReasonReconcileFinish  errx.Reason = "ReconcileFinish"
	ReasonResourceNotFound errx.Reason = "ResourceNotFound"
	ReasonDeleting         errx.Reason = "Deleting"

	// ConsensusCapsule.
	ReasonCheckpointUpdateFailed errx.Reason = "CheckpointUpdateFailed"
	ReasonS6ProbeFailed          errx.Reason = "S6ProbeFailed"

	// HelmCapsule.
	ReasonSkippingUninstall            errx.Reason = "SkippingUninstall"
	ReasonFailureConditionRecordFailed errx.Reason = "FailureConditionRecordFailed"

	// NetworkGenesis.
	ReasonStatusUpdateAfterErrorFailed errx.Reason = "StatusUpdateAfterErrorFailed"

	// NetworkUpgrade (execute/freeze/prepare/abort).
	ReasonWaitingForPrepare          errx.Reason = "WaitingForPrepare"
	ReasonWaitingForFreeze           errx.Reason = "WaitingForFreeze"
	ReasonCapsuleNotFound            errx.Reason = "CapsuleNotFound"
	ReasonUnhandledPhase             errx.Reason = "UnhandledPhase"
	ReasonWaitingForConfigCRsApplied errx.Reason = "WaitingForConfigCRsApplied"
	ReasonAbortAfterFreeze           errx.Reason = "AbortAfterFreeze"
	ReasonPhaseTransition            errx.Reason = "PhaseTransition"

	// NetworkUpgradePrepare registry audit.
	ReasonRegistryStrategy        errx.Reason = "RegistryStrategy"
	ReasonRegistryUnreachable     errx.Reason = "RegistryUnreachable"
	ReasonRegistryHashMismatch    errx.Reason = "RegistryHashMismatch"
	ReasonRegistryNoHashes        errx.Reason = "RegistryNoHashes"
	ReasonRegistryUnexpectedState errx.Reason = "RegistryUnexpectedState"

	// Orbit.
	ReasonOrbitLookupFailed         errx.Reason = "OrbitLookupFailed"
	ReasonWaitingForCapsuleDeletion errx.Reason = "WaitingForCapsuleDeletion"
)
