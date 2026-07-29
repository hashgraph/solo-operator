// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"path/filepath"
	"regexp"
	"time"

	"github.com/automa-saga/errx"
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
	DefaultUpgradePath         = "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/current"
	DefaultPreviousUpgradePath = "/opt/hgcapp/services-hedera/HapiApp2.0/data/upgrade/previous"
	DefaultEventsPath          = "/opt/solo/weaver/uc/events"
	DefaultCRPersistPath       = "/opt/solo/weaver/uc/crs"
	DefaultBackupPath          = "/opt/solo/weaver/uc/backup"
	DefaultSocketPath          = "/opt/solo/weaver/uc/uc.sock"

	// DefaultPollInterval is the normal marker-file polling cadence. It drops to
	// FreezePollInterval after freeze_scheduled.mf is seen, then reverts.
	DefaultPollInterval = 60 * time.Second
	// FreezePollInterval is the fast cadence used during the freeze window to
	// minimise now_frozen.mf detection latency.
	FreezePollInterval = 1 * time.Second

	// DefaultMetricsURL is the consensus node's Prometheus metrics endpoint on the
	// shared pod localhost. The platform-status sensor scrapes platform_PlatformStatus
	// from here. Port 9999 matches consensus.PrometheusPort.
	DefaultMetricsURL = "http://localhost:9999/metrics"
	// DefaultPlatformPollInterval is the cadence at which the platform-status sensor
	// scrapes the metrics endpoint. It is a lightweight local HTTP GET, so a short
	// interval keeps status.platformStatus responsive without meaningful cost.
	DefaultPlatformPollInterval = 5 * time.Second

	// DefaultMetricsStaleness is the threshold after which the metrics-derived
	// platform status is considered stale and the effective source falls back to
	// the log-derived value. ~3× the default 5s scrape interval.
	DefaultMetricsStaleness = 15 * time.Second

	// DefaultConfigCRPollInterval is how often the UC re-checks each config CR's
	// status.conditions while waiting for Valid=True during the execute phase.
	DefaultConfigCRPollInterval = 1 * time.Second
	// DefaultConfigCRDeployTimeout is the total wall-clock budget for scanning,
	// creating, and waiting for all config CRs to reach Valid=True. 20 minutes is
	// generous; a healthy operator reconciles ConfigMap-backed CRs within seconds.
	DefaultConfigCRDeployTimeout = 20 * time.Minute
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
	// PodName is the UC pod's own name, injected via the downward API
	// (metadata.name). The platform-status sensor patches this pod's
	// operator.solo.hiero.org/platform-status condition (pods/status) by this name,
	// and derives the owning ConsensusCapsule name from it (pod
	// "<capsule>-<ordinal>" → "<capsule>") for its audit/log labels.
	PodName string
	// KubeconfigPath is optional; empty means in-cluster config.
	KubeconfigPath string

	// MetricsAddr is the listen address for the UC's own /metrics endpoint.
	// Env: UC_METRICS_ADDR.
	MetricsAddr string
	// MetricsURL is the consensus node's Prometheus endpoint scraped by the
	// platform-status sensor. Env: UC_METRICS_URL.
	MetricsURL string
	// PlatformPollInterval is the cadence of the platform-status metrics scrape.
	// Env: UC_PLATFORM_POLL_INTERVAL.
	PlatformPollInterval time.Duration

	// ConsensusLogFile is the path to the consensus node's log file tailed by the
	// log monitor for PLATFORM_STATUS transitions. Env: UC_CONSENSUS_LOG_FILE.
	ConsensusLogFile string
	// MetricsStaleness is the threshold after which the metrics-derived status is
	// considered stale for the effective source. Env: UC_METRICS_STALENESS.
	MetricsStaleness time.Duration
	// LogPollInterval is the cadence at which the log monitor tails the consensus
	// log file for PLATFORM_STATUS transitions. Env: UC_LOG_POLL_INTERVAL.
	LogPollInterval time.Duration

	// UpgradePath is the directory polled for marker files.
	//
	// SAME-FILESYSTEM CONSTRAINT: UpgradePath and PreviousUpgradePath must reside
	// on the same filesystem (same mount point). Package rotation is implemented
	// via os.Rename(current, previous), which maps to a single atomic rename(2)
	// syscall. rename(2) fails with EXDEV when src and dst are on different
	// devices; there is no fallback that preserves atomicity across filesystems.
	// In the default layout both paths share data/upgrade/ on the same volume.
	// If the operator mounts them separately they must still be on the same block
	// device or bind-mount from the same source.
	UpgradePath string
	// PreviousUpgradePath is the directory holding the last applied upgrade package.
	// Used as a fallback when UpgradePath has already been rotated out after a successful install.
	// See UpgradePath for the same-filesystem constraint.
	PreviousUpgradePath string
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
	// ConfigCRPollInterval is how often the UC polls each config CR for Valid=True
	// during the execute phase (cluster-only mode). Env: UC_CONFIG_CR_POLL_INTERVAL.
	ConfigCRPollInterval time.Duration
	// ConfigCRDeployTimeout is the total budget for the config-CR deployment step
	// (scan + create + wait for Valid=True). Env: UC_CONFIG_CR_DEPLOY_TIMEOUT.
	ConfigCRDeployTimeout time.Duration
}

// DefaultConfig returns a Config pre-populated with the documented defaults.
func DefaultConfig() Config {
	return Config{
		Mode:                  ModeMainnet,
		NodeID:                -1,
		MetricsAddr:           DefaultMetricsAddr,
		ConsensusLogFile:      DefaultConsensusLogFile,
		MetricsStaleness:      DefaultMetricsStaleness,
		LogPollInterval:       DefaultLogPollInterval,
		UpgradePath:           DefaultUpgradePath,
		PreviousUpgradePath:   DefaultPreviousUpgradePath,
		EventsPath:            DefaultEventsPath,
		CRPersistPath:         DefaultCRPersistPath,
		BackupPath:            DefaultBackupPath,
		SocketPath:            DefaultSocketPath,
		MetricsURL:            DefaultMetricsURL,
		PlatformPollInterval:  DefaultPlatformPollInterval,
		PollInterval:          DefaultPollInterval,
		FreezePollInterval:    FreezePollInterval,
		ConfigCRPollInterval:  DefaultConfigCRPollInterval,
		ConfigCRDeployTimeout: DefaultConfigCRDeployTimeout,
	}
}

// ordinalSuffix matches the trailing "-<ordinal>" a StatefulSet appends to its
// pod names (e.g. "node0-0"). Stripping it yields the StatefulSet name, which
// equals the ConsensusCapsule name (consensus.StatefulSetName).
var ordinalSuffix = regexp.MustCompile(`-\d+$`)

// CapsuleName returns the owning ConsensusCapsule name derived from PodName by
// stripping the StatefulSet ordinal suffix. Empty when PodName is unset.
func (c Config) CapsuleName() string {
	return ordinalSuffix.ReplaceAllString(c.PodName, "")
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
	if c.CapsuleName() == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("pod-name is required (used to derive the ConsensusCapsule name)"), ReasonConfigInvalid),
			"Set --pod-name (UC_POD_NAME) via the downward API fieldRef metadata.name",
		)
	}
	if c.MetricsAddr == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("metrics-addr is required"), ReasonConfigInvalid),
			"Set --metrics-addr to the UC metrics listen address (e.g. :9990)",
		)
	}
	if c.MetricsURL == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("metrics-url is required"), ReasonConfigInvalid),
			"Set --metrics-url to the consensus node's Prometheus endpoint (e.g. http://localhost:9999/metrics)",
		)
	}
	if c.PlatformPollInterval <= 0 {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("platform-poll-interval %s must be positive", c.PlatformPollInterval), ReasonConfigInvalid),
			"Set --platform-poll-interval to a positive duration (e.g. 5s)",
		)
	}
	if c.UpgradePath == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("upgrade-path is required"), ReasonConfigInvalid),
			"Set --upgrade-path to the consensus node's upgrade-artifacts directory (e.g. data/upgrade/current)",
		)
	}
	if c.PreviousUpgradePath == "" {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("previous-upgrade-path is required"), ReasonConfigInvalid),
			"Set --previous-upgrade-path to the consensus node's previous upgrade-artifacts directory (e.g. data/upgrade/previous)",
		)
	}
	// Upgrade-path rotation uses os.Rename(current, previous), which is only
	// atomic when both paths are on the same filesystem. Siblings under the same
	// parent directory guarantee this — enforce it here so an EXDEV failure at
	// rotation time is impossible.
	upgradeDir := filepath.Dir(filepath.Clean(c.UpgradePath))
	previousDir := filepath.Dir(filepath.Clean(c.PreviousUpgradePath))
	if upgradeDir != previousDir {
		return errx.WithHints(
			errx.WithReason(
				errConfigInvalid.New("upgrade-path and previous-upgrade-path must be siblings (same parent directory): got %q and %q",
					c.UpgradePath, c.PreviousUpgradePath),
				ReasonConfigInvalid),
			"Set --upgrade-path and --previous-upgrade-path to directories under the same parent (e.g. data/upgrade/current and data/upgrade/previous)",
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
	if c.ConfigCRPollInterval <= 0 {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("config-cr-poll-interval %s must be positive", c.ConfigCRPollInterval), ReasonConfigInvalid),
			"Set --config-cr-poll-interval to a positive duration (e.g. 5s)",
		)
	}
	if c.ConfigCRDeployTimeout <= 0 {
		return errx.WithHints(
			errx.WithReason(errConfigInvalid.New("config-cr-deploy-timeout %s must be positive", c.ConfigCRDeployTimeout), ReasonConfigInvalid),
			"Set --config-cr-deploy-timeout to a positive duration (e.g. 20m)",
		)
	}
	return nil
}

// ProvisionerProxyEnabled reports whether the provisioner-proxy monitor should
// run. It is active only in cluster-only mode where no host daemon exists.
func (c Config) ProvisionerProxyEnabled() bool {
	return c.Mode == ModeClusterOnly
}
