// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// stubHealthChecker lets tests control DNS and TCP outcomes.
type stubHealthChecker struct {
	dnsErr error
	tcpErr error
}

func (s *stubHealthChecker) CheckDNS(_ context.Context, _ string) error { return s.dnsErr }
func (s *stubHealthChecker) CheckTCP(_ context.Context, _ string) error { return s.tcpErr }

func newTestCapsuleForHealth() *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test", Generation: 1},
	}
}

func conditionStatus(capsule *operatorv1alpha1.ConsensusCapsule, condType string) (metav1.ConditionStatus, bool) {
	for _, c := range capsule.Status.Conditions {
		if c.Type == condType {
			return c.Status, true
		}
	}
	return "", false
}

func TestRunHealthChecks_AllPass(t *testing.T) {
	capsule := newTestCapsuleForHealth()
	hc := &stubHealthChecker{}

	ready := runHealthChecks(context.Background(), hc, capsule, "node0.consensus.test.svc.cluster.local", 50111)

	if !ready {
		t.Error("expected ready=true when both checks pass")
	}
	if s, _ := conditionStatus(capsule, ConditionTypeDNSReady); s != metav1.ConditionTrue {
		t.Errorf("expected DNSReady=True, got %s", s)
	}
	if s, _ := conditionStatus(capsule, ConditionTypeConsensusPortReady); s != metav1.ConditionTrue {
		t.Errorf("expected ConsensusPortReady=True, got %s", s)
	}
}

func TestRunHealthChecks_DNSFails(t *testing.T) {
	capsule := newTestCapsuleForHealth()
	hc := &stubHealthChecker{dnsErr: errors.New("no such host")}

	ready := runHealthChecks(context.Background(), hc, capsule, "node0.consensus.test.svc.cluster.local", 50111)

	if ready {
		t.Error("expected ready=false when DNS fails")
	}
	if s, _ := conditionStatus(capsule, ConditionTypeDNSReady); s != metav1.ConditionFalse {
		t.Errorf("expected DNSReady=False, got %s", s)
	}
}

func TestRunHealthChecks_TCPFails(t *testing.T) {
	capsule := newTestCapsuleForHealth()
	hc := &stubHealthChecker{tcpErr: errors.New("connection refused")}

	ready := runHealthChecks(context.Background(), hc, capsule, "node0.consensus.test.svc.cluster.local", 50111)

	if ready {
		t.Error("expected ready=false when TCP fails")
	}
	if s, _ := conditionStatus(capsule, ConditionTypeConsensusPortReady); s != metav1.ConditionFalse {
		t.Errorf("expected ConsensusPortReady=False, got %s", s)
	}
	// DNS passed, so DNSReady should be True
	if s, _ := conditionStatus(capsule, ConditionTypeDNSReady); s != metav1.ConditionTrue {
		t.Errorf("expected DNSReady=True when only TCP fails, got %s", s)
	}
}

func TestRunHealthChecks_BothFail(t *testing.T) {
	capsule := newTestCapsuleForHealth()
	hc := &stubHealthChecker{
		dnsErr: errors.New("no such host"),
		tcpErr: errors.New("connection refused"),
	}

	ready := runHealthChecks(context.Background(), hc, capsule, "node0.consensus.test.svc.cluster.local", 50111)

	if ready {
		t.Error("expected ready=false when both checks fail")
	}
	if s, _ := conditionStatus(capsule, ConditionTypeDNSReady); s != metav1.ConditionFalse {
		t.Errorf("expected DNSReady=False, got %s", s)
	}
	if s, _ := conditionStatus(capsule, ConditionTypeConsensusPortReady); s != metav1.ConditionFalse {
		t.Errorf("expected ConsensusPortReady=False, got %s", s)
	}
}

func TestRunHealthChecks_ConditionReasonSet(t *testing.T) {
	capsule := newTestCapsuleForHealth()
	hc := &stubHealthChecker{dnsErr: errors.New("no such host")}

	runHealthChecks(context.Background(), hc, capsule, "node0.consensus.test.svc.cluster.local", 50111)

	for _, c := range capsule.Status.Conditions {
		if c.Type == ConditionTypeDNSReady {
			if c.Reason != ReasonDNSNotReady {
				t.Errorf("expected reason %s, got %s", ReasonDNSNotReady, c.Reason)
			}
			if c.ObservedGeneration != capsule.Generation {
				t.Errorf("expected ObservedGeneration %d, got %d", capsule.Generation, c.ObservedGeneration)
			}
			return
		}
	}
	t.Error("DNSReady condition not found")
}
