// SPDX-License-Identifier: Apache-2.0

package client

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// TestWaitReachable_DisabledReturnsNil verifies a non-positive timeout disables
// the preflight and returns immediately without touching the network (hc unused).
func TestWaitReachable_DisabledReturnsNil(t *testing.T) {
	c := &Client{} // no hc needed on the disabled path
	if err := c.WaitReachable("127.0.0.1:1", 0); err != nil {
		t.Fatalf("expected nil on zero timeout, got %v", err)
	}
}

// TestWaitReachable_UnreachableTimesOut verifies an unreachable endpoint fails
// with NodeUnreachable within the budget. Port 1 on loopback refuses/So the TCP
// dial fails fast and hc is never used, so a bare Client is sufficient.
func TestWaitReachable_UnreachableTimesOut(t *testing.T) {
	c := &Client{}
	start := time.Now()
	err := c.WaitReachable("127.0.0.1:1", 50*time.Millisecond)
	wantReason(t, err, reasonNodeUnreachable)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("WaitReachable overran its budget: took %s", elapsed)
	}
}

// wantReason asserts err carries the given errx reason code.
func wantReason(t *testing.T, err error, reason errx.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %q, got nil", reason)
	}
	got, ok := errx.ReasonOf(err)
	if !ok || got != reason {
		t.Errorf("reason = %q (ok=%v), want %q", got, ok, reason)
	}
}

// testClient builds a Client over an offline hiero.Client (ClientForNetwork does
// not dial), suitable for driving Submit through stubbed seams.
func testClient(t *testing.T) *Client {
	t.Helper()
	key, err := hiero.PrivateKeyGenerateEd25519()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	network := map[string]hiero.AccountID{"127.0.0.1:50211": {Account: 3}}
	c, err := New(network, hiero.AccountID{Account: 2}, key, Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// txID is a throwaway transaction id for stubbed responses.
func txID() hiero.TransactionID {
	return hiero.TransactionIDGenerate(hiero.AccountID{Account: 2})
}

func TestNew_Validations(t *testing.T) {
	key, err := hiero.PrivateKeyGenerateEd25519()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	t.Run("empty network", func(t *testing.T) {
		_, err := New(nil, hiero.AccountID{Account: 2}, key, Options{})
		wantReason(t, err, reasonNoNetwork)
	})

	t.Run("zero payer", func(t *testing.T) {
		network := map[string]hiero.AccountID{"127.0.0.1:50211": {Account: 3}}
		_, err := New(network, hiero.AccountID{}, key, Options{})
		wantReason(t, err, reasonInvalidPayer)
	})
}

func TestNew_DefaultsOperatorAndFee(t *testing.T) {
	c := testClient(t)
	if got := c.OperatorAccountID(); got.Account != 2 {
		t.Errorf("operator account = %v, want 0.0.2", got)
	}
	if fee := c.Hiero().GetDefaultMaxTransactionFee(); fee.AsTinybar() != hiero.NewHbar(defaultMaxTxFeeHbar).AsTinybar() {
		t.Errorf("default max tx fee = %s, want %d hbar", fee, defaultMaxTxFeeHbar)
	}
}

func TestNew_LocalNonZeroShardRealm(t *testing.T) {
	// A local network in shard 5 / realm 2 must produce a client bound to that
	// shard/realm (ClientForNetwork would have pinned 0/0).
	key, err := hiero.PrivateKeyGenerateEd25519()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	network := map[string]hiero.AccountID{
		"10.0.0.7:51211": {Shard: 5, Realm: 2, Account: 7},
	}
	payer := hiero.AccountID{Shard: 5, Realm: 2, Account: 58}

	c, err := New(network, payer, key, Options{TLS: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if c.Hiero().GetShard() != 5 || c.Hiero().GetRealm() != 2 {
		t.Errorf("client shard/realm = %d/%d, want 5/2", c.Hiero().GetShard(), c.Hiero().GetRealm())
	}
	// Certificate verification is off by default so TLS works against a local
	// network with no published address book.
	if c.Hiero().GetCertificateVerification() {
		t.Error("certificate verification should default to off for local networks")
	}
}

func TestNew_MixedShardRealmRejected(t *testing.T) {
	key, err := hiero.PrivateKeyGenerateEd25519()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	// Nodes in different shards are not a single valid network.
	network := map[string]hiero.AccountID{
		"10.0.0.7:50211": {Shard: 5, Realm: 2, Account: 7},
		"10.0.0.8:50211": {Shard: 6, Realm: 2, Account: 8},
	}
	_, err = New(network, hiero.AccountID{Account: 2}, key, Options{})
	wantReason(t, err, reasonNoNetwork)
}

func TestSubmit_SuccessRunsVerify(t *testing.T) {
	c := testClient(t)
	id := txID()
	node := hiero.AccountID{Account: 3}

	c.execute = func(hiero.TransactionInterface, *hiero.Client) (hiero.TransactionResponse, error) {
		return hiero.TransactionResponse{TransactionID: id, NodeID: node}, nil
	}
	c.getReceipt = func(*hiero.TransactionResponse, *hiero.Client) (hiero.TransactionReceipt, error) {
		return hiero.TransactionReceipt{Status: hiero.StatusSuccess}, nil
	}

	var gotResult *Result
	verify := func(_ *Client, r *Result) error {
		gotResult = r
		return nil
	}

	res, err := c.Submit(hiero.NewFreezeTransaction(), verify)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Receipt.Status != hiero.StatusSuccess {
		t.Errorf("status = %s, want SUCCESS", res.Receipt.Status)
	}
	if res.TransactionID != id || res.NodeID != node {
		t.Errorf("result id/node = %s/%s, want %s/%s", res.TransactionID, res.NodeID, id, node)
	}
	if gotResult == nil || gotResult.TransactionID != id {
		t.Error("verify hook did not receive the result with the transaction id")
	}
}

func TestSubmit_NonSuccessReceiptStatus(t *testing.T) {
	c := testClient(t)
	c.execute = func(hiero.TransactionInterface, *hiero.Client) (hiero.TransactionResponse, error) {
		return hiero.TransactionResponse{TransactionID: txID()}, nil
	}
	c.getReceipt = func(*hiero.TransactionResponse, *hiero.Client) (hiero.TransactionReceipt, error) {
		return hiero.TransactionReceipt{Status: hiero.StatusInvalidSignature}, nil
	}

	_, err := c.Submit(hiero.NewFreezeTransaction(), nil)
	wantReason(t, err, reasonTxStatusFailed)
	if !strings.Contains(err.Error(), hiero.StatusInvalidSignature.String()) {
		t.Errorf("error should carry the status: %v", err)
	}
}

func TestSubmit_ExceptionalReceiptStatus(t *testing.T) {
	c := testClient(t)
	id := txID()
	c.execute = func(hiero.TransactionInterface, *hiero.Client) (hiero.TransactionResponse, error) {
		return hiero.TransactionResponse{TransactionID: id}, nil
	}
	// The SDK signals a non-SUCCESS outcome by returning ErrHederaReceiptStatus.
	c.getReceipt = func(*hiero.TransactionResponse, *hiero.Client) (hiero.TransactionReceipt, error) {
		return hiero.TransactionReceipt{Status: hiero.StatusInvalidFreezeTransactionBody},
			hiero.ErrHederaReceiptStatus{TxID: id, Status: hiero.StatusInvalidFreezeTransactionBody}
	}

	_, err := c.Submit(hiero.NewFreezeTransaction(), nil)
	wantReason(t, err, reasonTxStatusFailed)
	if !strings.Contains(err.Error(), hiero.StatusInvalidFreezeTransactionBody.String()) {
		t.Errorf("error should carry the exceptional status: %v", err)
	}
}

func TestSubmit_ExecuteError(t *testing.T) {
	c := testClient(t)
	c.execute = func(hiero.TransactionInterface, *hiero.Client) (hiero.TransactionResponse, error) {
		return hiero.TransactionResponse{}, fmt.Errorf("dial tcp: connection refused")
	}

	_, err := c.Submit(hiero.NewFreezeTransaction(), nil)
	wantReason(t, err, reasonSubmitFailed)
}

func TestSubmit_ReceiptError(t *testing.T) {
	c := testClient(t)
	c.execute = func(hiero.TransactionInterface, *hiero.Client) (hiero.TransactionResponse, error) {
		return hiero.TransactionResponse{TransactionID: txID()}, nil
	}
	c.getReceipt = func(*hiero.TransactionResponse, *hiero.Client) (hiero.TransactionReceipt, error) {
		return hiero.TransactionReceipt{}, fmt.Errorf("receipt query timed out")
	}

	_, err := c.Submit(hiero.NewFreezeTransaction(), nil)
	wantReason(t, err, reasonReceiptFailed)
}

func TestSubmit_VerifyErrorPropagates(t *testing.T) {
	c := testClient(t)
	c.execute = func(hiero.TransactionInterface, *hiero.Client) (hiero.TransactionResponse, error) {
		return hiero.TransactionResponse{TransactionID: txID()}, nil
	}
	c.getReceipt = func(*hiero.TransactionResponse, *hiero.Client) (hiero.TransactionReceipt, error) {
		return hiero.TransactionReceipt{Status: hiero.StatusSuccess}, nil
	}

	sentinel := errx.Decorate(fmt.Errorf("hash mismatch"), errx.Reason("VerifyFailed"))
	_, err := c.Submit(hiero.NewFreezeTransaction(), func(*Client, *Result) error {
		return sentinel
	})
	wantReason(t, err, errx.Reason("VerifyFailed"))
}
