// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"github.com/automa-saga/errx"
	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("controller")

	// Transient: controller-runtime requeues with backoff.
	errK8sAPI = errNS.NewType("k8s_api_failed", errorx.Temporary())
	// Terminal: a requeue cannot fix it (the reconciler records a Failed status).
	errReconcile = errNS.NewType("reconcile_failed")

	// errUCSidecarRequired: the ConsensusCapsule does not enable the mandatory UC
	// sidecar. The UC is the sole producer of status.platformStatus, so without it
	// the capsule phase can never advance past Running. Terminal — the operator
	// must enable the sidecar in the spec.
	errUCSidecarRequired = errNS.NewType("uc_sidecar_required")
)

const (
	ReasonStatusUpdateFailed errx.Reason = "StatusUpdateFailed"
	ReasonStatusInitFailed   errx.Reason = "StatusInitFailed"
	ReasonFinalizerFailed    errx.Reason = "FinalizerUpdateFailed"
	ReasonListFailed         errx.Reason = "ListFailed"
	ReasonGetFailed          errx.Reason = "GetFailed"
	ReasonPatchFailed        errx.Reason = "PatchFailed"
	ReasonScaleFailed        errx.Reason = "ScaleFailed"
	ReasonNoValidRegistry    errx.Reason = "NoValidRegistry"
	ReasonManifestInvalid    errx.Reason = "ManifestInvalid"
	ReasonDuplicateSibling   errx.Reason = "DuplicateSibling"
	ReasonOrbitNotFound      errx.Reason = "OrbitNotFound"
	ReasonConfigMapUpsert    errx.Reason = "ConfigMapUpsertFailed"

	// Stray-literal condition reasons promoted from inline strings.
	ReasonReconcileSuccess  errx.Reason = "ReconcileSuccess"
	ReasonGenesisCompleted  errx.Reason = "GenesisCompleted"
	ReasonGenesisFailed     errx.Reason = "GenesisFailed"
	ReasonSpecMutated       errx.Reason = "SpecMutationRejected"
	ReasonConfigMapNotFound errx.Reason = "ConfigMapNotFound"
	ReasonConfigMapFound    errx.Reason = "ConfigMapFound"
	ReasonPrefetchJobBuild  errx.Reason = "PrefetchJobBuildFailed"
	ReasonPrefetchJobCreate errx.Reason = "PrefetchJobCreateFailed"

	ReasonUCSidecarRequired errx.Reason = "UCSidecarRequired"
)

const statusRBACHint = "Confirm the operator has patch permission on the resource's /status"

// wrapStatusUpdate decorates a failed status write (verb e.g. "updating status",
// "advancing to PendingInfraUpgrade"). Transient — controller-runtime requeues.
func wrapStatusUpdate(err error, verb string) error {
	return errx.Decorate(errK8sAPI.Wrap(err, "%s", verb), ReasonStatusUpdateFailed, statusRBACHint)
}

// wrapStatusInit decorates a failed initial status write.
func wrapStatusInit(err error) error {
	return errx.Decorate(errK8sAPI.Wrap(err, "initializing status"), ReasonStatusInitFailed, statusRBACHint)
}
