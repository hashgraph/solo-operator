# Helm Chart Publishing

## Context

`hashgraph/solo-operator` already published its Helm chart as part of `flow-release-deploy-artifact.yaml`, but the release logic was a single monolithic job. This change restructures it into reusable, composable workflows and adds PR-time chart validation.

Problems addressed:
1. **PR-time helm validation** — no `helm lint` / `helm template` check on pull requests
2. **Dry-run artifact upload** — dry-run releases silently skipped the push with no inspectable output
3. **`[skip ci]` guard** — the semantic-release version-bump commit could trigger a recursive CI run
4. **Monolithic job** — release, helm-publish, and docker-publish were all inline in one job
5. **Outdated tool versions** — upgraded helm `v3.13.3` → `v3.18.0`, Node 20 → 22, semantic-release v23 → v24
6. **Confusing GHCR namespace** — chart was pushed to `ghcr.io/hashgraph/solo-operator/solo-operator` (same prefix as Docker image); changed to `ghcr.io/hashgraph/charts/solo-operator`

Note: CRD templates under `charts/solo-operator/charts/crds/templates/` are **not committed** to git; `task helm:sync-crds` must run before any `helm lint` or `helm package`.

---

## Files Created

### `.github/workflows/zxc-helm-check.yaml`
Reusable workflow called on every PR:
- Setup Go + yq + Task + Helm
- `task helm:sync-crds` — regenerate CRD templates from `config/crd/bases/`
- `helm lint charts/solo-operator`
- `helm template` with default values, CRDs disabled, ServiceMonitor enabled, and HA mode (`replicaCount=2`)
- Validate each rendered file is non-empty and contains `kind:` lines

### `.github/workflows/zxc-release.yaml`
Reusable workflow with two jobs:

**Job `release`** (outputs: `version`, `released`):
- GPG import, DCO hook, Node 22, semantic-release v24.2.4
- Outputs `version` + `released=true/false` from `.release-version`

**Job `helm-publish`** (`needs: release`, `if: released == 'true'`):
- `task helm:sync-crds`
- Sync `Chart.yaml` version via `sed` with `grep -qE` guards
- `helm lint` + `helm package --destination "${RUNNER_TEMP}"` + SHA-256 digest
- Push to `oci://ghcr.io/hashgraph/charts` and JFrog (skipped on dry-run)
- Upload `.tgz` as workflow artifact when dry-run is true

---

## Files Modified

### `.github/workflows/flow-release-deploy-artifact.yaml`
- Delegates to `zxc-release.yaml` via `secrets: inherit`
- `docker-publish` job depends on `release` outputs
- Top-level permissions updated to `contents: write, issues: write, pull-requests: write, packages: write` (required by the reusable workflow)

### `.github/workflows/flow-pull-request-checks.yaml`
- Added `helm-check` job (calls `zxc-helm-check.yaml`), runs in parallel with `build`

### `.releaserc.json`
- Removed `helm package` from prepare step (packaging moved to `helm-publish` job)
- Added `[skip ci]` to `@semantic-release/git` commit message
- Removed `solo-operator-*.tgz` from GitHub release assets (chart distributed via OCI)

### `README.md`
- Updated all GHCR install paths from `oci://ghcr.io/hashgraph/solo-operator/solo-operator` to `oci://ghcr.io/hashgraph/charts/solo-operator`

---

## Verification

1. **PR helm check**: Open a PR → confirm `Helm Check` job appears and passes
2. **Dry-run release**: `workflow_dispatch` with `dry-run-enabled: true` → confirm `helm-chart-<version>` artifact appears in the run summary
3. **Real release**: `workflow_dispatch` with `dry-run-enabled: false` → confirm chart pushed to `oci://ghcr.io/hashgraph/charts/solo-operator:<version>` and JFrog
4. **Skip-CI guard**: After a real release, confirm the version-bump commit does not trigger a new run (`[skip ci]` in commit message)
5. **Lint locally**: `task helm:sync-crds && helm lint charts/solo-operator`
