// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	ConditionTypeDNSReady           = "DNSReady"
	ConditionTypeConsensusPortReady = "ConsensusPortReady"

	ReasonDNSNotReady           = "DNSNotReady"
	ReasonDNSResolved           = "DNSResolved"
	ReasonConsensusPortNotReady = "ConsensusPortNotReady"
	ReasonConsensusPortReady    = "ConsensusPortReady"

	tcpDialTimeout = 5 * time.Second
)

// HealthChecker performs liveness checks against a consensus node.
type HealthChecker interface {
	CheckDNS(ctx context.Context, hostname string) error
	CheckTCP(ctx context.Context, addr string) error
}

type defaultHealthChecker struct{}

func (h *defaultHealthChecker) CheckDNS(ctx context.Context, hostname string) error {
	_, err := net.DefaultResolver.LookupHost(ctx, hostname)
	return err
}

func (h *defaultHealthChecker) CheckTCP(ctx context.Context, addr string) error {
	d := net.Dialer{Timeout: tcpDialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// runHealthChecks performs DNS and TCP checks against the capsule's gossip endpoint.
// Returns (allReady bool) and updates conditions on the capsule status.
func runHealthChecks(ctx context.Context, hc HealthChecker, capsule *operatorv1alpha1.ConsensusCapsule, hostname string, gossipPort int32) bool {
	ready := true

	if err := hc.CheckDNS(ctx, hostname); err != nil {
		setCondition(&capsule.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeDNSReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonDNSNotReady,
			Message:            fmt.Sprintf("DNS lookup failed for %s: %v", hostname, err),
			ObservedGeneration: capsule.Generation,
		})
		ready = false
	} else {
		setCondition(&capsule.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeDNSReady,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonDNSResolved,
			Message:            fmt.Sprintf("DNS resolved: %s", hostname),
			ObservedGeneration: capsule.Generation,
		})
	}

	addr := fmt.Sprintf("%s:%d", hostname, gossipPort)
	if err := hc.CheckTCP(ctx, addr); err != nil {
		setCondition(&capsule.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeConsensusPortReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonConsensusPortNotReady,
			Message:            fmt.Sprintf("TCP connection failed to %s: %v", addr, err),
			ObservedGeneration: capsule.Generation,
		})
		ready = false
	} else {
		setCondition(&capsule.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeConsensusPortReady,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonConsensusPortReady,
			Message:            fmt.Sprintf("TCP connection succeeded: %s", addr),
			ObservedGeneration: capsule.Generation,
		})
	}

	return ready
}
