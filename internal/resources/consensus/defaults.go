// SPDX-License-Identifier: Apache-2.0

package consensus

const (
	// HapiBase is the working directory inside the consensus-node container.
	HapiBase = "/opt/hgcapp/services-hedera/HapiApp2.0"

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
	// AlloyDefaultImage is the fallback Grafana Alloy sidecar container image.
	AlloyDefaultImage = "grafana/alloy:v1.14.2"

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
	// CheetahDefaultImage is the fallback cheetah container image.
	CheetahDefaultImage = "ghcr.io/hashgraph/cheetah:latest"

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
