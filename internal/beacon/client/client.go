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
	"strings"

	"github.com/automa-saga/errx"
	hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
)

// defaultMaxTxFeeHbar is the generous flat max transaction fee applied when the
// caller does not set one, mirroring yahcli: beacon does no exact fee
// estimation, it just caps the fee well above what any upgrade/freeze/file
// operation costs.
const defaultMaxTxFeeHbar = 100

// Options tunes the underlying hiero.Client. The zero value targets a local,
// self-contained network (Solo/kind): plaintext transport, no certificate
// verification, and the default max transaction fee. It is not specific to
// mainnet/testnet — any network map is accepted.
type Options struct {
	// TLS selects TLS HAPI transport (node endpoints on :50212). Local networks
	// typically use plaintext; leave false.
	TLS bool
	// VerifyCertificate turns on node-certificate verification against the
	// network's address book. It defaults to false so TLS works against a local
	// network whose node certs are not in a published address book; enable it
	// only for public networks (mainnet/testnet/previewnet).
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
		execute: hiero.TransactionExecute,
		getReceipt: func(resp *hiero.TransactionResponse, hc *hiero.Client) (hiero.TransactionReceipt, error) {
			return resp.GetReceipt(hc)
		},
	}, nil
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
