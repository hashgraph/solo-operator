// SPDX-License-Identifier: Apache-2.0

package consensus

const (
	// UCDataPath is the root directory for all UC runtime state (crs/, events/, backup/, uc.sock).
	// Backed by the "uc-data" volume, which is an emptyDir by default or a PVC when
	// spec.persistentVolumeClaims.ucData is set. The UC binary creates subdirs as needed.
	UCDataPath = "/opt/solo/weaver/uc"
)

const (
	// DefaultConsensusContainerName is the consensus-node container name used when
	// spec.podProperties.containers.consensusNode.name is unset.
	DefaultConsensusContainerName = "consensus-node"

	// HapiBase is the working directory inside the consensus-node container.
	HapiBase = "/opt/hgcapp/services-hedera/HapiApp2.0"

	// HederaUID is the uid/gid the consensus-node image runs as (the "hedera"
	// user). The UC sidecar runs as the same uid:gid so it shares ownership of
	// the upgrade PVC (data/upgrade) with the node; fsGroup uses it as well.
	HederaUID int64 = 2000

	// AlloyUID is the "alloy" user in grafana/alloy; the image itself runs as root.
	AlloyUID int64 = 473

	// AlloyStoragePath is writable by AlloyUID, unlike the default relative to "/".
	AlloyStoragePath = "/var/lib/alloy/data"

	// GenesisInitImage is the image used by the genesis-config init container.
	// It only needs sh and cp.
	GenesisInitImage = "busybox:1.36.1"

	// SavedStateDir is the subdirectory written by the platform under data/saved
	// once the first round completes. Its presence means the node has live state
	// and does not need genesis-network.json on the next restart.
	SavedStateDir = "com.hedera.services.ServicesMain"
)

// Alloy telemetry sidecar defaults.
const (
	// AlloyDefaultAppName is the default application name label.
	AlloyDefaultAppName = "hedera-node"

	// AlloyDefaultEnvironment is the default environment label.
	AlloyDefaultEnvironment = "integration"

	// AlloyDefaultLogLevel is the default Alloy logging level.
	AlloyDefaultLogLevel = "info"
)

// Cheetah uploader defaults — kept in one place so that ConfigMap generation,
// container building, and tests all share the same values.
const (
	// CheetahDefaultLogLevel is the default pipeline log level.
	CheetahDefaultLogLevel = "Debug"

	// CheetahDefaultMaxProcessors is the default max concurrent file processors per pipeline.
	CheetahDefaultMaxProcessors = 10

	// CheetahDefaultConfigCheckInterval is the default interval for config hot-reload.
	CheetahDefaultConfigCheckInterval = "30s"

	// CheetahPprofPort is the pprof HTTP server port exposed by each cheetah container.
	CheetahPprofPort int32 = 6061

	// CheetahStatsPort is the stats HTTP server port exposed by each cheetah container.
	CheetahStatsPort int32 = 6060
)
