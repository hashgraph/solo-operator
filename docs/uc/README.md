# UC — Upgrade Controller sidecar

`uc` (`cmd/uc/`) is a sidecar that runs alongside each Hedera consensus node. It
polls the marker-file directory written by the consensus node and creates the
`NetworkUpgrade*` Custom Resources that drive the network upgrade flow. In
cluster-only mode it also advances the `Execute` CR from
`ReadyForProvisionerDaemon` to `PendingNodeUpgrade`.

It is released in lockstep with the operator as the
`ghcr.io/hashgraph/solo-operator-uc` image (see the repository
[README](../../README.md#components)).

## Commands

| Command | Purpose |
|---------|---------|
| `uc` | Run the sidecar (marker polling + CR creation) |
| `uc status` | Query the running sidecar's status over its control socket |
| `uc health` | Probe the running sidecar's liveness over its control socket |
| `uc version` | Print version information and exit |

## Configuration

The sidecar is configured via flags or `UC_`-prefixed environment variables
(precedence: CLI flag > env var > default; e.g. `--node-id` ↔ `UC_NODE_ID`).
Run `uc --help` for the full, authoritative flag list. Key flags:

| Flag | Description |
|------|-------------|
| `--mode` | Deployment mode: `mainnet` or `cluster-only` |
| `--node-id` | Consensus node identity (non-negative integer) |
| `--orbit` | Deployment orbit the upgrade CRs target |
| `--namespace` | Namespace to watch and create CRs in |
| `--pod-name` | UC pod name (downward API); the ConsensusCapsule name is derived from it |
| `--upgrade-path` | Directory polled for marker files |
| `--metrics-addr` | Listen address for the `/metrics` endpoint |
| `--socket-path` | Unix socket for the control server (used by `uc status`/`uc health`) |

## Further reading

- [Testing guide](testing-guide.md)
- [Test scenarios](test-scenarios.md)
- [Observability — metrics](observability/metrics.md), plus the
  [alert rules](observability/alert-rules.yaml) and
  [Grafana dashboard](observability/grafana-dashboard.json)
