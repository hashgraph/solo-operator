# consensus-mock

A fast, fake stand-in for the `consensus-node` container, used to time how
long the operator + `uc` sidecar take to carry a real upgrade end to end
(`NetworkUpgradePrepare` → `NetworkUpgradeFreeze` → `NetworkUpgradeExecute`)
without paying the cost of a real JVM consensus node.

It is a real drop-in replacement for the `consensusNode` container image in a
`ConsensusCapsule` — no operator or CRD changes are required to use it.

## What it does

- Logs a timestamp as the very first and very last thing it does, so pod
  start/stop latency can be measured precisely.
- Speaks just enough HAPI gRPC (`FreezeService`, plus the `NetworkService`/
  `CryptoService` RPCs `beacon` needs for its reachability preflight and
  receipt polling) to accept a **real** `beacon freeze` submission — not a
  simplified out-of-band trigger.
- Writes the same marker files a real consensus node writes into the shared
  `upgrade` volume when it receives a freeze command, so the `uc` sidecar
  reacts exactly as it would against a real node:
  - `PREPARE_UPGRADE` → `execute_immediate.mf`
  - `FREEZE_ONLY` / `FREEZE_UPGRADE` / `TELEMETRY_UPGRADE` → `freeze_scheduled.mf`
    (content = scheduled epoch seconds), then `now_frozen.mf` once that time
    arrives
  - `FREEZE_ABORT` → `freeze_aborted.mf`
- Emulates `PLATFORM_STATUS` transitions in the consensus log
  (`STARTING_UP` → `ACTIVE` on boot; `ACTIVE` → `FREEZING` →
  `FREEZE_COMPLETE` once frozen), so `ConsensusCapsule.status.phase` advances
  for real and `NetworkUpgradeExecute`'s phase gate isn't stuck.

It does **not** implement `FileService` or generate/copy any upgrade
manifest itself — see "Testing `PREPARE_UPGRADE`" below.

The image also ships trivial stubs for `/package/admin/s6/command/{s6-svc,s6-svstat}`
— `internal/resources/consensus/exec.go`'s `ActuationSubReconciler` execs
these on **every** `ConsensusCapsule` reconcile (Auto or Manual start policy)
to query/actuate the real image's s6-supervised process; without them every
reconcile errors with `ExecTransportFailed` and the capsule never leaves
`Pending`. consensus-mock has no supervision tree — its one process is up for
as long as the container runs — so `s6-svstat -o up` always reports `true`
and `s6-svc` is a no-op.

## Configuration

Flags (each with a `MOCK_`-prefixed env var default):

| Flag | Env var | Default | Purpose |
|---|---|---|---|
| `--grpc-addr` | `MOCK_GRPC_ADDR` | `:50211` | HAPI gRPC listen address |
| `--upgrade-path` | `MOCK_UPGRADE_PATH` | `uc.DefaultUpgradePath` | Where marker files are written |
| `--consensus-log-file` | `MOCK_CONSENSUS_LOG_FILE` | `uc.DefaultConsensusLogFile` | Where `PLATFORM_STATUS` lines are appended |
| `--startup-active-delay` | — | `0s` | `STARTING_UP` → `ACTIVE` delay |
| `--freeze-complete-delay` | — | `1s` | `FREEZING` → `FREEZE_COMPLETE` delay |
| `--node-id` | — | `0` | Cosmetic, included in log lines |

## Building

```bash
task -d test/consensus-mock build          # local binary -> bin/consensus-mock
task -d test/consensus-mock build:all      # cross-compile linux/amd64+arm64
task -d test/consensus-mock docker:build   # -> MOCK_IMG (default ghcr.io/hashgraph/solo-operator/consensus-mock:dev)
task -d test/consensus-mock docker:push
```

## Wiring into a `ConsensusCapsule`

Swap the image behind the existing `softwareVersion` field:

```yaml
spec:
  podProperties:
    containers:
      consensusNode:
        softwareVersion:
          repository: ghcr.io/hashgraph/solo-operator
          imageName: consensus-mock
          imageTag: dev
      uc:
        enabled: true
```

## Local kind harness: 0.1.0 → 0.2.0 upgrade flow via beacon Jobs

`test/consensus-mock/kind/` + the Taskfile targets below build two identical
consensus-mock images (`0.1.0`/`0.2.0` — same code, different tags, to
exercise the versioned-upgrade machinery), stand up a trimmed single-node
`node0` network running `0.1.0`, and drive each step of the upgrade flow by
running the real `beacon` CLI **as a Kubernetes Job** (not from the host)
against it. It reuses `docs/example/Taskfile.yml`'s `deploy:orbit`/
`deploy:common` (Orbit, secrets, config CRs — image-agnostic) and
`docs/beacon/examples/job-freeze.yaml`'s Job pattern.

**Prerequisite**: an operator must be reconciling the cluster. Either run it in
its own terminal the same way `docs/beacon/Taskfile.yml`'s `operator` task does:
```bash
task -d docs/beacon build && go run ./cmd/main.go --log-level=debug
```
or deploy it into the kind cluster via this harness (`task operator:deploy`,
or the full `task e2e`, which calls it for you).

```bash
task -d test/consensus-mock cluster                 # kind reset + CRDs (destroys any existing "solo-operator" kind cluster)
task -d test/consensus-mock docker:build:versions   # consensus-mock:0.1.0 + :0.2.0 (ghcr.io/hashgraph/solo-operator/consensus-mock)
task -d test/consensus-mock kind:load:versions
task -d test/consensus-mock kind:load:beacon
# also needed once per cluster: task docker:build:uc && task kind:load:uc  (root Taskfile)
task -d test/consensus-mock deploy                  # orbit+common+node0(mock:0.1.0)+genesis, ready in seconds
task -d test/consensus-mock beacon:config
task -d test/consensus-mock beacon:prepare          # PREPARE_UPGRADE Job; execute_immediate.mf appears
task -d test/consensus-mock beacon:freeze           # FREEZE_UPGRADE Job (10s out by default; FREEZE_DELAY_SECS=30 to override)
task -d test/consensus-mock beacon:abort            # FREEZE_ABORT Job — alternate path
task -d test/consensus-mock beacon:watch            # poll NetworkUpgrade* CRs (leave running in its own terminal)
task -d test/consensus-mock uc:logs                 # tail the UC sidecar
```

`deploy` accepts `MOCK_TAG=0.2.0` to redeploy node0 straight onto the
"upgraded" tag without going through the Job flow, as a quick sanity check.

**Known limitation**: `beacon:prepare` submits a real `PREPARE_UPGRADE` and
consensus-mock reacts correctly (`execute_immediate.mf` appears, the UC logs
`MarkerDetected`), but the UC then fails with `ManifestReadFailed` — no
`manifests/consensus-node-components.yaml` exists yet. Reaching a
`NetworkUpgradePrepare` CR requires the additional push + `build:manifest` +
`kubectl cp` steps in "Testing `PREPARE_UPGRADE`" above; this harness makes
iterating on that follow-up fast, it doesn't replace it.

## Testing `PREPARE_UPGRADE`

The operator's `NetworkUpgradePrepareReconciler` only marks a Prepare CR
`Succeeded` once it verifies the manifest's `layerHashes` against the
**real, network-reachable** target image (`internal/registry/checker.go`
pulls the image config and compares RootFS `diff_ids` — a placeholder hash
never passes, and `kind load docker-image` alone is not enough since this
check does a real registry pull). So:

1. Push (not just `kind load`) two consensus-mock images to a reachable
   registry, e.g. `ghcr.io/<you>/consensus-mock:v1` and `:v2`.
2. Generate the manifest for the v1→v2 upgrade:
   ```bash
   task -d test/consensus-mock build:manifest IMAGE=ghcr.io/<you>/consensus-mock:v2
   ```
3. Stage the generated file into the running pod's shared `upgrade` volume
   before submitting `PREPARE_UPGRADE` (consensus-mock does not do this
   itself):
   ```bash
   kubectl exec <pod> -c <consensus-node-container> -- mkdir -p <upgrade-path>/manifests
   kubectl cp manifests/consensus-node-components.yaml \
     <namespace>/<pod>:<upgrade-path>/manifests/consensus-node-components.yaml -c <consensus-node-container>
   ```

The same generated file is a useful verification fixture on its own — e.g.
hand-edit a copy with a wrong `layerHashes` entry to confirm the Prepare CR
correctly fails verification.

## Timing the full flow

Once `beacon:prepare`'s Prepare CR reaches `Succeeded` (see "Testing
`PREPARE_UPGRADE`"), `beacon:freeze`'s `NetworkUpgradeFreeze` CR should
follow, and eventually `NetworkUpgradeExecute` reaches
`ReadyForProvisionerDaemon`, the operator swaps the StatefulSet pod to
`:0.2.0`, and the new pod reaches `Active` again with the Execute CR
`Succeeded`. Time each `beacon` submit (the Job's log line) against its
corresponding UC `CRDelivered`/`PhaseChanged` log line
(`docs/uc/testing-guide.md` §6, or `task -d test/consensus-mock uc:logs`) —
this per-phase latency is what consensus-mock exists to make cheap to
iterate on.
