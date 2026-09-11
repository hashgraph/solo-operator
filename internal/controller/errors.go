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

	// errImageDigestRequired: the Orbit sets requireDigestOnDeploy but one or more of the
	// ConsensusCapsule's running images (the consensus node or an enabled sidecar/uploader)
	// is not digest-pinned. Terminal — the operator must set an explicit
	// softwareVersion.digest on each (or clear the Orbit policy).
	errImageDigestRequired = errNS.NewType("image_digest_required")

	// errImageRequired: a ConsensusCapsule has a running container (the consensus node or an
	// enabled sidecar/uploader) with no declared image. The operator provides no built-in
	// default images. Terminal — the operator must set softwareVersion on each container.
	errImageRequired = errNS.NewType("image_required")

	// errImageSourceInvalid: a container's SoftwareVersionSource is malformed — no
	// candidate repositories, or no usable per-platform layer hashes. A source must
	// carry layer hashes (verification is mandatory). Terminal — the operator must fix
	// the source or use a single softwareVersion instead.
	errImageSourceInvalid = errNS.NewType("image_source_invalid")
)

const (
	ReasonStatusUpdateFailed  errx.Reason = "StatusUpdateFailed"
	ReasonStatusInitFailed    errx.Reason = "StatusInitFailed"
	ReasonFinalizerFailed     errx.Reason = "FinalizerUpdateFailed"
	ReasonListFailed          errx.Reason = "ListFailed"
	ReasonGetFailed           errx.Reason = "GetFailed"
	ReasonPatchFailed         errx.Reason = "PatchFailed"
	ReasonScaleFailed         errx.Reason = "ScaleFailed"
	ReasonNoValidRegistry     errx.Reason = "NoValidRegistry"
	ReasonSourceUnpinned      errx.Reason = "SelectedSourceUnpinned"
	ReasonImageDigestRequired errx.Reason = "ImageDigestRequired"
	ReasonImageRequired       errx.Reason = "ImageRequired"
	ReasonImageSourceInvalid  errx.Reason = "ImageSourceInvalid"
	ReasonImageSourceResolved errx.Reason = "ImageSourceResolved"
	ReasonSourceHashFailed    errx.Reason = "SourceHashFailed"
	ReasonManifestInvalid     errx.Reason = "ManifestInvalid"
	ReasonDuplicateSibling    errx.Reason = "DuplicateSibling"
	ReasonOrbitNotFound       errx.Reason = "OrbitNotFound"
	ReasonConfigMapUpsert     errx.Reason = "ConfigMapUpsertFailed"

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
