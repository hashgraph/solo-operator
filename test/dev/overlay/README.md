# Vendored node-config overlay

These files are overlaid onto the extracted consensus-node package by
`task build` (see `../Taskfile.yml`). They are vendored here so the build has
**no external clone dependency** — the only network download is the
consensus-node software zip.

## Provenance

Copied verbatim from [hiero-local-node](https://github.com/hiero-ledger/hiero-local-node)
at commit `e2a7aeaa6664bc8eef580e16687ca9eea00e0d0d`, path
`compose-network/network-node/`.

These are **public test fixtures** (default local-node config, well-known test
keys/certs) — not secrets. They exist only to make the package installable for
local UC sidecar testing and must never be used for a real network.

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
