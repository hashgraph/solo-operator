# UC Observability Metrics

The Upgrade Controller (UC) sidecar exposes a standard Prometheus scrape
endpoint at `GET /metrics` on port **9990** (configurable via
`UC_METRICS_ADDR`). This endpoint is read-only and accepts no commands;
all coordination stays on the `NetworkUpgrade*` / `ConsensusCapsule` CRs.

## Metrics Reference

### uc_consensus_status

| Field | Value |
|-------|-------|
| Type | Gauge |
| Labels | `source`, `node_id` |
| Values | Platform status code (see table below), or `-1` = UNKNOWN |

The `source` label selects which observation channel produced the value:

- **`metrics`** — passthrough of the CN's `platform_PlatformStatus` from
  the `:9999` Prometheus endpoint, scraped every 5 s.
- **`log`** — last PLATFORM_STATUS transition parsed from the consensus
  log file (`swirlds.log`). Sticky: holds the last observed value between
  transitions and across log rotations. Persisted to disk for restart
  survival. `-1` on cold start until the first transition is observed.
- **`effective`** — reconciled view: uses `metrics` if the last successful
  scrape is within the staleness threshold (default 15 s), otherwise falls
  back to `log`, otherwise `-1`.

#### Platform Status Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | NO_VALUE | No status value set (pre-initialization) |
| 1 | STARTING_UP | Node is starting |
| 2 | ACTIVE | Node is active and participating |
| 4 | BEHIND | Node is behind the network |
| 5 | FREEZING | Freeze in progress |
| 6 | FREEZE_COMPLETE | Freeze completed |
| 7 | REPLAYING_EVENTS | Replaying events after restart |
| 8 | OBSERVING | Node is observing |
| 9 | CHECKING | Node is checking state |
| 10 | RECONNECT_COMPLETE | Reconnect completed |
| 11 | CATASTROPHIC_FAILURE | Fatal failure |
| -1 | UNKNOWN | No data available (UC sentinel, not a platform code) |

Code 3 is not in the platform enum.

### uc_consensus_log_timestamp

| Field | Value |
|-------|-------|
| Type | Gauge |
| Labels | `node_id` |
| Value | Unix seconds (mtime of the consensus log file), 0 if absent |

### uc_platform_last_scrape_timestamp

| Field | Value |
|-------|-------|
| Type | Gauge |
| Labels | `node_id` |
| Value | Unix seconds of the last successful `:9999` scrape, 0 if none |

### Self-Health Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `uc_build_info{version,commit}` | Gauge | Always 1; labels carry version metadata |
| `uc_up` | Gauge | Always 1 when the UC is running |
| `uc_monitor_up{monitor}` | Gauge | 1 = running, 0 = not running, per monitor |

Standard `go_*` and `process_*` collectors are also registered.

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--metrics-addr` | `UC_METRICS_ADDR` | `:9990` | Listen address |
| `--consensus-log-file` | `UC_CONSENSUS_LOG_FILE` | `.../HapiApp2.0/output/swirlds.log` | Log file path |
| `--metrics-staleness` | `UC_METRICS_STALENESS` | `15s` | Staleness threshold for effective source |

## Deployment Contract

The `PLATFORM_STATUS` marker line (from
`com.swirlds.platform.system.status.StatusStateMachine`) must reach the
file configured as `UC_CONSENSUS_LOG_FILE`. Under the shipped log4j2
configuration this is `swirlds.log`. If a custom `Log4j2Config` routes
the marker elsewhere, `source="log"` degrades to `-1` and `effective`
falls back to the metrics source — no hard failure.

## Control-Plane Endpoint

The UC's Unix-socket control plane exposes `GET /consensus/status`, which
returns a compact JSON snapshot of the same data the Prometheus metrics
export:

```json
{
  "nodeId": 0,
  "now": "2026-07-23T10:00:07Z",
  "snapshotAt": "2026-07-23T10:00:07Z",
  "effective": {"code": 2, "status": "ACTIVE", "source": "metrics"},
  "metrics":   {"code": 2, "status": "ACTIVE", "scrapedAt": "2026-07-23T10:00:05Z", "fresh": true},
  "log":       {"code": 2, "status": "ACTIVE", "fileModifiedAt": "2026-07-23T10:00:06Z"},
  "staleness": "15s"
}
```

**This endpoint reads the last-known monitor state — it does not trigger a
fresh scrape.** Data freshness depends on each monitor's poll interval
(PlatformStatusMonitor ≈ 5 s, LogMonitor ≈ 1 s). The `fresh` field
indicates whether the metrics scrape is within the staleness window.

Example usage via `kubectl exec`:

```bash
kubectl exec <pod> -c uc -- curl --unix-socket /run/uc/uc.sock http://localhost/consensus/status
```

## Kubernetes Exposure

The operator adds:
- A `uc-metrics` container port (9990) on the UC sidecar
- A `uc-metrics` service port (9990) on the consensus pod Service
- An optional **ServiceMonitor** gated by
  `uc.metrics.serviceMonitor.enabled` (default `false`) in the Helm chart

The ServiceMonitor selects consensus pod Services by the standard
`app.kubernetes.io/managed-by: solo-operator` and
`app.kubernetes.io/part-of: consensus-node` labels.
