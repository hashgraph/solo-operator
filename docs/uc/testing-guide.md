# UC Sidecar — Manual Testing Guide

This guide walks a human operator through exercising the Upgrade Controller (UC)
sidecar locally and observing its behaviour: the marker-file polling loop, the
console logs, the JSONL event log, and the control-socket status/version
endpoints.

It targets the foundational polling loop delivered in **story 8.1** (#991), the
persist-before-send primitive from **story 8.2** (#992), the
`execute_immediate.mf` → `NetworkUpgradePrepare` handler from **story 8.3**
(#993), the `freeze_scheduled.mf` → `NetworkUpgradeFreeze` handler from **story
8.4** (#994), the `freeze_aborted.mf` → `NetworkUpgradeFreezeAbort` handler from
**story 8.6** (#996), and the operation-id deduplication from **story 8.7**
(#997). When you drop `execute_immediate.mf` you will now see the loop construct,
persist, and create a real `NetworkUpgradePrepare` CR (§4a); dropping
`freeze_scheduled.mf` (with a valid epoch-seconds content) creates a
`NetworkUpgradeFreeze` CR (§4b); dropping `freeze_aborted.mf` creates a
`NetworkUpgradeFreezeAbort` CR (§4c). The remaining per-marker handler (#995
`now_frozen.mf`) and cleanup/recovery (#998) are still stubbed.

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

> **Kubernetes ≥ 1.32 required.** The UC dedup query filters `NetworkUpgrade*`
> CRs **server-side** on the `spec.operationId` `selectableField`, which is GA in
> 1.32 (`kubeVersion: ">=1.32.0-0"` in `Chart.yaml`). On an older cluster the
> CRDs install but the field-selector List is rejected by the API server.

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
touch "$UCDIR/markers/freeze_scheduled.mf"
```

Watch the first terminal for (in `debug`):
- a `CRDelivered` line for a `NetworkUpgradeFreeze` CR, or
- a `FreezeWithoutPrepare` warning if no non-terminal `NetworkUpgradePrepare` exists yet.

Markers are **not** deleted on detection (cleanup is #998), so you will see the
same marker re-dispatched on every tick. That is by design — repeat-work
suppression is the dedup cache's job (#997), not the poller's.

The four recognised markers and what each routes to:

| Marker file            | Routed handler (story)                | Status         |
|------------------------|----------------------------------------|----------------|
| `execute_immediate.mf` | `HandleExecuteImmediate` (#993)        | **implemented** — see §4a |
| `freeze_scheduled.mf`  | `HandleFreezeScheduled` (#994)         | **implemented** — see §4b |
| `now_frozen.mf`        | `HandleNowFrozen` (#995)               | stubbed        |
| `freeze_aborted.mf`    | `HandleFreezeAborted` (#996)           | **implemented** — see §4b |

---

## 4a. Create a NetworkUpgradePrepare CR (`execute_immediate.mf`, #993)

`execute_immediate.mf` is the first marker the consensus node writes when it
extracts an upgrade package. The CN unzips the archive **into the marker
directory itself** and writes the marker there too, so the package manifests
live under `<marker-path>/manifests/`. The handler reads
`manifests/consensus-node-components.yaml`, mints an `operationId`, and creates a
`NetworkUpgradePrepare` CR.

You can populate the marker directory two ways.

**Option A — build a realistic `build.zip` (recommended).** The `test/dev`
Taskfile builds the same deployment package the consensus node extracts,
including `manifests/consensus-node-components.yaml` with **real deterministic
layer hashes** (the RootFS `diff_ids` of the published consensus-node image, so
the operator's deterministic verification passes). Requires `crane` + `jq`
(`go install github.com/google/go-containerregistry/cmd/crane@latest`).

```bash
( cd test/dev && task setup && task build )   # -> test/dev/staging/build-v<ver>.zip

# Extract it into the marker dir, mirroring what the CN does on-node:
unzip -q test/dev/staging/build-v*.zip -d "$UCDIR/markers"
ls "$UCDIR/markers/manifests/consensus-node-components.yaml"   # present

# Now trigger the handler
touch "$UCDIR/markers/execute_immediate.mf"
```

See [test/dev/README.md](../../test/dev/README.md) for build options (version,
image, platforms) and the `overlay:update` task.

**Option B — hand-write a minimal manifest (quick smoke test).** Lay out just
the manifest the handler reads. The layer hash below is a placeholder — fine for
exercising the UC parse/create path, but it will not pass real deterministic
verification (use Option A for that):

```bash
mkdir -p "$UCDIR/markers/manifests"
cat > "$UCDIR/markers/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML

# Now trigger the handler
touch "$UCDIR/markers/execute_immediate.mf"
```

Watch the first terminal for a `CRDelivered` log line with the `operation_id`
and `cr_name`, then confirm the CR exists in the cluster:

```bash
kubectl get networkupgradeprepares -n solo
kubectl get networkupgradeprepares -n solo -o yaml | less
```

You should see a CR named `upgrade-0.75.0-<marker-time-lowercased>-prepare` with:
- labels `operator.solo.hedera.com/operation-id: upgrade-0.75.0-<marker-time>`
  and `operator.solo.hedera.com/step: prepare`,
- annotation `operator.solo.hedera.com/created-by: UC`,
- `spec.orbit` = your `--orbit`, `spec.operationId`, and `spec.images` copied
  verbatim from the manifest.

The `operationId` is **minted from the marker's creation time + package
version**, so it is stable across re-detections: on every subsequent tick the
handler recomputes the same name and the create is a safe no-op. Re-running the
`touch` updates the marker mtime and therefore mints a *new* operationId; use a
fresh `$UCDIR` (or leave the marker in place) to observe stable behaviour.

**Deduplication (#997).** The marker stays on disk (cleanup is #998), so the
handler re-sees it every tick. Two layers stop a duplicate create: an
authoritative cluster query for a CR with the same `spec.operationId`
(filtered **server-side** via the CRD `selectableField` on that field — see the
k8s 1.32 note in §2), fronted by an in-memory cache so steady-state ticks skip
the API call entirely. (`spec.operationId` is the source of truth; the
`operator.solo.hedera.com/operation-id` label is an auxiliary convenience.)
After the first `CRDelivered` you
will see **no further** `kubectl`-visible activity for that marker — the second
and later ticks are served from the cache and never reach the API server. On
restart, `RecoverOnStartup` re-seeds the cache by listing existing
`NetworkUpgrade*` CRs, so a restarted sidecar does not re-create a CR it already
delivered. The cache is a hint only: any K8s API error drops the entry and the
next tick re-queries, and the create's own `AlreadyExists` check is the ultimate
guard against a race.

**Invalid manifest:** set `schemaVersion: 99` (or remove
`images.consensusNode.version`) and re-run — the handler logs a `ManifestInvalid`
error line and creates **no** CR, rather than retrying forever.

---

## 4b. Schedule a freeze (`freeze_scheduled.mf`, #994)

`freeze_scheduled.mf` is written by the consensus node once it has submitted a
`FREEZE_UPGRADE` transaction. The file **content** is the scheduled freeze time
as decimal seconds since Unix epoch (e.g. `1744603200`). The handler creates a
`NetworkUpgradeFreeze` CR, copying `spec.images` and `status.selectedSources`
from the existing `NetworkUpgradePrepare` CR so the Freeze CR is self-contained.

**`operationId` re-derivation.** The handler re-derives the `operationId`
deterministically from the co-present `execute_immediate.mf` mtime and the
`consensus-node-components.yaml` manifest — the same inputs as
`HandleExecuteImmediate`, yielding the same value every tick.

**`spec.scheduledTime` comes from the file content**, not the marker's mtime.
The CN writes the freeze-transaction time as epoch seconds into the file; the
handler parses that and stores it in the CR.

**Gate: the `NetworkUpgradePrepare` CR must be in `Succeeded` phase** before a
Freeze CR is created. If the Prepare is still in-progress, the handler retries
silently on the next tick (DEBUG log). If the Prepare has `Failed`, the handler
halts (ERROR log, no retry).

**Staleness guard.** The handler compares the marker's mtime against two
references and skips (WARN log) if either check fails:
1. `freeze_scheduled.mf` mtime must be strictly newer than `execute_immediate.mf`
   mtime — if not, the marker is stale (leftover from a prior upgrade cycle while
   the CN has not yet cleaned up).
2. After resolving the Prepare CR, the marker mtime must also be newer than the
   Prepare CR's `creationTimestamp` — if not, it is also stale.

**Fallback when `execute_immediate.mf` is absent.** If the prepare-package marker
is missing, the handler logs `FreezeWithoutPrepare` at WARN and attempts to infer
the `operationId` from the manifest version alone (cluster + disk Prepare lookup
with the same version prefix). If all fallbacks are exhausted, the handler logs
`OperationIDExhausted` at ERROR and waits — delete `freeze_scheduled.mf` to
terminate the retry loop.

### Writing the marker correctly

The file must contain epoch seconds, **not** an empty file or arbitrary string.
With the Prepare CR from §4a in `Succeeded` phase:

```bash
# Write current time (seconds since epoch) as the freeze-scheduled time.
echo -n "$(date +%s)" > "$UCDIR/markers/freeze_scheduled.mf"
```

Watch for a `CRDelivered` log line, then confirm the CR:

```bash
kubectl get networkupgradefreezes -n solo          # short name: nuf
kubectl get networkupgradefreezes -n solo -o yaml | less
```

You should see a CR named `upgrade-0.75.0-<prepare-time-lowercased>-freeze` with:
- labels `operator.solo.hedera.com/operation-id` (matching the Prepare) and
  `operator.solo.hedera.com/step: freeze`,
- annotation `operator.solo.hedera.com/created-by: UC`,
- `spec.orbit`, `spec.operationId`, and `spec.scheduledTime` (from the epoch
  seconds in the file, converted to RFC3339),
- `spec.images` copied verbatim from the Prepare CR's `spec.images`,
- `spec.selectedSources` copied from `status.selectedSources` of the Prepare CR.

### Exercising the staleness guard manually

To confirm the guard works, create a stale marker (older than `execute_immediate.mf`):

```bash
# Set mtime to one hour in the past (before execute_immediate.mf was written).
STALE_TIME=$(date -v-1H +%s 2>/dev/null || date -d '-1 hour' +%s)
echo -n "$STALE_TIME" > "$UCDIR/markers/freeze_scheduled.mf"
touch -t "$(date -d @$STALE_TIME '+%Y%m%d%H%M.%S' 2>/dev/null || date -r $STALE_TIME '+%Y%m%d%H%M.%S')" \
  "$UCDIR/markers/freeze_scheduled.mf"
```

Watch for a `StaleMarker` WARN log: `"stale freeze_scheduled.mf — marker predates
current execute_immediate.mf; skipping until refreshed"`. No Freeze CR will be
created. Overwrite the marker with a fresh epoch time to resume:

```bash
echo -n "$(date +%s)" > "$UCDIR/markers/freeze_scheduled.mf"
```

### Exercising the fallback path manually

Remove `execute_immediate.mf` while `freeze_scheduled.mf` is present to trigger
the fallback:

```bash
rm "$UCDIR/markers/execute_immediate.mf"
```

Watch for `FreezeWithoutPrepare` WARN: `"execute_immediate.mf missing — attempting
fallback resolution via manifest version"`. If a Prepare CR with the matching
version prefix exists in the cluster, the handler resolves the `operationId` from
it and creates the Freeze CR. If no match is found, watch for
`OperationIDExhausted` ERROR.

### Other anomalies

**Prepare has Failed:** the handler logs `PrepareFailed` at ERROR and creates no
Freeze CR. This is a halt — delete `freeze_scheduled.mf` to clear.

**Invalid file content:** if the file contains non-numeric bytes, the handler logs
`ManifestInvalid` at ERROR and halts (no retry). This is defensive — the CN
always writes valid epoch seconds.

Processing this marker switches the poll cadence to `--freeze-poll-interval`
(§5).

---

## 4c. Cancel an upgrade (`freeze_aborted.mf`, #996)

`freeze_aborted.mf` is written when a `FREEZE_ABORT` transaction cancels a
pending upgrade (before the scheduled freeze time). The handler creates a
`NetworkUpgradeFreezeAbort` CR and takes no further action — the operator's
`FreezeAbortReconciler` owns the abort logic.

**`operationId` re-derivation (no phase gate).** The abort re-derives the
`operationId` deterministically from the co-present `execute_immediate.mf` mtime
and manifest — the same inputs as prepare. Unlike `freeze_scheduled.mf`, **no
Prepare CR gate applies**: a cancellation must be deliverable regardless of the
state of any prior CRs; the FreezeAbortReconciler handles state checks. The UC
also does not copy `spec.images` or `status.selectedSources` from the Prepare CR —
the abort CR carries only identity (`orbit` + `operationId`).

**Staleness guard.** The handler compares the abort marker's mtime against
`execute_immediate.mf` mtime and skips (WARN log) if the marker is older — a
stale `freeze_aborted.mf` leftover from a prior upgrade cycle will not latch onto
the next upgrade's identity if a new `execute_immediate.mf` lands.

**Fallback when `execute_immediate.mf` is absent.** The handler logs
`AbortWithoutPrepare` at WARN and attempts fallback resolution via manifest version
(cluster + disk Prepare lookup, non-Failed phases). If all fallbacks are
exhausted, it logs `OperationIDExhausted` at ERROR — delete `freeze_aborted.mf`
to terminate the retry loop.

With `execute_immediate.mf` and the manifest in place (from §4a, or fresh from
Option B):

```bash
touch "$UCDIR/markers/freeze_aborted.mf"
```

Watch for a `CRDelivered` line, then confirm the CR:

```bash
kubectl get networkupgradefreezeaborts -n solo   # short name: nua
kubectl get networkupgradefreezeaborts -n solo -o yaml | less
```

You should see a CR named `upgrade-0.75.0-<prepare-time-lowercased>-abort` —
**the same operationId as the Prepare CR** (re-derived from the same marker) —
with the `-abort` step suffix, and:
- labels `operator.solo.hedera.com/operation-id` (matching the Prepare) and
  `operator.solo.hedera.com/step: abort`,
- annotation `operator.solo.hedera.com/created-by: UC`,
- `spec.orbit` + `spec.operationId`; **no** `images` block (abort carries only
  identity).

Confirm the **audit trail is preserved** — the abort does not delete sibling CRs:

```bash
kubectl get networkupgradeprepares -n solo   # the Prepare CR is still there
```

### Exercising the staleness guard manually

```bash
# Write abort marker with mtime in the past (before execute_immediate.mf).
STALE_TIME=$(date -v-1H +%s 2>/dev/null || date -d '-1 hour' +%s)
touch "$UCDIR/markers/freeze_aborted.mf"
touch -t "$(date -d @$STALE_TIME '+%Y%m%d%H%M.%S' 2>/dev/null || \
  date -r $STALE_TIME '+%Y%m%d%H%M.%S')" "$UCDIR/markers/freeze_aborted.mf"
```

Watch for `StaleMarker` WARN: `"stale freeze_aborted.mf — marker predates current
execute_immediate.mf; skipping until refreshed"`. No Abort CR is created.

### Anomaly: `execute_immediate.mf` absent, no fallback

Drop `freeze_aborted.mf` into a fresh `$UCDIR` with no `execute_immediate.mf` and
no manifest — all fallbacks exhausted:

```bash
mkdir -p "$UCDIR2/markers"
touch "$UCDIR2/markers/freeze_aborted.mf"
# run UC pointing at $UCDIR2 — watch for OperationIDExhausted ERROR log
```

The handler logs `AbortWithoutPrepare` (WARN) then `OperationIDExhausted` (ERROR)
and creates no CR. Delete `freeze_aborted.mf` or add `execute_immediate.mf` to
resume.

Processing this marker also reverts the poll cadence to `--poll-interval` (§5).

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

> The `execute_immediate.mf` handler (§4a, #993) now drives this primitive: it
> writes `<operationId>.prepare.yaml` here **before** the create and removes it
> **after** the create is acknowledged. The window is brief, so under normal
> operation you will usually catch the file only mid-create (see the live-watch
> tip below) or after an injected failure. The remaining markers (#994–#996) are
> still stubs, so their CR files do not appear yet. Exercise the on-disk contract
> directly via its unit tests:

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

Watch the lifecycle live: in a tight loop print the persist dir while you drop
`execute_immediate.mf` (§4a), to catch the `<operationId>.prepare.yaml` file
appear before the CR shows up in `kubectl get networkupgradeprepares` and
disappear once the create is acknowledged:

```bash
while true; do ls "$UCDIR/crs"; sleep 0.2; done
```

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

**Prepare handler & manifest parser** (`internal/uc/prepare_test.go`,
`internal/uc/manifest.go`):

```bash
go test ./internal/uc/ -run 'TestParseComponents|TestHandleExecuteImmediate' -v
```

| Test | Asserts |
|------|---------|
| `TestParseComponentsManifestValid` | A valid `consensus-node-components.yaml` parses into `UpgradeImagesSpec` (consensus node + sidecars). |
| `TestParseComponentsManifestLenientUnknownField` | An additive unknown field within a known `schemaVersion` is ignored, not rejected (lenient parsing). |
| `TestParseComponentsManifestInvalid` | Unknown/missing `schemaVersion`, missing `consensusNode.version`, and malformed YAML all map to `ManifestInvalid`. |
| `TestHandleExecuteImmediateCreatesPrepareCR` | Builds + creates a `NetworkUpgradePrepare` with the correct name, labels, `created-by` annotation, `spec.orbit/operationId/images`; the persisted file is cleaned up after the create. |
| `TestHandleExecuteImmediateManifestInvalidCreatesNoCR` | An invalid manifest halts the handler (returns nil, no retry) and creates no CR / persists nothing. |
| `TestHandleExecuteImmediateIsIdempotent` | Re-dispatching the same marker creates exactly one CR (dedup query + cache, backstopped by the K8s `AlreadyExists` check). |
| `TestHandleExecuteImmediateLeavesPersistedOnCreateError` | A non-`AlreadyExists` create failure leaves the persisted CR on disk for restart recovery (#998). |

The handler uses a fake dynamic client, so no real cluster is needed. The
`operationId` is minted deterministically from the marker's mtime + package
version, so the tests pin the marker mtime for a stable name.

**Op-id deduplication** (`internal/uc/dedup_test.go`):

```bash
go test ./internal/uc/ -run 'TestDedup|TestHandleExecuteImmediateCache|TestHandleExecuteImmediateConcurrent' -v
```

| Test | Asserts |
|------|---------|
| `TestHandleExecuteImmediateCacheAvoidsAPIOnSteadyState` | First tick issues one List (dedup query) + one Create; every later tick is served from the cache with **zero** API calls. |
| `TestDedupSeededCacheSkipsCreate` | A CR already in the cluster, seeded into the cache by `RecoverOnStartup`, makes the next tick create nothing (restart safety). |
| `TestDedupAPIErrorInvalidatesCache` | A failed dedup query drops the cached entry and propagates the error; once the API recovers, the next tick re-queries and proceeds. |
| `TestHandleExecuteImmediateConcurrentNeverDuplicates` | Eight concurrent ticks for the same op-id converge on exactly one CR (cache races are benign; `AlreadyExists` is authoritative). |

**Freeze handler** (`internal/uc/freeze_test.go`):

```bash
go test ./internal/uc/ -run 'TestHandleFreezeScheduled' -v
```

| Test | Asserts |
|------|---------|
| `TestHandleFreezeScheduledCreatesFreezeCR` | Builds + creates a `NetworkUpgradeFreeze` named `<operationId>-freeze` with op-id + `step: freeze` labels, `created-by` annotation, `spec.orbit/operationId`; persisted file cleaned up. |
| `TestHandleFreezeScheduledSetsScheduledTimeFromContent` | `spec.scheduledTime` in the CR matches the epoch seconds written into `freeze_scheduled.mf` file content, not the marker mtime. |
| `TestHandleFreezeScheduledCopiesImagesFromPrepare` | `spec.images.consensusNode.version` is copied from the resolved Prepare CR. |
| `TestHandleFreezeScheduledCopiesSelectedSourcesWhenPresent` | `spec.selectedSources` is carried forward from `status.selectedSources` of the Prepare CR when populated. |
| `TestHandleFreezeScheduledNoPrepareIsNoOp` | No Prepare CR found for the derived `operationId` → WARN log, no Freeze CR created, returns nil (retry). |
| `TestHandleFreezeScheduledPrepareFailed` | A Failed Prepare → ERROR log, no Freeze CR created, returns nil (halt). |
| `TestHandleFreezeScheduledPrepareInProgress` | Prepare CR exists but phase is empty (in-progress) → DEBUG log, no Freeze CR, returns nil (retry). |
| `TestHandleFreezeScheduledStaleMarkerIsSkipped` | Freeze marker mtime predates `execute_immediate.mf` mtime → WARN `StaleMarker` log, no Freeze CR created (staleness guard 1). |
| `TestHandleFreezeScheduledNoExecuteMarkerIsNoOp` | `execute_immediate.mf` absent AND manifest absent → `OperationIDExhausted` ERROR log, no Freeze CR. |
| `TestHandleFreezeScheduledFallbackWhenNoExecuteMarker` | `execute_immediate.mf` absent but manifest + Succeeded Prepare present → resolves via version prefix, Freeze CR created (fallback path). |
| `TestHandleFreezeScheduledIsIdempotent` | Re-dispatching creates exactly one Freeze CR (dedup). |

**FreezeAbort handler & op-id resolver** (`internal/uc/freeze_abort_test.go`):

```bash
go test ./internal/uc/ -run 'TestResolveOperationID|TestHandleFreezeAborted' -v
```

| Test | Asserts |
|------|---------|
| `TestResolveOperationIDFromCluster` | Resolves the in-flight `operationId` from a non-terminal Prepare CR (cluster = source of truth). |
| `TestResolveOperationIDSkipsTerminalPrepare` | A `Succeeded`/`Failed` Prepare is ignored by `resolveOperationID` (the skip-terminal variant). |
| `TestResolveOperationIDDiskFallback` | With no CR in-cluster, resolves from a Prepare persisted on disk (crash window). |
| `TestResolveOperationIDDiskFallbackPicksMostRecentByTimestamp` | When two persisted Prepares disagree between lexical and chronological order, the chronologically newer one is picked. |
| `TestResolveOperationIDNoneReturnsEmpty` | No Prepare anywhere → empty id (caller no-ops and retries later), not an error. |
| `TestHandleFreezeAbortedCreatesCR` | Re-derives `operationId` from `execute_immediate.mf` + manifest; builds + creates a `NetworkUpgradeFreezeAbort` with op-id + `step: abort` labels, `created-by` annotation, `spec.orbit/operationId` (no images); persisted file cleaned up. |
| `TestHandleFreezeAbortedStaleMarkerIsSkipped` | Abort marker mtime predates `execute_immediate.mf` mtime → WARN `StaleMarker` log, no Abort CR created (staleness guard). |
| `TestHandleFreezeAbortedNoExecuteMarkerIsNoOp` | `execute_immediate.mf` absent AND manifest absent → `OperationIDExhausted` ERROR log, no Abort CR. |
| `TestHandleFreezeAbortedIsIdempotent` | Re-dispatching the marker creates exactly one Abort CR (dedup). |
| `TestHandleFreezeAbortedDoesNotDeleteSiblings` | A co-existing Prepare CR survives the abort (audit trail; the UC never deletes sibling CRs). |

**Restart recovery** (`internal/uc/recovery_test.go`, story 8.8, #998):

```bash
go test ./internal/uc/ -run 'TestRecover|TestCleanup' -v
```

| Test | Asserts |
|------|---------|
| `TestRecoverPersistedCRs_DiskOnlyResent` | A CR persisted to disk but absent from the cluster is re-sent to the K8s API on startup; the disk copy is deleted and the dedup cache is updated. |
| `TestRecoverPersistedCRs_AlreadyInCluster` | A disk CR that was already delivered before the crash has its stale disk copy removed without creating a duplicate CR. |
| `TestRecoverPersistedCRs_NoPersisted` | No persisted files → no-op, no error. |
| `TestCleanupCompletedArtifacts_SucceededRemovesAll` | When an Execute CR has reached `Succeeded`, startup cleanup removes persisted CRs, marker files, and the entire upgrade artifacts directory (marker-path). |
| `TestCleanupCompletedArtifacts_FailedRemovesMarkersNotDir` | When an Execute CR has `Failed`, marker files and persisted CRs are removed, but the marker-path directory is preserved for operator diagnostics. |
| `TestCleanupCompletedArtifacts_InFlightNotCleaned` | An Execute CR in a non-terminal phase (e.g. `PendingNodeUpgrade`) is not touched — marker files and persisted CRs are preserved. |
| `TestCleanupCompletedArtifacts_NoExecuteCRs` | No Execute CRs in the cluster → no-op, no error. |
| `TestRecoverOnStartup_FullFlow` | End-to-end: a crash-window disk CR is re-sent AND a completed-operation's marker files are cleaned, both in one `RecoverOnStartup` call. |

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

**Integration tests — real API server** (`test/integration/uc/`):

The suites above use the **fake** dynamic client, which **ignores field
selectors**. The UC dedup query filters server-side on the `spec.operationId`
`selectableField`, so that mechanism can only be validated against a real API
server. The envtest suite (run on Kubernetes 1.32) closes that gap — it
codifies the manual steps in §4a/§4b into automated checks:

```bash
task test:integration            # envtest 1.32; runs ./test/integration/...
```

| Test | Asserts |
|------|---------|
| `TestSelectableFieldFiltersServerSide` | A `dynamic` List with `FieldSelector: spec.operationId=…` narrows to exactly the matching CR server-side; a selector on the non-selectable `spec.orbit` is **rejected** (proving enforcement is real). **Fails if `selectableFields` is removed from the CRDs** — guards the dedup mechanism against regression. |
| `TestDedupIdempotentAndRecovery` | Repeated ticks create exactly one CR against a real server; a fresh process reseeded via `RecoverOnStartup` does not re-create it. |
| `TestHandleFreezeAbortedResolvesOperationID` | `HandleFreezeAborted` resolves the operationId from the existing Prepare's `spec.operationId` (read back from the apiserver) and creates a matching `NetworkUpgradeFreezeAbort`. |
| `TestCRStoreListSkipsCorruptFile` | A single corrupt persisted CR is skipped (warned), not fatal — the good CR still recovers. |

Each test runs in its own namespace for isolation, and the suite does **not**
start the controllers (the UC client behaviour is exercised on its own). Real
CRD validation also covers what the fake client cannot — e.g. the
`Sha256Digest` pattern on `layerHashes`.

Build-time version metadata is now provided by the shared, dependency-free
[`github.com/automa-saga/version`](https://github.com/automa-saga/version)
package (stamped via `-ldflags -X`); it carries its own tests, so there is no
in-repo version suite. `uc version` renders `text` (default) or `json`.
