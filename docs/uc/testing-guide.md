# UC Sidecar — Manual Testing Guide

This guide walks a human operator through exercising the Upgrade Controller (UC)
sidecar locally and observing its behaviour: the marker-file polling loop, the
console logs, the JSONL event log, and the control-socket status/version
endpoints.

It targets the foundational polling loop delivered in **story 8.1** (#991) plus
the persist-before-send primitive from **story 8.2** (#992). The per-marker CR
handlers (#993–#996), dedup (#997), and cleanup/recovery (#998) are stubbed for
now — when you drop a marker you will see the loop detect it and log a "handler
not yet implemented" line rather than a created CR, so the `--cr-persist-path`
directory stays empty in an end-to-end run today. §8 below shows how to exercise
the persistence primitive directly via its unit tests until the handlers wire it
in.

---

## 1. Build the binary

```bash
task build:uc          # -> bin/uc, with version metadata stamped via -ldflags
```

Confirm the version metadata was injected:

```bash
./bin/uc version            # human-readable text (default)
./bin/uc version -o json    # JSON (version, commit, date, goVersion)
./bin/uc --version          # shorthand, prints text and exits
```

You should see a real `version`/`commit`/`date` (not `dev`/`none`/`unknown`),
proving the `task build:uc` ldflags wiring works. A plain `go build` without
ldflags will show the `dev` placeholders — that is expected.

---

## 2. Prepare the cluster (install the CRDs)

The UC creates and lists `NetworkUpgrade*` Custom Resources, so their CRDs must
exist in the target cluster. If they are missing you will see the sidecar retry
forever with:

```
WRN list error — retrying error="list NetworkUpgradeExecute: the server could not find the requested resource" reason=ProvisionerProxyListError
```

Install the CRDs (idempotent; uses server-side apply):

```bash
task install            # kubectl apply --server-side -f config/crd/bases
```

Verify the four CRDs are registered:

```bash
kubectl api-resources --api-group=operator.solo.hedera.com | grep -i networkupgrade
# Expect: networkupgradeprepares, networkupgradefreezes,
#         networkupgradefreezeaborts, networkupgradeexecutes
kubectl get networkupgradeexecutes -A    # should return "No resources found", not an error
```

Also create the namespace the UC targets (`--namespace`, below) if it does not
already exist:

```bash
kubectl create namespace solo --dry-run=client -o yaml | kubectl apply -f -
```

To remove the CRDs afterwards: `task uninstall`.

---

## 3. Run the sidecar against scratch directories

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

## 4. Test marker detection and dispatch

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

## 5. Test the freeze-window interval switch

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

## 6. Inspect the JSONL event log

The UC writes a structured audit log (story 9.1) under `--events-path`, one
`upgrade-<operationId>.jsonl` file per upgrade.

> **At story 8.1 the events directory stays empty — this is expected, not a
> bug.** A `.jsonl` file is created only when a per-marker handler runs, and the
> handlers are still stubs (#993+). On startup the monitor calls only
> `events.Prune`, which *retains/deletes existing* logs to enforce the retention
> limit — it never creates one. So with a fresh scratch directory there is
> nothing to see yet. Dropping a marker (§4) logs `MarkerHandlerPending` to the
> console but writes no event file.

Once the handlers land, this is how you would inspect the log:

```bash
ls -la "$UCDIR/events"
# Tail an upgrade log as JSON lines:
cat "$UCDIR/events"/upgrade-*.jsonl 2>/dev/null | jq .
```

Each line is a self-contained JSON object (timestamp, level, reason, message,
and upgrade fields).

To exercise the writer/retention logic today, run its unit tests instead:
`go test ./internal/uc/events/ -v`.

---

## 7. Query the control socket (status & health)

The UC exposes a daemonkit control server on `--socket-path` with two endpoints,
`GET /status` and `GET /health`. The `uc` binary dials them for you:

```bash
./bin/uc status --socket-path "$UCDIR/uc.sock"   # pretty-printed per-monitor status JSON
./bin/uc health --socket-path "$UCDIR/uc.sock"   # prints "ok" + exit 0 while live, errors otherwise
```

`status` returns one entry per monitor (`marker-monitor`, and
`provisioner-proxy-monitor` in cluster-only mode), showing each monitor's
liveness and any connectivity error. `health` is a scriptable liveness probe.

Prefer raw `curl`? The same endpoints are available directly:

```bash
curl --unix-socket "$UCDIR/uc.sock" http://uc/status | jq .
curl --unix-socket "$UCDIR/uc.sock" -i http://uc/health   # 200 when the process is live
```

---

## 8. CR persistence (persist-before-send, #992)

For crash safety the factory serializes every constructed `NetworkUpgrade*` CR to
YAML under `--cr-persist-path` (`$UCDIR/crs` in this guide) **before** the K8s API
`Create`, and removes the file only **after** the create succeeds. A file left on
disk therefore marks the crash window between persist and send: on restart it is
re-read and handed to the recovery flow (#998) to re-send.

On-disk format:

- One file per CR, named deterministically `<operationId>.<step>.yaml`, where
  `<step>` is one of `prepare`, `freeze`, `execute`, `abort`. The name lets
  recovery correlate a file with its operation and the marker that produced it
  without parsing the body — e.g. `upgrade-v0.76.0-20260415T143000Z.prepare.yaml`.
- The body is the full CR serialized as YAML; it round-trips losslessly
  (marshal → write → read → unmarshal reproduces the object).
- Writes are atomic (temp file + rename), so a crash mid-write never leaves a
  truncated CR that recovery would try to re-send.

> **End-to-end today the `crs/` directory stays empty** — the marker handlers
> (#993+) are the callers of `Persist`/`Delete`, and they are still stubs. So a
> live run (§3–§5) creates no files here yet. Until the handlers land, exercise
> the primitive and its on-disk contract directly via its unit tests:

```bash
go test ./internal/uc/ -run TestCRStore -v
```

| Test | Asserts |
|------|---------|
| `TestCRStorePersistThenDelete` | A CR file appears under `--cr-persist-path` on `Persist`, named `<operationId>.<step>.yaml`, and is gone after `Delete` (the success path). |
| `TestCRStoreDeleteMissingIsNotError` | Deleting an already-absent file is a no-op, not an error (safe after a recovery re-send). |
| `TestCRStoreRoundTripsLosslessly` | `Persist` → `List` reproduces the CR object exactly and recovers `operationId`/`step` from the filename. |
| `TestCRStoreCrashLeftoverIsRecoverable` | CRs left on disk (simulated crash before the create ack) are all detected and decoded on the next `List` scan. |
| `TestCRStorePersistIsIdempotent` | Re-persisting the same `operationId`+`step` overwrites in place — one file, no error (re-detection of the same marker must not fail). |
| `TestCRStoreListIgnoresForeignAndPartialFiles` | `List` skips `.tmp` leftovers, foreign files, and names with an unknown step, returning only well-formed CRs. |
| `TestCRStoreListMissingDirIsEmpty` | A never-created persist dir yields no entries and no error. |
| `TestCRStoreRejectsBadInput` | A path-traversal `operationId`, an unknown step, or a nil CR is rejected at the boundary. |

Once the handlers wire it in, you will be able to watch the lifecycle live:
drop a marker (§4), then in a tight loop `ls "$UCDIR/crs"` to catch the
`<operationId>.<step>.yaml` file appear before the CR shows up in
`kubectl get networkupgrade...` and disappear once the create is acknowledged.

---

## 9. Clean up

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
go test ./internal/uc/ -run TestControlClient -v
```

| Test | Asserts |
|------|---------|
| `TestControlClientStatusPretty` | `Status` fetches `GET /status` and pretty-prints the JSON body (backs `uc status`). |
| `TestControlClientHealth` | `Health` succeeds against a live `GET /health` (backs `uc health`). |
| `TestControlClientNon200` | A non-200 response on either endpoint surfaces as an error. |
| `TestControlClientNoSocket` | An absent socket surfaces as a dial error on either endpoint. |

The `ControlClient` covers the full daemonkit control surface (`/status` and
`/health`); add a method (and `uc` subcommand) here for any new endpoint the
server registers.

Build-time version metadata is now provided by the shared, dependency-free
[`github.com/automa-saga/version`](https://github.com/automa-saga/version)
package (stamped via `-ldflags -X`); it carries its own tests, so there is no
in-repo version suite. `uc version` renders `text` (default) or `json`.
