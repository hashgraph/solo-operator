// SPDX-License-Identifier: Apache-2.0

package uc

// Marker file names written by the consensus node into the marker directory.
// The UC sidecar polls for these and translates each into a NetworkUpgrade* CR.
// See HIP XXXX2 (network-upgrade), "UC Sidecar" §902.
const (
	// MarkerExecuteImmediate triggers construction of a NetworkUpgradePrepare CR.
	MarkerExecuteImmediate = "execute_immediate.mf"
	// MarkerFreezeScheduled triggers a NetworkUpgradeFreeze CR and switches the
	// poll loop to the fast (freeze-window) interval.
	MarkerFreezeScheduled = "freeze_scheduled.mf"
	// MarkerNowFrozen triggers a NetworkUpgradeExecute CR.
	MarkerNowFrozen = "now_frozen.mf"
	// MarkerFreezeAborted triggers a NetworkUpgradeFreezeAbort CR and reverts the
	// poll loop to the normal interval.
	MarkerFreezeAborted = "freeze_aborted.mf"
)

// AllMarkers is the set of marker files the poller recognises, in detection order.
var AllMarkers = []string{
	MarkerExecuteImmediate,
	MarkerFreezeScheduled,
	MarkerNowFrozen,
	MarkerFreezeAborted,
}
