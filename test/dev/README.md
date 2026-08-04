# Building a `build.zip` for the UC sidecar

This directory builds a Hedera consensus-node deployment package (`build.zip`)
for exercising the solo-operator **UC sidecar** locally.

The package mirrors the layout the consensus node writes into its
`--marker-path` and additionally embeds a UC manifest at
`manifests/consensus-node-components.yaml` carrying **deterministic layer
hashes** — the RootFS `diff_ids` of the published consensus-node image, per
platform. These are exactly what the operator recomputes and verifies in
[`internal/registry/checker.go`](../../internal/registry/checker.go), so a
package built here passes deterministic verification.

> ⚠️ **Test-only credentials.** [`overlay/`](overlay/) contains private keys and
> certs (`data/keys/*.pem`, `hedera.key`, `hedera.crt`) vendored from the public
> [hiero-local-node](https://github.com/hiero-ledger/hiero-local-node) project.
> They are **public, well-known test fixtures — not secrets** — and must **never
> be used in production**. Any secret-scanner finding on them is a false
> positive (see [overlay/README.md](overlay/README.md)).

## Prerequisites

- `task` ([taskfile.dev](https://taskfile.dev))
- `git`, `curl`, `unzip`, `zip`
- `crane` and `jq` (for the layer-hash step):
  ```bash
  go install github.com/google/go-containerregistry/cmd/crane@latest
  ```
- Read access to `gcr.io/hedera-registry/consensus-node` (the layer hashes are
  read from the live image config).

## Usage

```bash
cd test/dev

task setup    # download both source zips (HEDERA_BASE_VERSION + HEDERA_UPGRADE_VERSION)
task build    # produce staging/build-v<base>.zip and staging/build-v<upgrade>.zip
```

Two **real** versions are built (set `HEDERA_BASE_VERSION` / `HEDERA_UPGRADE_VERSION`
at the top of `Taskfile.yml`; defaults `0.74.0` and `0.74.2`). Each must exist both
as a published source zip on builds.hedera.com and as a consensus-node container
image (the UC manifest pins the image + layer hashes).

Outputs land in `staging/`:

| File | Purpose |
|------|---------|
| `build-v<base>.zip`    | the version the network is deployed at (kept for other tests) |
| `build-v<upgrade>.zip` | the next version — uploaded via beacon (0.0.150) / NMT to drive an upgrade |

Each zip carries its own real `VERSION` and a `manifests/consensus-node-components.yaml`
pinned to that version's image.

## Configuration

Override any of these on the command line, e.g. `task build HEDERA_VERSION=0.72.1`:

| Var | Default | Meaning |
|-----|---------|---------|
| `HEDERA_VERSION` | `0.74.0` | source package version to download/extract |
| `CN_IMAGE`       | `gcr.io/hedera-registry/consensus-node` | image the manifest points at |
| `CN_VERSION`     | `{{ HEDERA_VERSION }}` | tag whose `diff_ids` become the layer hashes |
| `CN_PLATFORMS`   | `linux/amd64 linux/arm64` | platforms to emit hashes for (missing ones are skipped) |

## Generating just the manifest

To (re)write only the manifest — e.g. into a marker dir you're testing the UC
binary against — without rebuilding the zip:

```bash
task build:uc-manifest OUT_DIR=/path/to/markers/manifests
```

## Tasks

| Task | Description |
|------|-------------|
| `setup`            | fetch hiero-local-node config + source zip |
| `build`            | full build → `staging/*.zip` (runs `setup`) |
| `build:hedera-zip` | extract, overlay config, embed manifest, re-zip |
| `build:uc-manifest`| generate `consensus-node-components.yaml` with deterministic layer hashes |
| `clean`            | remove `extracted/` and `staging/` |
