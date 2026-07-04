// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"time"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// Mode selects which deployment profile the UC runs under. It governs whether
// the provisioner-proxy monitor is active. See HIP XXXX2 §542 (Deployment Profiles).
type Mode string

const (
	// ModeMainnet routes the execute phase through the real solo-provisioner
	// daemon on the host. The UC does NOT advance the Execute CR phase; the
	// provisioner-proxy monitor is disabled.
	ModeMainnet Mode = "mainnet"
	// ModeClusterOnly (hashsphere / fully Kubernetes-managed) has no host-level
	// provisioner daemon. The UC fulfils the daemon role: the provisioner-proxy
	// monitor advances ReadyForProvisionerDaemon -> PendingNodeUpgrade directly.
	ModeClusterOnly Mode = "cluster-only"
)

// Default filesystem locations and intervals for the UC sidecar. These mirror
// the configuration parameters documented in HIP XXXX2 §919.
const (
	DefaultMarkerPath    = "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current"
	DefaultEventsPath    = "/opt/solo/weaver/uc/events"
	DefaultCRPersistPath = "/opt/solo/weaver/uc/crs"
	DefaultBackupPath    = "/opt/solo/weaver/uc/backup"
	DefaultSocketPath    = "/opt/solo/weaver/uc/uc.sock"

	// DefaultPollInterval is the normal marker-file polling cadence. It drops to
	// FreezePollInterval after freeze_scheduled.mf is seen, then reverts.
	DefaultPollInterval = 60 * time.Second
	// FreezePollInterval is the fast cadence used during the freeze window to
	// minimise now_frozen.mf detection latency.
	FreezePollInterval = 1 * time.Second
)

// Config holds the runtime configuration for the UC sidecar.
type Config struct {
	// Mode is the deployment profile (mainnet or cluster-only).
	Mode Mode
	// NodeID is the consensus node identity (a non-negative integer, e.g. 0, 1, 2).
	NodeID int
	// Orbit is the deployment scope the upgrade CRs target.
	Orbit string
	// Namespace is the Kubernetes namespace the UC watches and creates CRs in.
	Namespace string
	// KubeconfigPath is optional; empty means in-cluster config.
	KubeconfigPath string

	// MarkerPath is the directory polled for marker files.
	MarkerPath string
	// EventsPath is the JSONL event-log directory.
	EventsPath string
	// CRPersistPath is where constructed CRs are written before the K8s API send.
	CRPersistPath string
	// BackupPath is the pre-upgrade backup directory.
	BackupPath string
	// SocketPath is the Unix socket for the daemonkit control server.
	SocketPath string

	// PollInterval is the normal marker polling cadence.
	PollInterval time.Duration
	// FreezePollInterval is the freeze-window polling cadence.
	FreezePollInterval time.Duration
}

// DefaultConfig returns a Config pre-populated with the documented defaults.
func DefaultConfig() Config {
	return Config{
		Mode:               ModeMainnet,
		NodeID:             -1,
		MarkerPath:         DefaultMarkerPath,
		EventsPath:         DefaultEventsPath,
		CRPersistPath:      DefaultCRPersistPath,
		BackupPath:         DefaultBackupPath,
		SocketPath:         DefaultSocketPath,
		PollInterval:       DefaultPollInterval,
		FreezePollInterval: FreezePollInterval,
	}
}

// Validate checks that required fields are set and intervals are sane.
func (c Config) Validate() error {
	switch c.Mode {
	case ModeMainnet, ModeClusterOnly:
	default:
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("mode %q is invalid", c.Mode), ReasonConfigInvalid),
			"Set --mode to 'mainnet' or 'cluster-only'",
		)
	}
	if c.NodeID < 0 {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("node-id %d is invalid", c.NodeID), ReasonConfigInvalid),
			"Set --node-id to a non-negative integer",
		)
	}
	if c.Orbit == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("orbit is required"), ReasonConfigInvalid),
			"Set --orbit to the owning Orbit resource name",
		)
	}
	if c.Namespace == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("namespace is required"), ReasonConfigInvalid),
			"Set --namespace to the node's Kubernetes namespace",
		)
	}
	if c.MarkerPath == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("marker-path is required"), ReasonConfigInvalid),
			"Set --marker-path to the consensus node's upgrade-artifacts directory (e.g. data/upgrade/current)",
		)
	}
	if c.CRPersistPath == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("cr-persist-path is required"), ReasonConfigInvalid),
			"Set --cr-persist-path to a writable directory for persist-before-send CRs",
		)
	}
	if c.PollInterval <= 0 {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("poll-interval %s must be positive", c.PollInterval), ReasonConfigInvalid),
			"Set --poll-interval to a positive duration (e.g. 60s)",
		)
	}
	if c.FreezePollInterval <= 0 {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("freeze-poll-interval %s must be positive", c.FreezePollInterval), ReasonConfigInvalid),
			"Set --freeze-poll-interval to a positive duration (e.g. 1s)",
		)
	}
	return nil
}

// ProvisionerProxyEnabled reports whether the provisioner-proxy monitor should
// run. It is active only in cluster-only mode where no host daemon exists.
func (c Config) ProvisionerProxyEnabled() bool {
	return c.Mode == ModeClusterOnly
}
