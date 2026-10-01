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
beacon file upload --expect-hash <H>

# 2. read back the canonical hash the network holds
beacon file info

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

### Sourcing the deployment zip

The upgrade zip is too large for a ConfigMap (1 MiB etcd cap). The pod supplies
it on a shared volume; `fsGroup: 65532` makes it readable by the non-root beacon
user. The zip is built by [`test/dev`](../../test/dev). Pick the delivery that
matches your cluster:

| Source | How | Example |
|---|---|---|
| **kind hostPath (extraMounts)** — recommended for local/CI | mount `test/dev/staging/` at create time | [kind-config-extramounts.yaml](examples/kind-config-extramounts.yaml) + [job-upload-hostpath.yaml](examples/job-upload-hostpath.yaml) |
| **PVC + loader** — real/multi-node clusters | `kubectl cp` zip into a PVC | [pvc-and-loader.yaml](examples/pvc-and-loader.yaml) + [job-upload-pvc.yaml](examples/job-upload-pvc.yaml) |
| **S3 / MinIO / OCI / HTTP** | initContainer fetches into emptyDir | [job-upload-s3.yaml](examples/job-upload-s3.yaml) |

### Upload, then freeze

```bash
kubectl apply -f examples/job-upload-s3.yaml       # or job-upload-hostpath.yaml
kubectl -n solo logs -f job/beacon-upload
# note the "Confirmed SHA-384: <H>" line, then:
#   edit examples/job-freeze.yaml → set --upgrade-zip-hash <H>
kubectl apply -f examples/job-freeze.yaml
```

Freeze Jobs need no artifact volume — just config + keys — and must pass `--yes`.

---

## End-to-end upgrade test (local kind)

Stand up a **v0.74.0** single-node network on kind, then drive an upgrade to
**v0.74.2** with beacon. beacon handles the HAPI control plane (upload + freeze);
the consensus node, UC sidecar, operator, and provisioner carry out the actual
upgrade after the freeze fires.

### Quick path (recommended)

Every step is wrapped as a task in [`docs/beacon/Taskfile.yml`](Taskfile.yml).
The `operator` and `port-forward` tasks are long-running (each needs its own
terminal); everything else is one-shot.

```
task -d docs/beacon                         # print the ordered runbook

# Terminal 1 — one-shot setup
task -d docs/beacon prepare-cluster         # build zips + kind reset + UC image
task -d docs/beacon deploy                  # warm images + single-node network + genesis
task -d docs/beacon config                  # beacon binary + config + payer key

# Terminal 2 — leave running
task -d docs/beacon operator

# Terminal 3 — leave running
task -d docs/beacon port-forward

# Terminal 1 — drive the upgrade
task -d docs/beacon upgrade                 # upload → info → prepare → freeze
task -d docs/beacon watch                   # observe NetworkUpgrade* CRs advance
task -d docs/beacon version                 # confirm v0.74.2 is live
task -d docs/beacon cleanup                 # kind delete
```

### Manual steps (reference)

The manual commands below match the tasks above 1:1. All run from the **repo
root**:

```bash
export ROOT=/path/to/solo-operator
cd "$ROOT"
```

| Role | Version | Set in |
|------|---------|--------|
| Deployed network | `0.74.0` | `docs/example/node{0..3}/consensus-capsule.yaml` (`imageTag`) |
| Upgrade package | `0.74.2` | `test/dev` `HEDERA_UPGRADE_VERSION` → `staging/build-v0.74.2.zip` |

**1. Build deployment zips + capture H**

The default build includes multi-registry support (primary + mirror) with
Sequential selection and a decoy at `registries[0]` to force failover. The
operator must skip the unpullable decoy and select a valid registry.

```bash
cd "$ROOT/test/dev"
task setup      # downloads build-v0.74.0.zip + build-v0.74.2.zip (needs internet)
CN_DECOY_REGISTRY=artifacts.hashgraph.io/consensus-node-docker-release-local/consensus-node:0.0.0-invalid \
CN_DECOY_POSITION_0=true \
CN_SELECTION_STRATEGY=Sequential \
task build      # -> staging/build-v0.74.0.zip and staging/build-v0.74.2.zip
export ZIP="$ROOT/test/dev/staging/build-v0.74.2.zip"
export H=$(shasum -a 384 "$ZIP" | awk '{print $1}')
echo "H=$H"
```

For a plain single-registry build (no failover), omit the `CN_*` env vars.

**2. Cluster + operator + network**

```bash
cd "$ROOT"
kind delete cluster --name solo || true
task kind:reset                              # creates 'solo-operator' + installs CRDs
```

Start the operator in its **own terminal**: `cd "$ROOT" && task run`

Build and load the UC sidecar (`:dev` is not published, `PullIfNotPresent`
needs it local):

```bash
cd "$ROOT"
task docker:build:uc
kind load docker-image ghcr.io/hashgraph/solo-operator/uc:dev --name solo-operator
```

Deploy a single-node network (one node reaches ACTIVE without starving a laptop):

```bash
cd "$ROOT/docs/example"
task setup
task deploy:single                           # node0 + genesis; waits for Ready
```

<details>
<summary>Alternative: full 4-node network (8+ CPU / 16+ GB)</summary>

```bash
cd "$ROOT/docs/example"
task setup
task deploy                                  # orbit + 4 nodes + genesis
kubectl apply -f node0/haproxycapsule.yaml
kubectl -n solo-orbit rollout status deploy/haproxy-node0 --timeout=120s
```

</details>

**3. beacon config + payer key**

```bash
cd "$ROOT"
task build:beacon                            # -> bin/beacon
mkdir -p /tmp/beacon-solo/keys

KEY_HEX=302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137
{ echo "-----BEGIN PRIVATE KEY-----"; echo "$KEY_HEX" | xxd -r -p | base64; echo "-----END PRIVATE KEY-----"; } > /tmp/beacon-solo/keys/account2.pem
: > /tmp/beacon-solo/keys/account2.pass

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

**4. Drive the upgrade**

Port-forward in a **separate terminal**: `kubectl -n solo-orbit port-forward svc/haproxy-node0 50211:50211`

```bash
cd "$ROOT"

# a) upload the v0.74.2 package into special file 0.0.150
./bin/beacon -c /tmp/beacon-solo/config.yaml file upload --zip "$ZIP" --expect-hash "$H"

# b) confirm the network holds that hash
./bin/beacon -c /tmp/beacon-solo/config.yaml file info

# c) stage the upgrade on every node
./bin/beacon -c /tmp/beacon-solo/config.yaml freeze prepare --upgrade-zip-hash "$H" --yes

# d) schedule the freeze (~3 min out, UTC)
START=$(date -u -v+3M '+%Y-%m-%d.%H:%M:%S' 2>/dev/null || date -u -d '+3 min' '+%Y-%m-%d.%H:%M:%S')
./bin/beacon -c /tmp/beacon-solo/config.yaml freeze upgrade --start-time "$START" --upgrade-zip-hash "$H" --yes
```

**5. Observe the upgrade** (beacon has exited; the pipeline runs on its own)

```bash
kubectl -n solo-orbit logs -f node0-0 -c consensus-node | grep -iE "freeze|upgrade"
kubectl -n solo-orbit logs -f node0-0 -c uc
kubectl -n solo-orbit get networkupgradeprepare,networkupgradefreeze,networkupgradeexecute -w
```

**6. Verify**

```bash
cd "$ROOT"
bin/beacon -c /tmp/beacon-solo/config.yaml network version   # expect 0.74.2
kubectl -n solo-orbit get pods -l app.kubernetes.io/component=consensus-node -o wide
```

**7. Cleanup:** `kind delete cluster --name solo-operator`

> **Boundary note.** Steps (a)–(d) are beacon's job. Everything in step 5 is the
> consensus node + UC + operator + provisioner reacting to the freeze. If the
> network does not return on `0.74.2`, inspect the `uc` container log and the
> NetworkUpgrade* CR phases.

---

## Test variants

The build step in the e2e walkthrough above uses `CN_*` env vars to control the
manifest the UC reads. Mix and match to exercise different operator paths. After
rebuilding, re-export `ZIP` and `H` and continue with the normal
[upgrade sequence](#4-drive-the-upgrade).

### Sequential selection + decoy failover (default above)

Decoy at `registries[0]`, Sequential strategy. The operator probes registries in
listed order — the decoy fails immediately, so the prepare selects the next valid
candidate.

```bash
CN_DECOY_REGISTRY=artifacts.hashgraph.io/consensus-node-docker-release-local/consensus-node:0.0.0-invalid \
CN_DECOY_POSITION_0=true \
CN_SELECTION_STRATEGY=Sequential \
task build
```

### Random selection

Random strategy, no decoy. The operator spreads pull traffic across all valid
registries — useful to verify that every candidate passes independently.

```bash
CN_SELECTION_STRATEGY=Random \
task build
```

### Plain single-registry (no failover)

Omit all `CN_*` vars to produce a manifest with only the primary registry and no
strategy override (falls back to the operator's `--registry-order` default).

```bash
task build
```

---

## Troubleshooting

- **`payer account N is not privileged`** — use `-p 2` (or 50/58) for uploads and
  freezes.
- **passphrase error in a Job** — add `account<N>.pass` to the `beacon-keys`
  Secret, or use an unencrypted PEM.
- **`freeze prepare` rejected / hash mismatch** — the `--upgrade-zip-hash` must
  equal what `file info` reports; re-upload if they differ.
- **TLS / cert errors against local nodes** — keep `tls: false` for plaintext
  `50211`; cert verification only engages for ledger-pinned networks.
