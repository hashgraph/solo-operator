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
# 0. (optional) build the artifact + print H, no network
beacon package software-zip --from <dir> --out softwareUpgrade.zip

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

The zip is typically built on the host by [`test/dev`](../../test/dev)
(`task build` → `staging/build-v9.99.99.zip`). Pick the delivery that matches your
cluster:

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
( cd test/dev && task build )                       # writes staging/build-v9.99.99.zip → node /var/beacon
# point job-upload-hostpath.yaml's --zip at /work/build-v9.99.99.zip, then:
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

## Troubleshooting

- **`payer account N is not privileged`** — use `-p 2` (or 50/58) for uploads and
  freezes.
- **passphrase error in a Job** — add `account<N>.pass` to the `beacon-keys`
  Secret, or use an unencrypted PEM.
- **`freeze prepare` rejected / hash mismatch** — the `--upgrade-zip-hash` must
  equal what `file info software-zip` reports; re-upload if they differ.
- **TLS / cert errors against local nodes** — keep `tls: false` for plaintext
  `50211`; cert verification only engages for ledger-pinned networks.
