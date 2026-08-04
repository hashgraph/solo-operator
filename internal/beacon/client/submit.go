// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"fmt"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// VerifyFunc is an optional read-back check run after a SUCCESS receipt, for
// stateful operations that must confirm the expected on-chain state is
// observable (e.g. a file's new hash, a schedule's existence). It receives the
// same Client so it can issue queries against the target network. Returning an
// error fails the whole Submit.
type VerifyFunc func(c *Client, result *Result) error

// Result is the verified outcome of a submitted transaction.
type Result struct {
	// TransactionID is the id the network assigned the submitted transaction.
	TransactionID hiero.TransactionID
	// NodeID is the node the transaction was submitted to.
	NodeID hiero.AccountID
	// Receipt is the SUCCESS receipt returned by the network.
	Receipt hiero.TransactionReceipt
}

// Submit signs (with the operator key set on the client), executes tx, awaits
// its receipt, and asserts the status is SUCCESS. A non-SUCCESS status is
// returned as a typed error carrying the status string; only on SUCCESS does it
// run verify (when non-nil) and return the Result.
func (c *Client) Submit(tx hiero.TransactionInterface, verify VerifyFunc) (*Result, error) {
	resp, err := c.execute(tx, c.hc)
	if err != nil {
		return nil, errx.Decorate(
			fmt.Errorf("submit transaction: %w", err),
			reasonSubmitFailed,
			"Confirm the target node is reachable and the payer key is authorized to submit",
		)
	}

	receipt, err := c.getReceipt(&resp, c.hc)
	if err != nil {
		// The SDK returns ErrHederaReceiptStatus (receipt + status) when the
		// network accepted the transaction but the outcome was exceptional; that
		// status is the meaningful result, not a transport failure.
		var statusErr hiero.ErrHederaReceiptStatus
		if errors.As(err, &statusErr) {
			return nil, statusError(statusErr.TxID, statusErr.Status)
		}
		return nil, errx.Decorate(
			fmt.Errorf("await receipt: %w", err),
			reasonReceiptFailed,
			"The transaction was submitted but its receipt could not be retrieved; re-query the receipt before resubmitting to avoid a duplicate",
		)
	}

	if receipt.Status != hiero.StatusSuccess {
		return nil, statusError(resp.TransactionID, receipt.Status)
	}

	result := &Result{
		TransactionID: resp.TransactionID,
		NodeID:        resp.NodeID,
		Receipt:       receipt,
	}

	logx.As().Debug().
		Str("reason", reasonTxSubmitted.String()).
		Str("transaction_id", resp.TransactionID.String()).
		Str("node", resp.NodeID.String()).
		Str("status", receipt.Status.String()).
		Msg("transaction reached SUCCESS")

	if verify != nil {
		if err := verify(c, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// statusError builds the typed non-SUCCESS error carrying the transaction id and
// the network status code.
func statusError(txID hiero.TransactionID, status hiero.Status) error {
	return errx.Decorate(
		fmt.Errorf("transaction %s failed with status %s", txID, status),
		reasonTxStatusFailed,
		fmt.Sprintf("The network rejected the transaction with %s; consult the Hedera response-code reference for that status", status),
	)
}
