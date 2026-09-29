// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	services "github.com/hiero-ledger/hiero-sdk-go/v2/proto/services"

	"github.com/hashgraph/solo-operator/internal/uc"
)

// freezeServer implements the one FreezeService RPC every freeze variant uses.
type freezeServer struct {
	services.UnimplementedFreezeServiceServer

	upgradePath         string
	status              *platformStatusWriter
	freezeCompleteDelay time.Duration

	mu             sync.Mutex
	nowFrozenTimer *time.Timer
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
		// Manifest is staged separately (Taskfile build:manifest + kubectl cp).
		s.touchMarkerLogged(uc.MarkerExecuteImmediate, "PREPARE_UPGRADE")

	case services.FreezeType_FREEZE_ABORT:
		s.cancelNowFrozen()
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

// scheduleNowFrozen fires immediately if startSeconds is already past. A prior
// pending timer (e.g. from an earlier FREEZE_UPGRADE) is cancelled first, so
// only the latest schedule can ever fire.
func (s *freezeServer) scheduleNowFrozen(startSeconds int64) {
	delay := time.Until(time.Unix(startSeconds, 0))
	if delay < 0 {
		delay = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nowFrozenTimer != nil {
		s.nowFrozenTimer.Stop()
	}
	s.nowFrozenTimer = time.AfterFunc(delay, func() {
		if err := touchMarker(s.upgradePath, uc.MarkerNowFrozen); err != nil {
			log.Printf("consensus-mock: now-frozen: %v", err)
			return
		}
		log.Printf("consensus-mock: freeze: reached scheduled time -> %s", uc.MarkerNowFrozen)
		s.status.freezeSequence(s.freezeCompleteDelay)
	})
}

// cancelNowFrozen stops a pending now_frozen.mf timer armed by an earlier
// FREEZE_UPGRADE, so a FREEZE_ABORT actually prevents it from firing.
func (s *freezeServer) cancelNowFrozen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nowFrozenTimer != nil {
		s.nowFrozenTimer.Stop()
		s.nowFrozenTimer = nil
	}
}

func (s *freezeServer) touchMarkerLogged(marker, label string) {
	if err := touchMarker(s.upgradePath, marker); err != nil {
		log.Printf("consensus-mock: freeze %s: %v", label, err)
		return
	}
	log.Printf("consensus-mock: freeze %s -> %s", label, marker)
}

// decodeTransactionBody unwraps SignedTransactionBytes into a TransactionBody.
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
