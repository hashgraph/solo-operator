// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/uc"
)

// mockConfig fields each have a MOCK_-prefixed env var fallback, flag-overridable.
type mockConfig struct {
	grpcAddr            string
	upgradePath         string
	consensusLogFile    string
	startupActiveDelay  time.Duration
	freezeCompleteDelay time.Duration
	nodeID              int
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func parseFlags() mockConfig {
	var cfg mockConfig

	flag.StringVar(&cfg.grpcAddr, "grpc-addr", envOr("MOCK_GRPC_ADDR", fmt.Sprintf(":%d", common.GrpcPort)),
		"Address to listen for HAPI gRPC on")
	flag.StringVar(&cfg.upgradePath, "upgrade-path", envOr("MOCK_UPGRADE_PATH", uc.DefaultUpgradePath),
		"Directory to write UC marker files into")
	flag.StringVar(&cfg.consensusLogFile, "consensus-log-file", envOr("MOCK_CONSENSUS_LOG_FILE", uc.DefaultConsensusLogFile),
		"Path to the consensus log file the UC log monitor tails for PLATFORM_STATUS transitions")
	flag.DurationVar(&cfg.startupActiveDelay, "startup-active-delay", 0*time.Millisecond,
		"Delay before transitioning STARTING_UP -> ACTIVE")
	flag.DurationVar(&cfg.freezeCompleteDelay, "freeze-complete-delay", time.Second,
		"Delay before transitioning FREEZING -> FREEZE_COMPLETE after now_frozen.mf is written")
	flag.IntVar(&cfg.nodeID, "node-id", 0, "Cosmetic node id included in log lines")

	flag.Parse()
	return cfg
}
