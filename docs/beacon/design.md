<!-- SPDX-License-Identifier: Apache-2.0 -->

# `beacon` — Design Document

| | |
|---|---|
| **Status** | Draft |
| **Component** | `cmd/beacon` (new) in `solo-operator` — may later move to a standalone `solo-beacon` repository |
| **Author(s)** | _TBD_ |
| **Related HIPs** | [hip-1494 — Consensus Node Deployment Package](https://github.com/hiero-ledger/hiero-improvement-proposals/blob/LM-consensus-node-deployment-package/HIP/hip-1494.md) (draft branch), [hip-1496 — Globally Synchronized Network Upgrade Process](https://github.com/hiero-ledger/hiero-improvement-proposals/blob/LM-network-upgrade/HIP/hip-1496.md) (draft branch) |
| **Supersedes** | `yahcli` (Java, `hiero-consensus-node/hedera-node/yahcli`) for Solo use cases |

> **Name.** `beacon` — a guiding signal transmitted to the running network. The metaphor is deliberate: this tool *emits commands/transactions to an already-running network* (a signal/command role), as opposed to building, shipping, or deploying it (the infrastructure role owned by `solo-operator` and `solo-provisioner`). The name is unique, greppable, does not ride on the existing `solo` CLI, and reserves a clean future home: `solo-beacon`.

---

## 1. Summary

`beacon` is a standalone, node-operator-facing command-line tool that submits **HAPI transactions** to a running Hedera consensus network. It is the Go replacement for the Java [`yahcli`](https://github.com/hiero-ledger/hiero-consensus-node/tree/main/hedera-node/yahcli) DevOps CLI, scoped first and foremost to **driving and testing the consensus-node (CN) software-upgrade flow** described in hip-1496, and secondarily to the general operational transactions (accounts, system files, DAB nodes, version) that yahcli provides.

It is a **protocol-layer control plane**: a thin, ergonomic CLI over [`hiero-sdk-go`](https://github.com/hiero-ledger/hiero-sdk-go) that builds, signs, submits, and verifies individual privileged transactions against live nodes over gRPC. It is deliberately *not* a Kubernetes/infrastructure tool and *not* a load generator.

## 2. Motivation

The globally-synchronized network upgrade (hip-1496) is triggered by a specific, ordered sequence of HAPI transactions against the live network — staging the upgrade artifact into a system file, telling the network to validate and prepare it, then scheduling the freeze that boots the new software. Today the only tool that performs this sequence is `yahcli`, a Java program tightly coupled to the `hedera-services` BDD test framework and distributed as a Docker image running a slow-to-start JAR.

For the Solo toolkit we need a control-plane trigger that:

- is a **single static Go binary** (fast start, trivial to run by hand or in CI, no JVM);
- is built on the **same SDK the rest of Solo already uses** (`hiero-sdk-go`, already a direct dependency of `solo-operator`);
- shares the repository, conventions, and release pipeline of the operator that owns the upgrade CRDs and the UC sidecar it complements — while staying loosely coupled enough to graduate into a standalone `solo-beacon` repo later;
- can be extended to the wider yahcli operational surface (accounts, sysfiles, DAB) as needed.

## 3. Goals / Non-Goals

### Goals
- Fully drive and test the CN upgrade flow end-to-end: stage artifact → verify hash → `freeze prepare` → `freeze upgrade` → confirm new version.
- Reproduce yahcli's freeze family under a `freeze` group: `freeze only`, `freeze upgrade`, `freeze prepare`, `freeze abort`, `freeze telemetry`.
- Provide the supporting operations needed around an upgrade: system-file upload/download (incl. special files `0.0.150`/`0.0.159`), special-file hash check, network version query, account balance/info, and DAB `nodes` create/update/delete.
- Keep yahcli's key material on-disk conventions so existing keys transfer as-is (`<network>/keys/account<N>.pem[/.pass]`); the config is reshaped to one file per network for safety (§10), not the shared multi-network `config.yml`.
- Be safe by default for privileged operations: explicit target selection, dry-run where feasible, clear success/failure exit codes.

### Non-Goals
- **Not** a Kubernetes/infra actuator. It never touches kubeconfig, Helm, pods, or host filesystems. That is the job of `solo-operator` (controllers) and `solo-provisioner` (host/daemon execute phase).
- **Not** a load generator. Sustained transaction load under chaos is [`hammer`](https://github.com/hashgraph/solo-chaos)'s job.
- **Not** a long-running service or reconciler. Each invocation performs one bounded operation and exits.
- **No** multi-sig payer support in the MVP (matching yahcli's current limitation).

## 4. Where `beacon` fits in the upgrade choreography

The network upgrade (hip-1496) is a pipeline across **four distinct components at three layers**. `beacon` is the **trigger at the front**; it must not be confused with the infra actuators at the back.

```
   PROTOCOL LAYER                 KUBERNETES CONTROL LAYER              HOST / INFRA LAYER
 ┌─────────────────┐          ┌──────────────┬──────────────────┐   ┌───────────────────────┐
 │     beacon      │          │  UC sidecar  │  solo-operator   │   │  solo-provisioner     │
 │  (this tool)    │          │              │  controllers     │   │  daemon               │
 ├─────────────────┤          ├──────────────┼──────────────────┤   ├───────────────────────┤
 │ node operator   │  HAPI    │ watches node │ reconciles       │   │ watches               │
 │ submits:        │  tx to   │ marker files │ NetworkUpgrade*  │   │ NetworkUpgradeExecute │
 │  • sysfile up   │  LIVE    │ → emits      │ CR phases;       │   │ CR; on phase          │
 │    (0.0.150)    │  network │ NetworkUp-   │ advances to      │   │ ReadyForProvisioner-  │
 │  • prepare-     │ ───────► │ grade* CRs   │ ReadyForProvi-   │──►│ Daemon: swaps binary, │
 │    upgrade      │          │              │ sionerDaemon     │   │ restarts node process │
 │  • freeze-      │          │              │                  │   │                       │
 │    upgrade      │          │              │                  │   │                       │
 └─────────────────┘          └──────────────┴──────────────────┘   └───────────────────────┘
   hiero-sdk-go, gRPC            client-go, CRs                        client-go watch + host ops
   Hedera account keys           in-cluster RBAC                       scoped kubeconfig
```

Reading the pipeline:

1. **`beacon` (this tool) — the trigger.** A node operator (or CI) stages the deployment package (hip-1494) into system file `0.0.150`, then submits `freeze prepare` and `freeze upgrade` HAPI transactions to the **live network**. This is what sets the whole upgrade in motion.
2. **UC sidecar** (`cmd/uc` in this repo) — watches each node's host-mounted marker directory and translates markers into `NetworkUpgrade*` Custom Resources.
3. **`solo-operator` controllers** — reconcile those CRs, advancing the upgrade through its phases until a `NetworkUpgradeExecute` CR reaches phase `ReadyForProvisionerDaemon`.
4. **`solo-provisioner` daemon** — the infra actuator; watches `NetworkUpgradeExecute`, and on that phase performs the host-side execute step (swap binaries, restart the node process).

**Why this places `beacon` here and nowhere else:**

| Dimension | `beacon` (trigger) | `solo-provisioner` (reactor) | `hammer` (load) |
|---|---|---|---|
| Layer | HAPI protocol client | K8s/host infra actuator | HAPI protocol client |
| Direction | **initiates** the flow | **reacts** to CRs (watch→execute) | fires continuous load |
| Execution | one bounded op, verified | daemon, event-driven | massively concurrent |
| Deps | `hiero-sdk-go` | `client-go`, host ops | `hiero-sdk-go` |
| Credentials | privileged **Hedera keys** | scoped **kubeconfig** | one operator key |
| Binding | network-agnostic (any net) | one cluster / one CN | one target net |

`beacon` and `solo-provisioner` are complementary halves of the same upgrade, at opposite ends and opposite directions — not candidates to merge. Folding the initiator into the reactor would invert the choreography and mix two trust models (Hedera signing keys vs. a cluster kubeconfig) in one binary. `beacon` and `hammer` share the SDK but serve opposite purposes (precise, verified single ops vs. aggregate throughput under fault).

`solo-operator` is the correct starting home because it is the one repository that already (a) depends on `hiero-sdk-go`, (b) owns the `NetworkUpgrade*` CRDs, and (c) ships the UC sidecar — so a node-operator-facing HAPI CLI is the natural manual complement to that automated machinery, sharing its SDK plumbing and staying at the same protocol layer. Should it later warrant its own release cadence and identity, the substrate (§5.1) is structured to lift out cleanly into a standalone `solo-beacon` repository.

## 5. Architecture

`beacon` mirrors yahcli's proven structure — a thin command layer over a transaction-building library — but swaps the substrate from the Java BDD `HapiSpec` framework to `hiero-sdk-go`.

```
cobra Command  ──►  operation (internal/beacon/ops)  ──►  hiero-sdk-go  ──►  gRPC to node
 (parse flags)       (build → sign → submit → verify)      (proto + signing)
```

Each command:
1. loads config + resolves the target network/node and payer;
2. loads the payer (and any other required) key from the keystore;
3. constructs a `hiero.Client` pointed at the target node(s);
4. builds exactly one operation, submits it, awaits the receipt, and **verifies the outcome** (status code, and for stateful ops a read-back);
5. prints a clear result and returns `0` on success / non-zero on failure.

Unlike yahcli, `beacon` cares about each transaction's individual outcome — success means "the receipt status was `SUCCESS` **and** the expected on-chain state is observable," not merely "a transaction was submitted."

### 5.1 Proposed package layout

```
solo-operator/                       # (or, later, a standalone solo-beacon repo)
├── cmd/
│   └── beacon/
│       └── main.go                 # cobra root, global flags, envPrefix "BEACON_"
├── internal/
│   └── beacon/
│       ├── config/                 # config.yaml loader (one network/file), node/payer resolution
│       ├── keystore/               # account<N>.pem/.pass loader (Ed25519 + ECDSA)
│       ├── client/                 # network map → hiero.Client, submit+await+verify helper
│       ├── ops/
│       │   ├── freeze.go           # all 5 FreezeType variants
│       │   ├── files.go            # file upload/download/info (FileUpdate/Append, FileContentsQuery, FileInfoQuery)
│       │   ├── pkg.go              # local software-zip packaging + SHA-384 (no network)
│       │   ├── version.go          # NetworkVersionInfoQuery
│       │   ├── accounts.go         # balance / info / send / rekey (later)
│       │   └── nodes.go            # DAB node create/update/delete (later)
│       └── commands/               # cobra subcommands wiring flags → ops
└── docs/beacon/
    └── design.md                   # this document
```

The `internal/beacon/{config,keystore,client}` packages form a small, self-contained substrate with no dependency on operator-internal packages. This is intentional: it keeps `beacon` a clean HAPI-only tool today, and lets the whole `cmd/beacon` + `internal/beacon` tree be promoted into a standalone `solo-beacon` module later with minimal churn (the `internal/` packages simply become that repo's packages).

### 5.2 Conventions (inherited from `solo-operator`)

- **cobra** for commands; **viper**-style env binding under prefix `BEACON_` (e.g. `--payer` ↔ `BEACON_PAYER`), matching the `uc` sidecar's `UC_` pattern.
- Reuse the same shared libraries `solo-operator` already depends on (no new equivalents): **logging** via `github.com/automa-saga/logx`, **errors** via `github.com/automa-saga/errx`, and **build/version stamping** via `github.com/automa-saga/version` (as in `cmd/uc`). Do not introduce `joomcode/errorx` or other logging/error libraries in `beacon` code.
- SPDX header on every Go file; Conventional Commits; commits signed `-s` (DCO) and `-S` (GPG).
- Built with Task; a `task build:beacon` target produces `bin/beacon`.

## 6. Command surface

### 6.1 Global options (root)

| Flag | Env | Meaning |
|---|---|---|
| `-c, --config` | `BEACON_CONFIG` | path to the network's `config.yaml` (default `./config.yaml`) |
| `-p, --payer` | `BEACON_PAYER` | payer account num (e.g. `2`, `50`, `58`) |
| `-a, --node-account` | `BEACON_NODE_ACCOUNT` | override target node's fee-collection account |
| `-i, --node-ip` | `BEACON_NODE_IP` | target a node by IP |
| `-v, --log-level` | `BEACON_LOG_LEVEL` | `warn` \| `info` \| `debug` |

A `config.yaml` describes exactly **one** network (§10), and its directory is that
network's self-contained context: `keys/` and `sysfiles/` resolve relative to it.
Network, keys, and artifacts are therefore all selected by the single `-c` flag —
there is no separate `--network` or `--working-dir`, and no `defaultNetwork` to get
wrong. The typical invocation needs no context flag at all:
`cd solo-local && beacon file download 150`.

### 6.2 Upgrade-critical commands (MVP — Phase 1)

The five freeze variants are grouped under a **`freeze`** command (the HAPI `FreezeService` noun), mirroring beacon's `file`/`network` groups and yahcli's freeze family. Each is a single `FreezeService.freeze` call; only the fields below are ever set on the `FreezeTransactionBody` — the deprecated `start_hour/min`, `end_hour/min` fields are never used.

| Command | `freeze_type` | `start_time` | `update_file` | `file_hash` | HAPI |
|---|---|---|---|---|---|
| `freeze prepare --upgrade-zip-hash <hex> [--upgrade-file-num 150]` | `PREPARE_UPGRADE` (2) | — | `0.0.150` | ✅ | `Freeze` |
| `freeze upgrade --start-time <utc> --upgrade-zip-hash <hex>` | `FREEZE_UPGRADE` (3) | ✅ | `0.0.150` | ✅ | `Freeze` |
| `freeze telemetry --start-time <utc> --upgrade-zip-hash <hex>` | `TELEMETRY_UPGRADE` (5) | ✅ | `0.0.159` | ✅ | `Freeze` |
| `freeze only --start-time <utc>` | `FREEZE_ONLY` (1) | ✅ | — | — | `Freeze` |
| `freeze abort` | `FREEZE_ABORT` (4) | — | — | — | `Freeze` |
| `file upload software-zip [--restart-from-failure]` | — | — | — | — | `FileUpdate` + chunked `FileAppend` → `0.0.150` |
| `file info software-zip` (alias `file info 150`) | — | — | — | — | `FileInfoQuery` (reads SHA-384 from file memo) |
| `network version` | — | — | — | — | `NetworkVersionInfoQuery` (HAPI `NetworkGetVersionInfo`) |
| `version` | — | — | — | — | none (local) — beacon's own build metadata (`-o text\|json`) |
| `package software-zip --from <dir> --out <zip>` | — | — | — | — | none (local) — deterministic zip + SHA-384 |

- `--start-time` is UTC in yahcli's format `yyyy-MM-dd.HH:mm:ss`, parsed to a `time.Time` (which is what the `hiero-sdk-go` freeze API's `SetStartTime` takes; see §8.3).
- `--upgrade-zip-hash` is a **hex** SHA-384; it is decoded to raw bytes for `file_hash`. The **same** hash must be passed to `freeze prepare` and `freeze upgrade`.
- `network version` queries the connected network's active build (services + HAPI version); the plain `version` command, by contrast, prints **beacon's own** build metadata and needs no network. (`beacon --version` is the equivalent root flag.)
- Fee: match yahcli's simple approach — set a generous `maxTransactionFee` rather than computing an exact fee.
- `package software-zip` is a **local, no-network** convenience: it deterministically zips an already-assembled directory into the raw special-file artifact for `0.0.150` and prints its SHA-384 (the `H` that feeds `freeze prepare`/`freeze upgrade`, §7). beacon owns **only** this final zip+hash step; sourcing the inputs (downloading the consensus-node build, overlaying node config, embedding the UC manifest, bumping `VERSION`) stays in `test/dev/Taskfile.yml`, which invokes this command for the last step. This is the one deliberate, narrow exception to the pure-HAPI boundary (§3/§4) — it touches only local files, never the network or Kubernetes.

### 6.3 Supporting commands (Phase 2+)

- The `file` command group operates on the Hedera File Service by **file id** (a bare number or `S.R.N` literal, composed with the network's shard/realm — e.g. `121`, `122`, `159`, or `0.0.150`). It is named `file` (not the yahcli-era `sysfiles`) because it is generic over *any* file, not just the privileged system files.
- `file download <file-id> -o <path>` is **fully generic today**: `FileContentsQuery` returns the raw bytes of *any* file, written verbatim — no serdes needed.
- `file info <file-id>` is generic today: `FileInfoQuery` returns size, expiry, keys, and the memo (for special files, the running SHA-384) — it subsumes the old `hash-check`.
- `file upload <file-id> <path>` is generic **only for raw/special files** (`150`/`159`, uploaded verbatim). Structured system files (app properties `0.0.121`, api permissions `0.0.122`, fee schedules, throttles) additionally require the JSON⇄protobuf serdes that lives in the Java services code (§9 gap 4) — that remains Phase 2. `software-zip` is a convenience alias for `file upload 150` (special-file path). _(Note: the legacy address-book files `0.0.101/102` are largely superseded by the roster; they are only examples of "any file id", not promised named commands.)_
- `accounts balance|info|send|rekey` — pre/post-upgrade sanity probes and key management.
- `nodes create|update|delete` (DAB) → `NodeCreate/NodeUpdate/NodeDelete` — address-book membership changes that may accompany an upgrade.
- `keys gen-new|print-public` — offline key utilities.

## 7. The CN upgrade flow, end to end

The canonical sequence a node operator (or CI job) runs against a live network, and the state coupling that ties the steps together:

```
0. beacon package software-zip --from <dir> --out <zip>  # (optional) build the artifact + print SHA-384 H, no network
1. beacon file upload software-zip                # FileUpdate + N×FileAppend → 0.0.150
      └─ network records running SHA-384 in the file memo
2. beacon file info software-zip                  # read memo → the canonical hex hash H
3. beacon freeze prepare   --upgrade-zip-hash H    # nodes validate 0.0.150 against H, unzip & stage
4. beacon freeze upgrade   --start-time T \         # schedule the freeze that boots the new software
                           --upgrade-zip-hash H      #   (H must equal step 3)
   … network freezes at T; UC sidecar → operator controllers → provisioner daemon execute …
5. beacon network version                           # confirm the new build is live post-restart
   (abort path: beacon freeze abort)
```

The **SHA-384 `H` is the contract that couples the whole chain**: the network stores it in the `0.0.150` file memo after upload, `file info` reads it back, and `freeze prepare`/`freeze upgrade` must both carry that exact value or the nodes reject the upgrade. This is why upload, info, prepare, and freeze must be designed and tested as one chain, not as isolated commands.

## 8. Detailed design of the non-trivial parts

Most commands are a near-mechanical mapping to `hiero-sdk-go` (see §9). Two areas need real design.

### 8.1 Special-file upload to `0.0.150`

This is the only genuinely involved piece. Special files (`0.0.150`, `0.0.159`) have no 1 MB size cap and the network tracks their running SHA-384 in the file memo. The upload:

1. Read the raw deployment-package zip (special files are uploaded verbatim; other sysfiles first go through JSON→proto serdes).
2. **Initial op:** `FileUpdateTransaction(0.0.150)` with `contents = firstChunk` (first `bytesPerOp` bytes). yahcli defaults: `bytesPerOp = 4096`.
3. **Then N × `FileAppendTransaction(0.0.150)`**, each up to `bytesPerOp` bytes, until the file is fully written. (`hiero-sdk-go`'s `FileAppendTransaction` can auto-chunk; we must raise its defaults — see §9 gaps.)
4. Each op signed by the payer (privileged `0.0.2` / `0.0.50` / `0.0.58`).

yahcli batches appends into concurrent "bursts" (default 256/burst) purely for throughput; for the MVP, **sequential appends are acceptable** and simpler. Burst concurrency can be a later optimization.

### 8.2 `--restart-from-failure` (resumable upload)

A multi-thousand-append upload must be resumable. The design ports yahcli's hash-prefix logic:

1. `FileInfoQuery(0.0.150)` → read the file **memo** = the running SHA-384 the network has committed so far.
2. Walk **only the append/chunk boundaries** — i.e. the exact positions the upload commits at (`min(bytesPerOp, len)`, then `2·bytesPerOp`, `3·bytesPerOp`, …), computing `sha384(zip[0:boundary])` at each until one matches the on-chain memo. Constraining the search to these boundaries makes it **O(#chunks)**, not O(file size); hashing arbitrary prefix lengths would be needlessly expensive on a multi-MB zip.
3. Skip exactly the appends up to the matched boundary and resume from there.

This makes an interrupted upload cheap to continue and is the trickiest logic to get byte-exact; it warrants dedicated unit tests with a known-vector zip.

### 8.3 Time & hash parsing

- UTC `yyyy-MM-dd.HH:mm:ss` → `time.Time` (a small parser; Go's reference layout is `2006-01-02.15:04:05`).
- `--upgrade-zip-hash` hex → `[]byte` via `hex.DecodeString`.

## 9. Mapping to `hiero-sdk-go`

All required types already exist in `hiero-sdk-go/v2` (a direct `solo-operator` dependency). The `FreezeTransaction` maps 1:1 to the fields in §6.2.

| Operation | `hiero-sdk-go` type | Key API |
|---|---|---|
| Freeze family | `NewFreezeTransaction()` | `SetFreezeType`, `SetStartTime(time.Time)`, `SetFileID(FileID)`, `SetFileHash([]byte)` |
| `FreezeType` enum | `hiero.FreezeType` | `FreezeTypeFreezeOnly`(1), `…PrepareUpgrade`(2), `…FreezeUpgrade`(3), `…FreezeAbort`(4), `…TelemetryUpgrade`(5) — values match HAPI exactly |
| Initial file write | `FileUpdateTransaction` | `SetFileID`, `SetContents([]byte)`, `SetFileMemo` |
| Append chunk | `FileAppendTransaction` | `SetFileID`, `SetContents`, `SetChunkSize`, `SetMaxChunks` |
| Hash check | `FileInfoQuery` | → `FileInfo.FileMemo` (SHA-384) |
| Version | `NetworkVersionInfoQuery` | services + HAPI version |
| DAB nodes | `NodeCreate/Update/DeleteTransaction` | address-book membership |
| Keys | SDK PEM loaders | Ed25519 + ECDSA `PrivateKey` |

### Gaps requiring application logic (no SDK equivalent)

1. **Append defaults differ.** `FileAppendTransaction` defaults to `chunkSize: 2048, maxChunks: 20`. A deployment-package zip is tens of MB → thousands of appends. We must `SetChunkSize(4096)` and raise `SetMaxChunks` well beyond 20, or drive the update/append loop manually for full control over progress and resume.
2. **Initial-update-then-append split.** The SDK appends to an existing file; yahcli's pattern is *`FileUpdate`(chunk 0) then append the rest*. We replicate that split explicitly.
3. **Restart-from-failure.** No SDK support; port the memo hash-prefix logic (§8.2).
4. **Sysfile serdes** (JSON⇄proto for `0.0.121/122`, fee schedules, throttles) live in the Java services code, not the SDK. Only needed for Phase 2 non-special sysfiles — **not required for the software-upgrade path**, since `0.0.150`/`0.0.159` are raw bytes.
5. **Config + keystore.** yahcli's `config.yml` + `<network>/keys/account<N>.pem/.pass` layout is bespoke; reimplement it (the SDK provides the key-parsing primitives).

## 10. Configuration & key management

Each network gets its **own self-contained directory**: one `config.yaml`
describing that single network, beside the `keys/` and `sysfiles/` it uses. This
is a deliberate departure from yahcli's single multi-network `config.yml`: keeping
`mainnet` next to `solo-local` in one file invites a stale `--network` (from shell
history) firing a privileged transaction at the wrong environment. One file =
one network removes that class of mistake, and `-c` selects the whole context at
once (§6.1). Key material still uses yahcli's `account<N>.pem[/.pass]` names, so
existing keys migrate as-is.

```
solo-local/                       # the network's directory (dir(config) = its context)
├── config.yaml                   # describes exactly this one network
├── keys/
│   ├── account2.pem
│   ├── account2.pass             # optional; else prompt for passphrase
│   └── account58.pem
└── sysfiles/
    └── softwareUpgrade.zip       # the deployment package (hip-1494) → 0.0.150
```

`config.yaml` — a single flat network object (no `networks:` map, no
`defaultNetwork`), with yahcli-compatible field names plus the additions Solo
needs (`host`/`port`/`tls`):

```yaml
# A single network. Keys resolve to <dir(config)>/keys/account<N>.pem and
# artifacts to <dir(config)>/sysfiles/. For Solo networks this file is generated
# from Orbit.AddressBook (see §10.1); shard/realm come from there.
name: solo-local               # label for logs/output; defaults to the directory name
shard: 0                       # from Orbit.AddressBook.ShardId (default 0)
realm: 0                       # from Orbit.AddressBook.RealmId (default 0)
defaultPayer: 2                # privileged account for upgrades (0.0.2 / 0.0.50 / 0.0.58)
defaultNodeAccount: 3
nodes:
  - { id: 0, account: 3, host: network-node1-svc.solo.svc.cluster.local, port: 50211, tls: false }
  - { id: 1, account: 4, host: network-node2-svc.solo.svc.cluster.local, port: 50211, tls: false }
# allowedReceiverAccountIds: []   # Phase-2 guardrail, used only by `accounts send`
```

A public network (e.g. `mainnet`) is just another such directory with a
hand-written `config.yaml` — network-agnostic, no cluster needed.

Field reference (vs yahcli):

| Field | Status | Notes |
|---|---|---|
| `name` | new | per-file network label (logs/output); defaults to the config directory name |
| `defaultNetwork`, `networks` | **removed** | one network per file — no in-file selector, no cross-environment map |
| `defaultPayer`, `defaultNodeAccount`, `shard`, `realm` | kept, same names | drop-in migration from yahcli; `shard`/`realm` default `0`/`0` and propagate to nodes |
| `nodes[].id`, `.account` | kept | node id + fee-collection account num |
| `nodes[].host` | renamed from `ipv4Addr` (**`ipv4Addr` still accepted as an alias**) | supports IP **or** FQDN (Solo service DNS) |
| `nodes[].port` | new, default `50211` | Solo/Envoy/HAProxy may expose non-default HAPI ports |
| `nodes[].tls` | new, default `false` | TLS HAPI (`:50212`) when the endpoint requires it |
| `allowedReceiverAccountIds` | kept, Phase-2 | only used by `accounts send` |
| _(mirror node)_ | not yet | reserved for a future optional field; no MVP flow needs it |

- IDs are composed with the network's `shard`/`realm` (payer, node accounts, and the upgrade file `shard.realm.150`/`159`) — never hardcoded `0.0`. The CLI accepts **both** a bare number and a full `S.R.N` literal (mirrors yahcli's `normalizePossibleIdLiteral` and the operator's `splitAccountID`).
- Keys: Ed25519 **and** ECDSA(secp256k1), loaded from `account<N>.pem` with an optional sibling `account<N>.pass`; prompt interactively when no `.pass` is present.
- **No multi-sig** payer in the MVP.
- Privileged operations (special-file upload, all freeze variants) require the payer to be one of `0.0.2`, `0.0.50`, `0.0.58`.

### 10.1 Config source & generation

A per-network `config.yaml` is beacon's **only runtime input** — this keeps beacon a clean, portable, HAPI-only tool with no `client-go`/kubeconfig dependency (§4), runnable against any network (previewnet, mainnet, non-Solo).

To avoid a hand-maintained node list drifting from what the operator provisions, each network's file is **generated** for Solo networks rather than authored by hand:

- The cluster is the source of truth: `Orbit.Spec.Consensus.AddressBook` already carries `ShardId`, `RealmId`, `LedgerId`, and the node topology, and the operator materializes a full address-book ConfigMap (node id, account, service host/port).
- An **external** generator (a `task beacon:config` / small helper, **not** part of the `beacon` binary) renders one network's `<network>/config.yaml` from `Orbit.AddressBook` or that ConfigMap. Because generation lives outside beacon, the binary never imports `client-go` and the §4 trust boundary holds.
- Non-Solo networks simply get a hand-written `<network>/config.yaml`, exactly like yahcli (minus the multi-network wrapper).

_Deferred enhancement:_ `beacon --discover-from-network` could bootstrap the node list purely over HAPI via `FileGetContents(0.0.102)` given one reachable seed node — attractive but gated on the on-chain address-book endpoints being externally reachable (in K8s they may be internal), so it is a post-MVP option, not a replacement for the file interface.

## 11. Testing strategy

- **Unit:** freeze-body construction per `FreezeType` (assert exact proto fields); hex/time parsing; the restart-from-failure hash-prefix algorithm against a fixed zip vector; config + keystore loading.
- **Integration:** against a local Solo network (kind) — run the full §7 sequence and assert `network version` reflects the new build; assert `file info` returns the uploaded hash; assert `freeze abort` cancels a scheduled freeze.
- **Golden path in CI:** a job that provisions a small network, uploads a signed test package, and exercises the upgrade trigger end-to-end (dovetails with the operator/provisioner e2e once the CR pipeline is wired).

## 12. Security considerations

- `beacon` handles **privileged Hedera signing keys**. It must never log key material, and passphrase prompts must not echo. This is a distinct trust boundary from the operator's in-cluster RBAC and the provisioner's kubeconfig — a reason to keep it a separate binary with a narrow surface.
- Target selection is explicit: `-c` names exactly one network's config file (one file = one environment, §10), and `--node-*` the node — no implicit "current context." Consider a confirmation prompt or `--yes` gate for the irreversible freeze/upgrade commands when run interactively.
- The hash contract (§7) is itself a safety mechanism: nodes reject a `freeze prepare` whose `file_hash` disagrees with the staged `0.0.150`.

## 13. Phasing

| Phase | Scope |
|---|---|
| **P1 — Upgrade MVP** | config + keystore + client substrate; `freeze` family; special-file upload (sequential) + restart-from-failure; `file info`; `network version`; `package software-zip` (local zip+hash). Enough to fully drive/test hip-1496 upgrades. |
| **P2 — Operational parity** | `accounts balance/info/send/rekey`; DAB `nodes create/update/delete`; broader `file` uploads with JSON⇄proto serdes. |
| **P3 — Ergonomics** | burst/concurrent appends; richer dry-run/preview; shell completion; structured (JSON) output for CI. |
| **P4 — (optional) Extraction** | promote `cmd/beacon` + `internal/beacon` into a standalone `solo-beacon` repository with its own release cadence, if warranted. |

## 14. Alternatives considered

- **Enhance `hammer` (solo-chaos).** Rejected: `hammer` is a load generator whose mental model is `bots × tps × duration` and which deliberately ignores per-transaction outcomes. Control-plane ops need sequential, verified, single transactions with key management — the opposite design. (They can still share a thin SDK substrate if useful.)
- **Enhance `solo-provisioner`.** Rejected: the provisioner is a K8s/host infra actuator that *reacts* to `NetworkUpgradeExecute` CRs via `client-go` and has no `hiero-sdk-go` dependency. `beacon` *initiates* the flow at the HAPI layer with Hedera keys. They are opposite ends of the same pipeline (§4); merging them inverts the choreography and mixes trust models.
- **Start in a brand-new standalone repo immediately.** Deferred, not rejected: `solo-operator` already provides the SDK dependency, the upgrade CRDs, the UC sidecar, and a release pipeline, so starting here keeps the human-facing trigger next to the automation it complements and avoids standing up a repo before the shape is proven. The substrate is intentionally structured (§5.1) so extraction into `solo-beacon` is cheap once that is warranted (§13 P4).

### Naming — why `beacon`

- **Correct metaphor.** A beacon *emits a signal/command to the running network*. That is precisely this tool's role (trigger/command), as opposed to build/ship/deploy names (`ship`, `dock`, `lifter`) that describe the infra layer this tool is explicitly not.
- **No collisions.** Avoids `op` (1Password CLI), `up` (serverless / `compose up`), `hammer` (your own load generator), `envoy`/`lighthouse`/`pulsar`/`beam` (existing infra/consensus projects), and does not ride on or get confused with the existing `solo` CLI.
- **Clean future identity.** Reserves an obvious standalone name, `solo-beacon`, if the tool graduates to its own repo.
- Known adjacency to accept consciously: Ethereum's "beacon chain." Judged acceptable — Hedera ≠ Ethereum and the tool is unambiguously Hedera-specific.

## 15. Decisions & open questions

1. **Deployment-package hashing — resolved.** The node operator supplies the expected SHA-384 (typically shipped with the release). `file upload software-zip` takes `--expect-hash <hex>` **or** `--expect-hash-file <path>` (bare hex or `sha384sum` format); beacon computes `sha384(zip)` locally, **aborts before uploading on mismatch**, and prints the confirmed hash to feed into `freeze prepare`. (`package software-zip` prints the same hash at build time, §6.2.)
2. **Interactive safety gate — resolved (yes).** The irreversible commands (`freeze prepare`, `freeze upgrade`, `freeze only`, `freeze telemetry`, `freeze abort`) prompt for confirmation when attached to a TTY; `--yes` bypasses (and is required for non-interactive/CI use).
3. **Config source of truth — resolved (per-network file + external generator).** One flat `config.yaml` per network (no multi-network map, no `defaultNetwork`) is beacon's only runtime input, selected by the single `-c` flag; its directory holds that network's `keys/` and `sysfiles/` (§10). One file = one environment removes the cross-environment footgun of a shared config. For Solo networks each file is generated from `Orbit.AddressBook` by an external helper so the cluster stays the source of truth (§10.1). HAPI self-discovery (`--discover-from-network`) is a deferred enhancement.
4. **Shard/realm — resolved.** Per-network `shard`/`realm` (default `0`/`0`), sourced from `Orbit.AddressBook.{ShardId,RealmId}` when generated; all IDs beacon builds are composed with them, and the CLI accepts bare numbers or `S.R.N` literals (§10). _Open:_ confirm whether Solo runs any non-zero-shard network today (drives whether 14.9 needs a non-zero-shard E2E case, else Phase-2).
5. **Mirror node / explorer — deferred (optional).** Not needed for the upgrade flow: transaction outcome is confirmed via HAPI `TransactionReceipt`/`TransactionRecord`, and upload integrity via `FileInfoQuery` (network-computed SHA-384 in the file memo). A mirror node / explorer is only useful for after-the-fact or independent transaction detail; if wanted later, add an optional per-network `mirrorNode` (and/or `explorerUrl`) field — the config is reserved for it (§10).

## 16. References

- yahcli (Java, being replaced): `hiero-consensus-node/hedera-node/yahcli` — [README](https://github.com/hiero-ledger/hiero-consensus-node/blob/main/hedera-node/yahcli/README.md)
- `hiero-sdk-go`: <https://github.com/hiero-ledger/hiero-sdk-go>
- UC sidecar: `cmd/uc` and `internal/uc` in this repository
- `solo-provisioner` (solo-weaver): consensus upgrade daemon `internal/daemon/consensus/upgrade_monitor.go`
- `hammer` load generator: <https://github.com/hashgraph/solo-chaos>
- [hip-1494 — Consensus Node Deployment Package](https://github.com/hiero-ledger/hiero-improvement-proposals/blob/LM-consensus-node-deployment-package/HIP/hip-1494.md) (draft branch)
- [hip-1496 — Globally Synchronized Network Upgrade Process](https://github.com/hiero-ledger/hiero-improvement-proposals/blob/LM-network-upgrade/HIP/hip-1496.md) (draft branch)
