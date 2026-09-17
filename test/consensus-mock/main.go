// SPDX-License-Identifier: Apache-2.0

// Command consensus-mock is a fast fake stand-in for the consensus-node
// container, used to time the UC-driven upgrade flow. See README.md.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const gracefulStopTimeout = 5 * time.Second

func main() {
	fmt.Printf("consensus-mock: started at %s\n", time.Now().UTC().Format(time.RFC3339Nano))

	cfg := parseFlags()
	log.Printf("consensus-mock: node-id=%d grpc-addr=%s upgrade-path=%s consensus-log-file=%s",
		cfg.nodeID, cfg.grpcAddr, cfg.upgradePath, cfg.consensusLogFile)

	statusWriter, err := newPlatformStatusWriter(cfg.consensusLogFile)
	if err != nil {
		log.Fatalf("consensus-mock: %v", err)
	}
	statusWriter.startupSequence(cfg.startupActiveDelay)

	grpcServer := newGRPCServer(&freezeServer{
		upgradePath:         cfg.upgradePath,
		status:              statusWriter,
		freezeCompleteDelay: cfg.freezeCompleteDelay,
	})

	lis, err := net.Listen("tcp", cfg.grpcAddr)
	if err != nil {
		log.Fatalf("consensus-mock: listen %s: %v", cfg.grpcAddr, err)
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("consensus-mock: grpc server stopped: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(gracefulStopTimeout):
		grpcServer.Stop()
	}

	fmt.Printf("consensus-mock: stopped at %s\n", time.Now().UTC().Format(time.RFC3339Nano))
}
