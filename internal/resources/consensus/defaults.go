// SPDX-License-Identifier: Apache-2.0

package consensus

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
