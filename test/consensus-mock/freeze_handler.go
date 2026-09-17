// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/proto"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"

	"github.com/hashgraph/solo-operator/internal/uc"
)

// freezeServer implements just the FreezeService RPC beacon uses
// (internal/beacon/ops/freeze.go builds every freeze variant — Prepare,
// FreezeUpgrade, TelemetryUpgrade, FreezeOnly, FreezeAbort — as a single
// FreezeTransaction distinguished only by freeze_type).
type freezeServer struct {
	services.UnimplementedFreezeServiceServer

	upgradePath         string
	status              *platformStatusWriter
	freezeCompleteDelay time.Duration
}

func (s *freezeServer) Freeze(_ context.Context, tx *services.Transaction) (*services.TransactionResponse, error) {
	body, err := decodeTransactionBody(tx)
	if err != nil {
		log.Printf("consensus-mock: freeze: %v", err)
		return &services.TransactionResponse{NodeTransactionPrecheckCode: services.ResponseCodeEnum_INVALID_TRANSACTION_BODY}, nil
	}

	fz := body.GetFreeze()
	if fz == nil {
		return &services.TransactionResponse{NodeTransactionPrecheckCode: services.ResponseCodeEnum_INVALID_FREEZE_TRANSACTION_BODY}, nil
	}

	switch fz.GetFreezeType() {
	case services.FreezeType_PREPARE_UPGRADE:
		// The manifest itself is staged into the upgrade path out of band
		// (test/consensus-mock/Taskfile.yml build:manifest + kubectl cp) —
		// consensus-mock only signals that a prepare was requested.
		s.touchMarkerLogged(uc.MarkerExecuteImmediate, "PREPARE_UPGRADE")

	case services.FreezeType_FREEZE_ABORT:
		s.touchMarkerLogged(uc.MarkerFreezeAborted, "FREEZE_ABORT")

	case services.FreezeType_FREEZE_ONLY, services.FreezeType_FREEZE_UPGRADE, services.FreezeType_TELEMETRY_UPGRADE:
		seconds := fz.GetStartTime().GetSeconds()
		if err := writeFreezeScheduled(s.upgradePath, seconds); err != nil {
			log.Printf("consensus-mock: freeze %s: %v", fz.GetFreezeType(), err)
			break
		}
		log.Printf("consensus-mock: freeze %s scheduled at %d -> %s", fz.GetFreezeType(), seconds, uc.MarkerFreezeScheduled)
		s.scheduleNowFrozen(seconds)

	default:
		log.Printf("consensus-mock: freeze: unrecognised freeze type %s", fz.GetFreezeType())
	}

	return &services.TransactionResponse{NodeTransactionPrecheckCode: services.ResponseCodeEnum_OK}, nil
}

// scheduleNowFrozen fires at the scheduled freeze time (or immediately if
// already past), touches now_frozen.mf, and drives the ACTIVE -> FREEZING ->
// FREEZE_COMPLETE platform-status transition.
func (s *freezeServer) scheduleNowFrozen(startSeconds int64) {
	delay := time.Until(time.Unix(startSeconds, 0))
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		if err := touchMarker(s.upgradePath, uc.MarkerNowFrozen); err != nil {
			log.Printf("consensus-mock: now-frozen: %v", err)
			return
		}
		log.Printf("consensus-mock: freeze: reached scheduled time -> %s", uc.MarkerNowFrozen)
		s.status.freezeSequence(s.freezeCompleteDelay)
	})
}

// touchMarkerLogged touches marker and logs the outcome under label,
// shared by the presence-only freeze variants (PREPARE_UPGRADE, FREEZE_ABORT).
func (s *freezeServer) touchMarkerLogged(marker, label string) {
	if err := touchMarker(s.upgradePath, marker); err != nil {
		log.Printf("consensus-mock: freeze %s: %v", label, err)
		return
	}
	log.Printf("consensus-mock: freeze %s -> %s", label, marker)
}

// decodeTransactionBody unwraps a HAPI Transaction into its TransactionBody.
// The hiero-sdk-go client beacon uses always populates SignedTransactionBytes
// (a marshalled SignedTransaction wrapping the body bytes); the deprecated
// bare Body/Sigs fields are not used by any client this mock needs to serve.
func decodeTransactionBody(tx *services.Transaction) (*services.TransactionBody, error) {
	raw := tx.GetSignedTransactionBytes()
	if len(raw) == 0 {
		return nil, fmt.Errorf("transaction has no signedTransactionBytes")
	}

	signed := &services.SignedTransaction{}
	if err := proto.Unmarshal(raw, signed); err != nil {
		return nil, fmt.Errorf("unmarshal signed transaction: %w", err)
	}
	body := &services.TransactionBody{}
	if err := proto.Unmarshal(signed.GetBodyBytes(), body); err != nil {
		return nil, fmt.Errorf("unmarshal transaction body: %w", err)
	}
	return body, nil
}
