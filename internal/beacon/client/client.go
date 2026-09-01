// SPDX-License-Identifier: Apache-2.0

// Package client wraps a hiero.Client with the verified-submit helper every
// beacon operation uses.
//
// A beacon command resolves its target node(s) and payer (internal/beacon/config)
// and loads the payer key (internal/beacon/keystore), then builds a Client here
// pointed at those node(s) with the payer as operator. Operations submit exactly
// one transaction through Submit, which signs, executes, awaits the receipt, and
// asserts the outcome is SUCCESS — surfacing any non-SUCCESS status as a typed
// error (design §5: success means "the receipt status was SUCCESS and the
// expected on-chain state is observable," not merely "a transaction was sent").
package client

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/automa-saga/errx"
	"github.com/automa-saga/logx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// defaultMaxTxFeeHbar is the generous flat max transaction fee applied when the
// caller does not set one, mirroring yahcli: beacon does no exact fee
// estimation, it just caps the fee well above what any upgrade/freeze/file
// operation costs.
const defaultMaxTxFeeHbar = 100

// Fail-fast SDK timeouts. The hiero-sdk defaults (2-minute requestTimeout,
// 10-second grpcDeadline) make a call against an unreachable node stall for two
// silent minutes before erroring. beacon targets a local Solo/kind network where
// ops ACK in seconds, so we tighten the total budget while keeping the per-call
// deadline and the SDK's own retry/backoff (maxAttempts) intact.
const (
	clientRequestTimeout = 60 * time.Second
	clientGrpcDeadline   = 10 * time.Second
)

// Preflight-probe tuning (see WaitReachable). The TCP dial catches a missing
// port-forward/Service immediately; the HAPI probe is bounded independently of
// the SDK's own budget so the wait loop stays responsive.
const (
	tcpProbeTimeout     = 2 * time.Second
	hapiProbeTimeout    = 5 * time.Second
	probeInitialBackoff = 500 * time.Millisecond
	probeMaxBackoff     = 5 * time.Second
)

// Options tunes the underlying hiero.Client. The zero value targets a local,
// self-contained network (Solo/kind): plaintext transport, no certificate
// verification, and the default max transaction fee. It is not specific to
// mainnet/testnet — any network map is accepted.
type Options struct {
	// TLS selects TLS HAPI transport (node endpoints on :50212). Local networks
	// typically use plaintext; leave false.
	TLS bool
	// VerifyCertificate turns on node-certificate verification against the
	// network's address book. beacon's --tls-verify flag defaults this to true.
	VerifyCertificate bool
	// MaxTransactionFee overrides the default flat fee cap when non-zero.
	MaxTransactionFee hiero.Hbar
	// LogLevel is beacon's resolved log level (warn|info|debug). When "debug"
	// the underlying SDK logger is turned up to TRACE so its execute/receipt
	// retry loop is visible — the single most useful signal when a submit
	// precheck succeeds but the receipt never arrives ("exceptional precheck
	// status OK"). Any other value leaves the SDK logger at its default
	// (quiet). It is independent of beacon's own logx logger.
	LogLevel string
}

// Client is a beacon-flavored hiero.Client: the SDK client plus the submit seam
// used to verify each transaction's outcome.
type Client struct {
	hc *hiero.Client

	// execute and getReceipt are seams over the SDK's package-level execute and
	// per-response receipt calls, swapped in unit tests to exercise Submit
	// without a live network. Production wiring points them at the real SDK.
	execute    func(tx hiero.TransactionInterface, hc *hiero.Client) (hiero.TransactionResponse, error)
	getReceipt func(resp *hiero.TransactionResponse, hc *hiero.Client) (hiero.TransactionReceipt, error)
}

// New builds a verified-submit Client for network (each node endpoint
// "host:port" → its fee-collection AccountID), with payer as the operator signed
// by payerKey. Any network map is accepted — a local Solo/kind cluster just as
// much as a public network — because the client is built purely from the given
// nodes with no mirror node or published address book required. Callers must
// Close the returned Client.
func New(network map[string]hiero.AccountID, payer hiero.AccountID, payerKey hiero.PrivateKey, opts Options) (*Client, error) {
	if len(network) == 0 {
		return nil, errx.Decorate(
			errClientConfig.New("empty node network"),
			reasonNoNetwork,
			"Resolve at least one target node before building the client (check config.yaml and --node-*)",
		)
	}
	if payer.Account == 0 {
		return nil, errx.Decorate(
			errClientConfig.New("payer account is zero"),
			reasonInvalidPayer,
			"Set --payer/-p or a network defaultPayer to a privileged account (0.0.2, 0.0.50, or 0.0.58)",
		)
	}

	// ClientForNetworkV2 (unlike ClientForNetwork) derives the client's
	// shard/realm from the node accounts and surfaces a malformed network as an
	// error, so a non-zero-shard local network works and a bad node map is not
	// silently swallowed.
	hc, err := hiero.ClientForNetworkV2(network)
	if err != nil {
		return nil, errx.Decorate(
			errClientConfig.Wrap(err, "build client for network"),
			reasonNoNetwork,
			"Ensure all nodes share one shard/realm and each endpoint maps to a valid node account",
		)
	}
	hc.SetTransportSecurity(opts.TLS)
	// Default (false) verifies nothing against a published address book, which is
	// what a local network needs; opt in only for public networks.
	hc.SetCertificateVerification(opts.VerifyCertificate)
	hc.SetOperator(payer, payerKey)

	// Fail fast instead of the SDK's 2-minute default stall on an unreachable node.
	hc.SetRequestTimeout(clientRequestTimeout)
	hc.SetGrpcDeadline(clientGrpcDeadline)

	// At -v debug, surface the SDK's own execute/receipt retry loop (logged at
	// TRACE) so a stalled receipt shows each per-attempt node status rather than
	// a single opaque "exceptional precheck status OK". The SDK logger is quiet
	// by default; we only turn it up, never down.
	if strings.EqualFold(opts.LogLevel, "debug") {
		hc.SetLogger(hiero.NewLogger("beacon-sdk", hiero.LoggerLevelTrace))
	}

	fee := opts.MaxTransactionFee
	if fee.AsTinybar() == 0 {
		fee = hiero.NewHbar(defaultMaxTxFeeHbar)
	}
	if err := hc.SetDefaultMaxTransactionFee(fee); err != nil {
		return nil, errx.Decorate(
			errClientConfig.Wrap(err, "set default max transaction fee"),
			reasonClientInit,
			"Provide a valid, positive max transaction fee",
		)
	}

	return &Client{
		hc:      hc,
		execute: submitTransaction,
		getReceipt: func(resp *hiero.TransactionResponse, hc *hiero.Client) (hiero.TransactionReceipt, error) {
			return resp.GetReceipt(hc)
		},
	}, nil
}

// WaitReachable blocks until the target node endpoint is reachable, or timeout
// elapses. It is the preflight every network command runs before submitting a
// transaction, so a command launched (e.g. from a Kubernetes Job) before its
// port-forward/Service is ready waits for connectivity instead of failing on the
// first transaction. A timeout <= 0 disables the wait and returns immediately.
//
// Reachability is checked in two escalating steps per attempt:
//  1. a raw TCP dial to endpoint — cheap, and fails immediately when the port is
//     not being forwarded/served at all (the common local-dev case); then
//  2. a HAPI NetworkVersionQuery — confirms the consensus node itself answers,
//     not merely that something (e.g. HAProxy) bound the port.
//
// Every unsuccessful attempt logs at WARN (with the endpoint, attempt count, and
// next retry delay) so a stalled preflight is visible rather than silent; the
// first success logs once at INFO. On timeout it returns a temporary (retryable)
// error decorated with the last failure and remediation hints.
func (c *Client) WaitReachable(endpoint string, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}

	deadline := time.Now().Add(timeout)
	backoff := probeInitialBackoff
	var lastErr error

	for attempt := 1; ; attempt++ {
		if conn, err := net.DialTimeout("tcp", endpoint, tcpProbeTimeout); err != nil {
			lastErr = err
			logx.As().Warn().Str("reason", reasonNodeUnreachable.String()).
				Str("endpoint", endpoint).Int("attempt", attempt).Dur("retry_in", backoff).
				Err(err).Msg("node endpoint not reachable (TCP) — waiting")
		} else {
			_ = conn.Close()
			// Port is open; confirm the node answers HAPI before we proceed.
			if err := c.probeHAPI(); err != nil {
				lastErr = err
				logx.As().Warn().Str("reason", reasonNodeUnreachable.String()).
					Str("endpoint", endpoint).Int("attempt", attempt).Dur("retry_in", backoff).
					Err(err).Msg("endpoint open but node not answering HAPI yet — waiting")
			} else {
				logx.As().Info().Str("reason", reasonNodeReachable.String()).
					Str("endpoint", endpoint).Int("attempt", attempt).Msg("node reachable")
				return nil
			}
		}

		// Stop once another full backoff would overrun the deadline, so we never
		// sleep past the caller's budget.
		if time.Now().Add(backoff).After(deadline) {
			return errx.WithHints(
				errx.WithReason(
					errNetworkOp.Wrap(lastErr, "node %s not reachable within %s", endpoint, timeout),
					reasonNodeUnreachable,
				),
				"Confirm the node endpoint is reachable (is the kubectl port-forward or Service up?)",
				"Increase --connect-timeout if the network is still starting up",
			)
		}

		time.Sleep(backoff)
		if backoff < probeMaxBackoff {
			if backoff *= 2; backoff > probeMaxBackoff {
				backoff = probeMaxBackoff
			}
		}
	}
}

// probeHAPI runs a NetworkVersionQuery bounded by hapiProbeTimeout, independent of
// the SDK's own request budget, so a single probe cannot stall the WaitReachable
// loop. The query runs in a goroutine; if it outlives the bound it is abandoned
// (it drains on its own within the client's requestTimeout) and reported as a
// timeout so the caller retries.
func (c *Client) probeHAPI() error {
	done := make(chan error, 1)
	go func() {
		_, err := hiero.NewNetworkVersionQuery().Execute(c.hc)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(hapiProbeTimeout):
		return fmt.Errorf("HAPI probe timed out after %s", hapiProbeTimeout)
	}
}

// submitTransaction submits tx and returns at the node's precheck ACK, without
// awaiting a consensus receipt.
//
// It exists because hiero.TransactionExecute special-cases *FileAppendTransaction
// (and *TopicMessageSubmitTransaction) into the SDK's chunked executor, which
// calls GetReceipt — a full consensus-receipt poll (~one poll interval) — after
// every chunk. We drive our own single-chunk appends and confirm them via
// periodic checkpoints + the final hash read-back, so that per-chunk receipt
// wait is pure overhead: it was silently pacing a large upload to one round-trip
// per chunk. For a FileAppend we instead freeze it and call the base
// Transaction.Execute (precheck only, no receipt). All other transactions keep
// the standard path.
func submitTransaction(tx hiero.TransactionInterface, hc *hiero.Client) (hiero.TransactionResponse, error) {
	if fa, ok := tx.(*hiero.FileAppendTransaction); ok {
		if !fa.IsFrozen() {
			if _, err := fa.FreezeWith(hc); err != nil {
				return hiero.TransactionResponse{}, err
			}
		}
		// fa.Transaction is the embedded base *Transaction; its Execute submits
		// without the chunked executor's per-chunk GetReceipt.
		return fa.Transaction.Execute(hc)
	}
	return hiero.TransactionExecute(tx, hc)
}

// Hiero returns the underlying SDK client so operations can build transactions
// and read-back queries against the same network/operator.
func (c *Client) Hiero() *hiero.Client {
	return c.hc
}

// OperatorAccountID returns the payer/operator account the client signs as.
func (c *Client) OperatorAccountID() hiero.AccountID {
	return c.hc.GetOperatorAccountID()
}

// Close releases the client's network connections.
func (c *Client) Close() error {
	return c.hc.Close()
}
