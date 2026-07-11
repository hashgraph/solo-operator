// SPDX-License-Identifier: Apache-2.0

package uc

import "github.com/hashgraph/solo-operator/internal/errx"

// Reason codes for the UC sidecar — the single source of truth for the
// `reason=` identifier carried by both errors (via errx.WithReason) and
// structured log lines (via logx ...Str("reason", ReasonX.String())). Keeping
// them as typed constants makes the vocabulary enumerable here and typo-safe at
// call sites. Codes are PascalCase; names are grouped by subsystem.
const (
	// Lifecycle / informational (not failures).
	ReasonUCStarted                      errx.Reason = "UCStarted"
	ReasonMarkerMonitorStarted           errx.Reason = "MarkerMonitorStarted"
	ReasonMarkerMonitorStopped           errx.Reason = "MarkerMonitorStopped"
	ReasonProvisionerProxyMonitorStarted errx.Reason = "ProvisionerProxyMonitorStarted"
	ReasonProvisionerProxyMonitorStopped errx.Reason = "ProvisionerProxyMonitorStopped"
	ReasonPollIntervalChanged            errx.Reason = "PollIntervalChanged"
	ReasonMarkerHandlerPending           errx.Reason = "MarkerHandlerPending"
	ReasonCRDelivered                    errx.Reason = "CRDelivered"
	ReasonCRExists                       errx.Reason = "CRExists"
	ReasonCRFailed                       errx.Reason = "CRFailed"
	ReasonUCCacheSeeded                  errx.Reason = "UCCacheSeeded"
	ReasonReadyForProvisionerDaemon      errx.Reason = "ReadyForProvisionerDaemon"
	ReasonPendingNodeUpgrade             errx.Reason = "PendingNodeUpgrade"

	// Config.
	ReasonConfigInvalid errx.Reason = "ConfigInvalid"

	// Manifest / deployment package.
	ReasonManifestInvalid    errx.Reason = "ManifestInvalid"
	ReasonManifestReadFailed errx.Reason = "ManifestReadFailed"

	// CR construction.
	ReasonCRBuildFailed errx.Reason = "CRBuildFailed"

	// Persist-before-send (disk / --cr-persist-path).
	ReasonMarkerStatFailed        errx.Reason = "MarkerStatFailed"
	ReasonWorkDirCreateFailed     errx.Reason = "WorkDirCreateFailed"
	ReasonCRPersistWriteFailed    errx.Reason = "CRPersistWriteFailed"
	ReasonCRPersistDeleteFailed   errx.Reason = "CRPersistDeleteFailed"
	ReasonCRPersistListFailed     errx.Reason = "CRPersistListFailed"
	ReasonCRPersistReadFailed     errx.Reason = "CRPersistReadFailed"
	ReasonCRPersistCorrupt        errx.Reason = "CRPersistCorrupt"
	ReasonCRPersistCleanupFailed  errx.Reason = "CRPersistCleanupFailed"
	ReasonEventLogPruneFailed     errx.Reason = "EventLogPruneFailed"
	ReasonUpgradePathUnreadable         errx.Reason = "UpgradePathUnreadable"
	ReasonPreviousUpgradePathUnwritable errx.Reason = "PreviousUpgradePathUnwritable"
	ReasonCRPersistPathUnwritable       errx.Reason = "CRPersistPathUnwritable"

	// Kubernetes API.
	ReasonInClusterConfigFailed errx.Reason = "InClusterConfigFailed"
	ReasonKubeconfigLoadFailed  errx.Reason = "KubeconfigLoadFailed"
	ReasonDynamicClientFailed   errx.Reason = "DynamicClientFailed"
	ReasonCRCreateFailed        errx.Reason = "CRCreateFailed"
	ReasonDedupQueryFailed      errx.Reason = "DedupQueryFailed"
	ReasonCacheSeedFailed       errx.Reason = "CacheSeedFailed"
	ReasonPrepareListFailed     errx.Reason = "PrepareListFailed"
	ReasonExecuteListFailed      errx.Reason = "ExecuteListFailed"
	ReasonFreezeAbortListFailed  errx.Reason = "FreezeAbortListFailed"
	ReasonExecutePatchFailed    errx.Reason = "ExecutePatchFailed"
	ReasonWatchFailed           errx.Reason = "WatchFailed"
	ReasonWatchErrorEvent       errx.Reason = "WatchErrorEvent"

	// Marker handling.
	ReasonMarkerHandlerFailed  errx.Reason = "MarkerHandlerFailed"
	ReasonAbortWithoutPrepare  errx.Reason = "AbortWithoutPrepare"
	ReasonFreezeWithoutPrepare errx.Reason = "FreezeWithoutPrepare"
	ReasonPrepareFailed        errx.Reason = "PrepareFailed"
	ReasonStaleMarker          errx.Reason = "StaleMarker"
	ReasonOperationIDExhausted errx.Reason = "OperationIDExhausted"

	// Startup recovery / cache.
	ReasonUCCacheSeedFailed      errx.Reason = "UCCacheSeedFailed"
	ReasonUCStartupRecoveryError errx.Reason = "UCStartupRecoveryError"
	ReasonCRResent               errx.Reason = "CRResent"
	ReasonCRResentFailed         errx.Reason = "CRResentFailed"
	ReasonArtifactCleanup        errx.Reason = "ArtifactCleanup"
	ReasonArtifactCleanupFailed  errx.Reason = "ArtifactCleanupFailed"

	// Control socket client.
	ReasonControlRequestFailed errx.Reason = "ControlRequestFailed"
	ReasonControlDialFailed    errx.Reason = "ControlDialFailed"
	ReasonControlReadFailed    errx.Reason = "ControlReadFailed"
	ReasonControlBadStatus     errx.Reason = "ControlBadStatus"

	// Provisioner-proxy monitor.
	ReasonProvisionerProxyAuthError          errx.Reason = "ProvisionerProxyAuthError"
	ReasonProvisionerProxyBusy               errx.Reason = "ProvisionerProxyBusy"
	ReasonProvisionerProxyListError          errx.Reason = "ProvisionerProxyListError"
	ReasonProvisionerProxyMissingOperationID errx.Reason = "ProvisionerProxyMissingOperationID"
	ReasonProvisionerProxyTransitionFailed   errx.Reason = "ProvisionerProxyTransitionFailed"
	ReasonProvisionerProxyTransitionPanic    errx.Reason = "ProvisionerProxyTransitionPanic"
	ReasonProvisionerProxyWatchError         errx.Reason = "ProvisionerProxyWatchError"
	ReasonProvisionerProxyWatchPanic         errx.Reason = "ProvisionerProxyWatchPanic"
	ReasonUpgradePackageRotated              errx.Reason = "UpgradePackageRotated"
	ReasonUpgradePackageDiscarded            errx.Reason = "UpgradePackageDiscarded"
	ReasonUpgradePackageRotateFailed         errx.Reason = "UpgradePackageRotateFailed"

	// systemd notify.
	ReasonSdNotifyReadyFailed    errx.Reason = "SdNotifyReadyFailed"
	ReasonSdNotifyStoppingFailed errx.Reason = "SdNotifyStoppingFailed"
)
