# UC Sidecar — Manual Testing Guide

This guide walks a human operator through exercising the Upgrade Controller (UC)
sidecar locally and observing its behaviour: the marker-file polling loop, the
console logs, the JSONL event log, and the control-socket status/version
endpoints.

It targets the foundational polling loop delivered in **story 8.1** (#991). The
per-marker CR handlers (#993–#996), persistence (#992), dedup (#997), and
cleanup (#998) are stubbed for now — when you drop a marker you will see the loop
detect it and log a "handler not yet implemented" line rather than a created CR.

---

## 1. Build the binary

```bash
task build:uc          # -> bin/uc, with version metadata stamped via -ldflags
```

Confirm the version metadata was injected:

```bash
./bin/uc version            # human-readable text
./bin/uc version -o json    # JSON (version, commit, date, goVersion)
./bin/uc version -o yaml    # YAML
./bin/uc --version          # shorthand, prints text and exits
```

You should see a real `version`/`commit`/`date` (not `dev`/`none`/`unknown`),
proving the `task build:uc` ldflags wiring works. A plain `go build` without
ldflags will show the `dev` placeholders — that is expected.

---

## 2. Run the sidecar against scratch directories

The UC defaults point at production host paths under `/opt/...`. For local
testing, point every path at a scratch directory so nothing requires root:

```bash
export UCDIR=$(mktemp -d)
mkdir -p "$UCDIR/markers" "$UCDIR/events" "$UCDIR/crs" "$UCDIR/backup"

./bin/uc \
  --mode cluster-only \
  --node-id 0 \
  --orbit my-orbit \
  --namespace solo \
  --kubeconfig "$HOME/.kube/config" \
  --marker-path  "$UCDIR/markers" \
  --events-path  "$UCDIR/events" \
  --cr-persist-path "$UCDIR/crs" \
  --backup-path  "$UCDIR/backup" \
  --socket-path  "$UCDIR/uc.sock" \
  --poll-interval 5s \
  --freeze-poll-interval 1s \
  --log-level debug
```

Notes:
- A reachable `--kubeconfig` (or in-cluster config) is required — `uc` builds a
  dynamic client at startup. For a pure loop-only smoke test, any valid
  kubeconfig works; the stubbed handlers do not create CRs yet.
- `--poll-interval 5s` shortens the normal 60s cadence so you don't have to wait
  a minute between ticks while testing.
- Every flag can also be set via a `UC_`-prefixed env var, e.g.
  `UC_MARKER_PATH`, `UC_POLL_INTERVAL`. CLI flag > env var > default.

On startup you should see a `MarkerMonitorStarted` log line with the marker path
and poll interval.

---

## 3. Test marker detection and dispatch

In a second terminal, drop marker files into the marker directory. The loop
re-scans every poll interval and dispatches each marker it finds.

```bash
# Triggers the NetworkUpgradePrepare handler (#993, stubbed)
touch "$UCDIR/markers/execute_immediate.mf"
```

Watch the first terminal for (in `debug`):
- a `MarkerHandlerPending` line naming `execute_immediate.mf`.

Markers are **not** deleted on detection (cleanup is #998), so you will see the
same marker re-dispatched on every tick. That is by design — repeat-work
suppression is the dedup cache's job (#997), not the poller's.

The four recognised markers and what each routes to:

| Marker file            | Routed handler (story)           |
|------------------------|----------------------------------|
| `execute_immediate.mf` | `HandleExecuteImmediate` (#993)  |
| `freeze_scheduled.mf`  | `HandleFreezeScheduled` (#994)   |
| `now_frozen.mf`        | `HandleNowFrozen` (#995)         |
| `freeze_aborted.mf`    | `HandleFreezeAborted` (#996)     |

---

## 4. Test the freeze-window interval switch

The loop polls at the normal cadence (`--poll-interval`) and drops to the fast
cadence (`--freeze-poll-interval`, default 1s) during the freeze window to
minimise `now_frozen.mf` detection latency.

```bash
# 1. Enter the freeze window — interval drops to freeze-poll-interval
touch "$UCDIR/markers/freeze_scheduled.mf"
```

Look for a `PollIntervalChanged` log line with `from=5s to=1s`.

```bash
# 2. End the freeze window — interval reverts to poll-interval
touch "$UCDIR/markers/now_frozen.mf"      # OR freeze_aborted.mf
```

Look for a `PollIntervalChanged` line with `from=1s to=5s`.

(`freeze_aborted.mf` reverts the cadence the same way as `now_frozen.mf`.)

---

## 5. Inspect the JSONL event log

The UC writes a structured audit log (story 9.1) under `--events-path`. On
startup it prunes old `upgrade-*.jsonl` files to the retention limit; per-upgrade
event lines are appended as handlers are completed in later stories.

```bash
ls -la "$UCDIR/events"
# Tail any upgrade log as JSON lines:
cat "$UCDIR/events"/upgrade-*.jsonl 2>/dev/null | jq .
```

Each line is a self-contained JSON object (timestamp, level, reason, message,
and upgrade fields). If no upgrade has produced events yet, the directory may be
empty — that is expected at the 8.1 stage.

---

## 6. Query the control socket (status & health)

The UC exposes a daemonkit control server on `--socket-path`. The `uc status`
subcommand dials it for you:

```bash
./bin/uc status --socket-path "$UCDIR/uc.sock"
```

You should get pretty-printed JSON with one entry per monitor
(`marker-monitor`, and `provisioner-proxy-monitor` in cluster-only mode),
showing each monitor's liveness and any connectivity error.

Prefer raw `curl`? The same endpoints are available directly:

```bash
curl --unix-socket "$UCDIR/uc.sock" http://uc/status | jq .
curl --unix-socket "$UCDIR/uc.sock" -i http://uc/health   # 200 when the process is live
```

---

## 7. Clean up

```bash
# Ctrl-C the sidecar (it shuts down on SIGINT/SIGTERM), then:
rm -rf "$UCDIR"
```

---

## Automated coverage

The behaviours above are also covered by Go tests.

**Marker poller** (`internal/uc/marker_monitor_test.go`):

```bash
go test ./internal/uc/ -run 'TestPoll|TestRun' -v
```

| Test | Asserts |
|------|---------|
| `TestPollDispatchesEachMarker` | Each of the four markers routes to its matching handler exactly once; unrelated files are ignored. |
| `TestPollIntervalTransitions` | Cadence switches 60s→1s on `freeze_scheduled.mf` and reverts 1s→60s on `now_frozen.mf` / `freeze_aborted.mf`; `execute_immediate.mf` and an empty directory leave the cadence unchanged. |
| `TestPollReDispatchesPersistentMarker` | A persistent marker is re-dispatched every tick (cleanup is #998); repeat-work suppression is the dedup cache's job (#997), not the poller's. |
| `TestPollContinuesAfterHandlerError` | A failing handler does not abort the scan — remaining markers in the same tick are still detected and still drive the interval transition. |
| `TestPollMissingDirectoryIsTolerated` | A missing marker directory yields no dispatches and no panic. |
| `TestRunStopsOnContextCancel` | `Run` returns cleanly on context cancellation instead of blocking on the poll timer. |

The tests inject a fake `markerDispatcher` so the loop runs without a Kubernetes
client; the per-marker CR construction is covered by #993–#996. Detection is
intentionally idempotent-friendly: the loop does not assume a marker fires only
once, and no `operationId` is minted at this layer.

**Control socket** (`internal/uc/control_client_test.go`):

```bash
go test ./internal/uc/ -run TestFetchStatus -v
```

| Test | Asserts |
|------|---------|
| `TestFetchStatusPrettyPrints` | `uc status` dials the Unix socket, fetches `GET /status`, and pretty-prints the JSON body. |
| `TestFetchStatusNon200` / `TestFetchStatusNoSocket` | A non-200 response and an absent socket both surface as errors. |

Build-time version metadata is now provided by the shared, dependency-free
[`github.com/automa-saga/version`](https://github.com/automa-saga/version)
package (stamped via `-ldflags -X`); it carries its own tests, so there is no
in-repo version suite. `uc version` renders `text` (default) or `json`.
