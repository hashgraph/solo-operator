// SPDX-License-Identifier: Apache-2.0

package client

import "github.com/joomcode/errorx"

// client error taxonomy. Each error this package returns is created from one of
// these types and decorated with a reason code + remediation hints via the
// internal/errx helpers, so the base is a real errorx error rather than the
// generic "unexpected non-errorx error" fallback. The errorx.Temporary() trait
// marks conditions a caller may sensibly retry (a submit or receipt round-trip);
// its absence marks a terminal condition (bad client config, or a definitive
// non-SUCCESS network status) that re-running unchanged will not fix.
var (
	errNS = errorx.NewNamespace("beacon.client")

	// errClientConfig: the client could not be built from the given network,
	// payer, or fee — a configuration error, not a network condition. Terminal.
	errClientConfig = errNS.NewType("config")

	// errNetworkOp: submitting the transaction or awaiting its receipt failed at
	// the transport/consensus layer (unreachable node, BUSY, receipt not yet
	// available). Transient — worth a retry once the network settles.
	errNetworkOp = errNS.NewType("network_op", errorx.Temporary())

	// errTxRejected: the network returned a receipt with a definitive non-SUCCESS
	// status. Terminal — the transaction was handled and rejected; resubmitting
	// the same transaction will fail the same way.
	errTxRejected = errNS.NewType("tx_rejected")
)
