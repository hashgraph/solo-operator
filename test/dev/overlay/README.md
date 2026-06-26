# Vendored node-config overlay

These files are overlaid onto the extracted consensus-node package by
`task build` (see `../Taskfile.yml`). They are vendored here so the build has
**no external clone dependency** — the only network download is the
consensus-node software zip.

## Provenance

Copied verbatim from [hiero-local-node](https://github.com/hiero-ledger/hiero-local-node)
at commit `e2a7aeaa6664bc8eef580e16687ca9eea00e0d0d`, path
`compose-network/network-node/`.

## ⚠️ TEST-ONLY CREDENTIALS — NOT SECRETS

This directory intentionally contains private keys and certificates:

- `data/keys/s-private-node{1..4}.pem`, `data/keys/s-public-node{1..4}.pem`
- `hedera.key`, `hedera.crt`

**These are well-known, publicly published fixtures** from the open-source
[hiero-local-node](https://github.com/hiero-ledger/hiero-local-node) project
(Apache-2.0). They are committed deliberately so the package is installable for
local UC sidecar testing **without any external clone**.

They are **NOT secrets** and carry **no security value**:

- they are identical for every checkout of hiero-local-node and this repo,
- they protect nothing — no real network, account, or environment uses them,
- they **must NEVER be used in production** or any non-local environment.

Any secret-scanning / security-audit finding on these files is a **false
positive** and may be suppressed with this note as justification.

## Refreshing

The source commit is pinned in `../Taskfile.yml` as `HIERO_LOCAL_NODE_COMMIT`
(currently `e2a7aeaa6664bc8eef580e16687ca9eea00e0d0d`). To update:

```bash
# bump HIERO_LOCAL_NODE_COMMIT in ../Taskfile.yml, then:
task overlay:update            # re-vendors from that commit (preserves this README)
# or pin ad hoc without editing the file:
task overlay:update HIERO_LOCAL_NODE_COMMIT=<commit>
```

After updating, set the commit above to match and commit the diff.
