# beacon — usage runbook

`beacon` is the Solo HAPI control-plane CLI: it submits privileged HAPI
transactions (the consensus-node software-upgrade flow) to a running Hedera
network over gRPC, using `hiero-sdk-go`. It is a protocol-layer tool only — it
never touches kubeconfig, Helm, or pods. See [design.md](design.md) for the full
design; this document is the operator runbook.

There are two ways to run it, sharing one binary and one `config.yaml`:

- **[A. Host-direct](#a-host-direct-runbook)** — run `beacon` on your machine
  against a port-forwarded node. Simplest; best for humans and dev loops.
- **[B. In-cluster Job](#b-in-cluster-job)** — run the `beacon` image as a
  Kubernetes `Job` that reaches nodes over Service DNS. Best for CI and
  automation; no port-forward.

Only the `nodes[].host`/`port` values in `config.yaml` differ between them.

For a complete from-scratch walkthrough — bring up a v0.74.0 network on kind and
upgrade it to v0.74.2 with beacon — see
**[End-to-end upgrade test](#end-to-end-upgrade-test-local-kind)**.

---

## Configuration

`beacon` takes a single `-c/--config` pointing at one network's `config.yaml`.
Its **directory is the network's context**: `keys/` and `sysfiles/` resolve
relative to it.

```
solo-local/
├── config.yaml                    # this network (name, shard/realm, payer, nodes)
├── keys/
│   ├── account2.pem               # payer private key (Ed25519 or ECDSA)
│   └── account2.pass              # PEM passphrase (optional; required for non-interactive use)
└── sysfiles/
    └── softwareUpgrade.zip        # the deployment package (host-direct default path)
```

A minimal `config.yaml` (see [examples/configmap-config.yaml](examples/configmap-config.yaml)):

```yaml
name: solo-local
shard: 0
realm: 0
defaultPayer: 2            # privileged payer for special-file upload + freeze (0.0.2 / 0.0.50 / 0.0.58)
defaultNodeAccount: 3
nodes:
  - id: 0
    account: 3
    host: 127.0.0.1        # host-direct: a port-forward; in-cluster: Service DNS
    port: 50211
    tls: false             # local kind: plaintext gRPC, no cert verification
```

Global flags (all bindable via `BEACON_*` env): `-c/--config`, `-p/--payer`,
`-a/--node-account`, `-i/--node-ip`, `-v/--log-level`.

**Passphrase:** interactively, `beacon` prompts (no echo). Non-interactively (CI,
Jobs) there is no TTY, so provide `keys/account<N>.pass` alongside the PEM, or use
an unencrypted PEM.

---

## The canonical upgrade sequence (§7)

The `H` below is the deployment package's SHA-384 — the contract that couples the
whole chain. Upload records it in the `0.0.150` file memo; `file info` reads it
back; `freeze prepare`/`freeze upgrade` must both carry that exact value.

```bash
# 0. build the artifact + compute H, no network (see test/dev/Taskfile.yml)
#    H = the deployment package's SHA-384
H=$(shasum -a 384 softwareUpgrade.zip | awk '{print $1}')

# 1. stage the zip into special file 0.0.150 (verify-before-upload with --expect-hash)
beacon file upload software-zip --expect-hash <H>

# 2. read back the canonical hash the network holds
beacon file info software-zip

# 3. stage the software on every node, pinned to H
beacon freeze prepare  --upgrade-zip-hash <H>

# 4. schedule the freeze that boots the new software (H must equal step 3)
beacon freeze upgrade  --start-time 2026-08-04.14:30:00 --upgrade-zip-hash <H>

# … network freezes at the start time; UC → operator → provisioner execute …

# 5. confirm the new build is live post-restart
beacon network version
```

Abort a scheduled freeze before it fires: `beacon freeze abort`.

Notes:
- **Privileged payer:** special-file upload and all freeze variants require the
  payer to be `0.0.2`, `0.0.50`, or `0.0.58`.
- **Resumable upload:** if an upload is interrupted, re-run with
  `--restart-from-failure` — it matches the on-chain running hash to a chunk
  boundary and re-sends only the missing chunks.
- **Confirmation gate:** the freeze commands prompt on a TTY; pass `--yes` to
  bypass (required for non-interactive/CI use).

---

## A. Host-direct runbook

Prerequisites: a Solo network running on kind, and `keys/` populated.

1. Expose one node's HAPI gRPC to your host (the freeze flow needs just one):

   ```bash
   kubectl -n solo port-forward svc/haproxy-node1-svc 50211:50211
   ```

2. Point `config.yaml` at it (`host: 127.0.0.1`, `port: 50211`, `tls: false`).

3. Run the [sequence above](#the-canonical-upgrade-sequence-7) with
   `beacon -c ./solo-local/config.yaml …`. The upload reads the zip from
   `./solo-local/sysfiles/softwareUpgrade.zip` by default, or pass `--zip <path>`.

---

## B. In-cluster Job

Run the `beacon` image as a `Job`. Small inputs are a ConfigMap + Secret; the
config's `nodes[].host` uses Service DNS (e.g.
`haproxy-node1-svc.solo.svc.cluster.local`).

```bash
kubectl apply -f examples/configmap-config.yaml   # config.yaml
kubectl -n solo create secret generic beacon-keys \
  --from-file=account2.pem=./keys/account2.pem \
  --from-file=account2.pass=./keys/account2.pass  # or edit examples/secret-keys.yaml
```

### Sourcing the deployment zip (tens of MB)

The upgrade zip is far larger than a ConfigMap can hold (1 MiB etcd cap), and
`beacon` is deliberately storage-agnostic — it reads a **local file path** and
knows nothing about S3/HTTP. So the pod supplies the file on a shared volume.
`fsGroup: 65532` on each Job makes the volume and mounted keys readable by the
non-root beacon user (uid 65532); only the *sourcing* differs, never the beacon
container.

The zip is typically built on the host by [`test/dev`](../../test/dev), which
produces two real versions — `staging/build-v<base>.zip` (the deployed version)
and `staging/build-v<upgrade>.zip` (the next version). beacon uploads the
**upgrade** zip. Pick the delivery that matches your cluster:

| Source | How | Portable | Example |
|---|---|---|---|
| **kind hostPath (extraMounts)** | mount `test/dev/staging/` into the node at create time; the Job hostPath sees every rebuild with no copy | ✗ (kind) — **recommended for local/CI** | [kind-config-extramounts.yaml](examples/kind-config-extramounts.yaml) + [job-upload-hostpath.yaml](examples/job-upload-hostpath.yaml) |
| **kind hostPath (docker cp)** | `docker cp staging/build-*.zip <cluster>-control-plane:/var/beacon/…` for a one-off | ✗ (kind) | [job-upload-hostpath.yaml](examples/job-upload-hostpath.yaml) |
| **PVC + loader** | `kubectl cp` the zip into a PVC via a throwaway pod; beacon mounts it read-only | ✅ (real/multi-node) | [pvc-and-loader.yaml](examples/pvc-and-loader.yaml) + [job-upload-pvc.yaml](examples/job-upload-pvc.yaml) |
| **S3 / MinIO / OCI / HTTP** | an `initContainer` fetches into an `emptyDir`; beacon mounts it read-only | ✅ | [job-upload-s3.yaml](examples/job-upload-s3.yaml) |

**Why not a PVC on kind?** kind's default `local-path` provisioner is itself
hostPath-backed, so a PVC only adds an indirection plus a populate step over a
direct node hostPath — with no benefit on a single node. Reach for a PVC on
real/multi-node clusters, where a node hostPath isn't available. (`--restart-from-failure`
needs no local persistence — resume state is on-chain — so it's not a reason to
prefer a PVC either.)

**kind extraMounts loop:**

```bash
# absolute path; see examples/kind-config-extramounts.yaml
kind create cluster --config examples/kind-config-extramounts.yaml
( cd test/dev && task build )                       # writes staging/build-v<base|upgrade>.zip → node /var/beacon
# point job-upload-hostpath.yaml's --zip at /work/build-v<upgrade>.zip (the upgrade version), then:
kubectl apply -f examples/job-upload-hostpath.yaml
```

### Upload, then freeze

```bash
kubectl apply -f examples/job-upload-s3.yaml       # or job-upload-hostpath.yaml
kubectl -n solo logs -f job/beacon-upload-software-zip
# note the "Confirmed SHA-384: <H>" line, then:
#   edit examples/job-freeze.yaml → set --upgrade-zip-hash <H>
kubectl apply -f examples/job-freeze.yaml
```

Freeze Jobs (`examples/job-freeze.yaml`) need no artifact volume — just config +
keys — and must pass `--yes` (no TTY to confirm).

---

## End-to-end upgrade test (local kind)

A full, from-scratch walkthrough: stand up a network at **v0.74.0** (a single
node is enough and is the recommended local path — see step 2), then use `beacon`
to drive a software upgrade to **v0.74.2**. beacon is the HAPI control plane
(upload + freeze); the consensus node, UC sidecar, operator, and provisioner
carry out the actual node upgrade after the freeze fires.

Version map for this test:

| Role | Version | Where it is set |
|------|---------|-----------------|
| Deployed network | `0.74.0` | `docs/example/node{0..3}/consensus-capsule.yaml` (`imageTag`) |
| Upgrade package (beacon uploads this) | `0.74.2` | `test/dev` `HEDERA_UPGRADE_VERSION` → `staging/build-v0.74.2.zip` |

> **Run everything from the worktree that has beacon** — the `feat/beacon-cli-mvp`
> branch. The main checkout on `docs/beacon-design` has neither `internal/beacon`
> nor the aligned example/`test/dev`, so its `task build`/deploy produce the old
> artifacts. Set once:
>
> ```bash
> export ROOT=/path/to/solo-operator/.claude/worktrees/replace-errx-package-bf48cf
> cd "$ROOT"
> ```

### 1. Build both deployment zips + capture the upgrade hash

```bash
cd "$ROOT/test/dev"
task setup      # downloads build-v0.74.0.zip + build-v0.74.2.zip (needs internet)
task build      # -> staging/build-v0.74.0.zip and staging/build-v0.74.2.zip
export ZIP="$ROOT/test/dev/staging/build-v0.74.2.zip"
export H=$(shasum -a 384 "$ZIP" | awk '{print $1}')
echo "H=$H"
```

`H` is the SHA-384 that couples the whole chain (upload memo, `file info`,
`freeze prepare`, `freeze upgrade`). Recompute it whenever the zip is rebuilt.

### 2. Cluster + operator + network at v0.74.0

```bash
cd "$ROOT"
kind delete cluster --name solo || true          # remove any stray cluster
task kind:reset                                   # creates 'solo-operator' + installs CRDs
```

Start the operator in its **own terminal** and leave it running:

```bash
cd "$ROOT" && task run
```

The UC sidecar image the operator injects is tagged with the operator's build
version (`:dev` under `go run`), which is not published — build and load it
locally so the `PullIfNotPresent` sidecar finds it:

```bash
cd "$ROOT"
task docker:build:uc
docker tag ghcr.io/hashgraph/solo-operator-uc:latest ghcr.io/hashgraph/solo-operator-uc:dev
kind load docker-image ghcr.io/hashgraph/solo-operator-uc:dev --name solo-operator
```

Deploy the network into namespace `solo-orbit`. For local beacon testing prefer
the **single-node** path — one consensus node reaches ACTIVE trivially and won't
starve a laptop the way four JVMs do (four nodes commonly return `BUSY` and stall
receipts under CPU pressure). `task deploy:single` deploys orbit + node0 (with
its HAProxy) + genesis; genesis derives the roster from the deployed capsules, so
one node yields a valid 1-node roster at 100% weight:

```bash
cd "$ROOT/docs/example"
task setup
task deploy:single                                # node0 + genesis; waits for Ready + prints the port-forward
```

<details>
<summary>Alternative: full 4-node network</summary>

Only needed to exercise multi-node behaviour. Requires a well-resourced Docker VM
(8+ CPU / 16+ GB); under-provisioned, nodes return `BUSY` and transactions never
get receipts.

```bash
cd "$ROOT/docs/example"
task setup
task deploy                                       # orbit + 4 nodes + genesis
kubectl apply -f node0/haproxycapsule.yaml        # node0 gRPC endpoint for beacon
kubectl -n solo-orbit rollout status deploy/haproxy-node0 --timeout=120s
```

</details>

Checkpoint — node0 pod `3/3 Running` (consensus-node + alloy + uc); the 4-node
path shows all four:

```bash
kubectl -n solo-orbit get pods -l app.kubernetes.io/component=consensus-node
```

### 3. beacon config + payer key (well-known 0.0.2 genesis key)

```bash
cd "$ROOT"
task build:beacon                                 # -> bin/beacon
mkdir -p /tmp/beacon-solo/keys

# 0.0.2 genesis key (public key matches network-genesis bootstrap.genesisPublicKey)
KEY_HEX=302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137
{ echo "-----BEGIN PRIVATE KEY-----"; echo "$KEY_HEX" | xxd -r -p | base64; echo "-----END PRIVATE KEY-----"; } > /tmp/beacon-solo/keys/account2.pem
: > /tmp/beacon-solo/keys/account2.pass           # empty passphrase (unencrypted PEM)

cat > /tmp/beacon-solo/config.yaml <<'EOF'
name: solo-local
shard: 0
realm: 0
defaultPayer: 2
defaultNodeAccount: 3
nodes:
  - id: 0
    account: 3
    host: 127.0.0.1
    port: 50211
    tls: false
EOF
```

### 4. Drive the upgrade with beacon

Port-forward node0's HAProxy in a **separate terminal** (leave running):

```bash
kubectl -n solo-orbit port-forward svc/haproxy-node0 50211:50211
```

Then run the HAPI sequence:

```bash
cd "$ROOT"
BEACON="bin/beacon -c /tmp/beacon-solo/config.yaml"

# a) upload the v0.74.2 package into special file 0.0.150 (verify-before-write)
$BEACON file upload software-zip --zip "$ZIP" --expect-hash "$H"

# b) confirm the network holds exactly that hash
$BEACON file info software-zip                    # SHA-384 must equal H

# c) stage the upgrade on every node, pinned to H
$BEACON freeze prepare --upgrade-zip-hash "$H" --yes

# d) schedule the freeze that boots the new software (~3 min out, UTC)
START=$(date -u -v+3M '+%Y-%m-%d.%H:%M:%S' 2>/dev/null || date -u -d '+3 min' '+%Y-%m-%d.%H:%M:%S')
$BEACON freeze upgrade --start-time "$START" --upgrade-zip-hash "$H" --yes
```

### 5. Observe the automated upgrade (no beacon involvement past the freeze)

At the scheduled time the network freezes and the rest of the pipeline runs on
its own. Watch it unfold:

```bash
# nodes reach FREEZE_COMPLETE, then the consensus node writes upgrade markers
kubectl -n solo-orbit logs -f node0-0 -c consensus-node | grep -iE "freeze|upgrade"
# the UC sidecar detects the marker, mints the operationId from disk, creates CRs
kubectl -n solo-orbit logs -f node0-0 -c uc
# the operator + provisioner advance the NetworkUpgrade* CRs
kubectl -n solo-orbit get networkupgradeprepare,networkupgradefreeze,networkupgradeexecute -w
```

Expected chain: `FREEZE_COMPLETE` → node writes `execute_immediate.mf` → NMT
extracts 0.0.150 into the upgrade dir → UC creates Prepare/Freeze/Execute CRs
(operationId derived from the on-disk package) → operator/provisioner swap the
image and restart the pod on the new software → UC reports `ACTIVE`.

### 6. Verify the new version

```bash
cd "$ROOT"
bin/beacon -c /tmp/beacon-solo/config.yaml network version   # Services version -> 0.74.2
kubectl -n solo-orbit get pods -l app.kubernetes.io/component=consensus-node -o wide
```

### 7. Cleanup

```bash
kind delete cluster --name solo-operator
```

> **What beacon guarantees vs. what the pipeline does.** Steps (a)–(d) are
> beacon's job and are fully verifiable on their own: the upload records `H` in
> the 0.0.150 memo, `file info` reads it back, and the freeze transactions land
> with receipts. Everything in **step 5** is the consensus node + UC + operator +
> provisioner reacting to the freeze — beacon has already exited. Unlike the
> synthetic `docs/example` `upgrade:*` tasks (which hand-apply CRs and manually
> patch `status.phase`, because no real package reaches the node's disk), this
> beacon-driven path delivers the package for real, so the UC advances the CRs on
> its own. If the network does not return on `0.74.2`, inspect the `uc` container
> log and the NetworkUpgrade* CR phases — that is the UC→operator→provisioner
> stage, not beacon.

---

## Troubleshooting

- **`payer account N is not privileged`** — use `-p 2` (or 50/58) for uploads and
  freezes.
- **passphrase error in a Job** — add `account<N>.pass` to the `beacon-keys`
  Secret, or use an unencrypted PEM.
- **`freeze prepare` rejected / hash mismatch** — the `--upgrade-zip-hash` must
  equal what `file info software-zip` reports; re-upload if they differ.
- **TLS / cert errors against local nodes** — keep `tls: false` for plaintext
  `50211`; cert verification only engages for ledger-pinned networks.
