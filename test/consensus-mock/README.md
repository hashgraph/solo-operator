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
manifest itself — see "Local kind harness" below.

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
task -d test/consensus-mock build   # consensus-mock:0.1.0 + :0.2.0 (ghcr.io/hashgraph/solo-operator/consensus-mock), cross-compiled linux/amd64+arm64
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

The operator's `NetworkUpgradePrepareReconciler` only marks a Prepare CR
`Succeeded` once it verifies the upgrade target's `layerHashes` against the
**real, network-reachable** image (`internal/registry/checker.go` pulls the
image config and compares RootFS `diff_ids` — there is no opt-out: a
component that can't be verified is never deployed). `kind/registry.yaml`
runs a small `registry:2` Deployment inside the cluster for exactly this —
`0.2.0` (the upgrade target) is pushed there and verified for real; `0.1.0`
(the initial deploy) is only ever `kind load`ed and never goes through
verification, since that only applies to `NetworkUpgrade*` targets, not a
capsule's first deploy.

**Prerequisites**: `crane` (`go install
github.com/google/go-containerregistry/cmd/crane@v0.22.0`) and `jq`, used by
`build:manifest` to compute real deterministic layer hashes. An operator must
also be reconciling the cluster — either run it in its own terminal the same
way `docs/beacon/Taskfile.yml`'s `operator` task does:
```bash
task -d docs/beacon build && go run ./cmd/main.go --log-level=debug
```
or deploy it into the kind cluster via this harness (`task e2e` does it for you) — note that mode allow-lists
the in-cluster registry's ClusterIP via `--allowed-registry-hosts` (the
operator's registry client denies private-IP hosts by default as an SSRF
guard); running the operator on the host instead needs the same flag set
manually, pointed at the registry's ClusterIP (`REGISTRY_CLUSTER_IP` in the
Taskfile, currently `10.96.100.100`).

```bash
task -d test/consensus-mock e2e       # build images, kind reset + CRDs + in-cluster registry (destroys any existing "solo-operator" kind cluster),
                                      # load images, deploy operator + node0 (mock:0.1.0) + genesis, then run `upgrade`
task -d test/consensus-mock upgrade   # beacon prepare (real layerHashes manifest vs. the in-cluster registry) -> freeze (FREEZE_DELAY_SECS=30 to override the 10s default)
                                      # -> wait for NetworkUpgradeExecute and print the timing report
```

`upgrade` can be re-run against an already deployed node0. Every other step is an internal task that `e2e`/`upgrade` call.

The manifest `beacon:prepare` generates (`manifests/consensus-node-components.yaml`,
via `build:manifest`) is also a useful verification fixture on its own — e.g.
hand-edit a copy with a wrong `layerHashes` entry, re-stage it with
`kubectl cp`, and confirm the Prepare CR correctly fails verification instead
of silently passing.

## Timing the full flow

Once `beacon:prepare`'s Prepare CR reaches `Succeeded`, `beacon:freeze`'s `NetworkUpgradeFreeze` CR should
follow, and eventually `NetworkUpgradeExecute` reaches
`ReadyForProvisionerDaemon`, the operator swaps the StatefulSet pod to
`:0.2.0`, and the new pod reaches `Active` again with the Execute CR
`Succeeded`. Time each `beacon` submit (the Job's log line) against its
corresponding UC `CRDelivered`/`PhaseChanged` log line
(`docs/uc/testing-guide.md` §6, or `kubectl -n solo-orbit logs -f node0-0 -c uc`) —
this per-phase latency is what consensus-mock exists to make cheap to
iterate on.
