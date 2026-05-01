# Helm Chart Publishing — Align with hiero-velocity Pattern

## Context

`hashgraph/solo-operator` already publishes its Helm chart to GHCR OCI and JFrog as part of `flow-release-deploy-artifact.yaml`. However, compared to the `swirldslabs/hiero-velocity` reference implementation, it lacks:

1. **PR-time helm validation** — no `helm lint` / `helm template` check on pull requests
2. **Dry-run artifact upload** — dry-run releases silently skip the push with no inspectable output
3. **`[skip ci]` guard** — the semantic-release version-bump commit can trigger a recursive CI run
4. **Reusable workflow structure** — all release logic is in one monolithic job; hiero-velocity separates `release` and `helm-publish` into distinct jobs inside a reusable `zxc-release.yaml`
5. **Stale tool versions** — helm `v3.13.3`, Node 20, semantic-release v23 (hiero-velocity uses v3.18.0 / Node 22 / v24)

Note: CRD templates under `charts/solo-operator/charts/crds/templates/` are **not committed** to git; `task helm:sync-crds` must run before any `helm lint` or `helm package`.

---

## Files to Create

### 1. `.github/workflows/zxc-helm-check.yaml`
Reusable workflow called on every PR. Steps:
- Harden runner
- Checkout
- Setup Go (needed to install yq)
- Install yq: `go install github.com/mikefarah/yq/v4@latest`
- Setup Task
- Setup Helm (`v3.18.0`)
- `task helm:sync-crds` — regenerate CRD templates from `config/crd/bases/`
- `helm lint charts/solo-operator`
- `helm template solo-operator charts/solo-operator > /tmp/default.yaml`
- `helm template` with `crds.install=false` (to test without CRD subchart)
- `helm template` with `metrics.serviceMonitor.enabled=true` (tests optional resource)
- `helm template` with `replicaCount=2` (HA mode)
- Validate each rendered file is non-empty and contains `kind:` lines

Inputs: `helm-version` (default `v3.18.0`), `chart-path` (default `charts/solo-operator`)

### 2. `.github/workflows/zxc-release.yaml`
New reusable workflow replacing the inline release logic. Two jobs:

**Job `release`** (outputs: `version`, `released`):
- Harden runner, checkout (fetch-depth: 0), Node 22, GPG import, DCO hook
- Install semantic-release packages (v24.2.4 versions)
- Run `npx semantic-release [--dry-run]`
- Read `.release-version` → output `version` + `released=true/false`

**Job `helm-publish`** (`needs: release`, `if: released == 'true'`):
- Harden runner, checkout (fetch-depth: 0)
- Setup Go + yq, Task, Helm `v3.18.0`
- `task helm:sync-crds`
- Sync `Chart.yaml` version with `sed` (idempotent, guards with `grep -qE`)
- `helm lint`
- `helm package charts/solo-operator --destination "${RUNNER_TEMP}"` → output `path`, `name`, SHA-256 `digest`
- `helm registry login` to GHCR (skipped on dry-run)
- `helm registry login` to JFrog (skipped on dry-run)
- `helm push <pkg> oci://ghcr.io/hashgraph/charts` → chart lands at `ghcr.io/hashgraph/charts/solo-operator:<version>` (skipped on dry-run)
- `helm push <pkg> oci://<jf-registry>/solo-operator-helm-release` (skipped on dry-run)
- `actions/upload-artifact` with the `.tgz` when dry-run is true

Inputs: `dry-run` (bool), `node-version` (default `22`), `helm-version` (default `v3.18.0`), `helm-chart-path` (default `charts/solo-operator`)
Secrets passed through: `GH_ACCESS_TOKEN`, `GPG_KEY_CONTENTS`, `GPG_KEY_PASSPHRASE`, `JF_ACCESS_TOKEN`

---

## Files to Modify

### 3. `.github/workflows/flow-release-deploy-artifact.yaml`
Replace the inline `release` / helm-push logic with a call to `zxc-release.yaml`. Also update `GH_HELM_GROUP` from `hashgraph/solo-operator` → `hashgraph/charts`:
```yaml
jobs:
  release:
    uses: ./.github/workflows/zxc-release.yaml
    with:
      dry-run: ${{ inputs.dry-run-enabled }}
      helm-chart-path: charts/solo-operator
    secrets:
      GH_ACCESS_TOKEN: ${{ secrets.GH_ACCESS_TOKEN }}
      GPG_KEY_CONTENTS: ${{ secrets.GPG_KEY_CONTENTS }}
      GPG_KEY_PASSPHRASE: ${{ secrets.GPG_KEY_PASSPHRASE }}
      JF_ACCESS_TOKEN: ${{ secrets.JF_ACCESS_TOKEN }}

  docker-publish:
    needs: release
    if: ${{ needs.release.outputs.released == 'true' }}
    # ... keep Docker build+push steps here (inline or extract to zxc-docker-publish.yaml later)
```

The Docker build+push steps remain in `flow-release-deploy-artifact.yaml` for now (scoped to this PR; Docker refactor is separate work).

### 4. `.github/workflows/flow-pull-request-checks.yaml`
Add a new `helm-check` job:
```yaml
  helm-check:
    name: Helm Check
    uses: ./.github/workflows/zxc-helm-check.yaml
```
No `needs` dependency — runs in parallel with existing `build` job.

### 5. `.releaserc.json`
Two changes:
1. **Add `[skip ci]` to the git commit message** in `@semantic-release/git`:
   ```json
   "message": "chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}"
   ```
2. **Remove `helm package` from the `prepare` exec step** — packaging moves to the `helm-publish` workflow job. Keep the `sed` version update and `.release-version` write. Remove `solo-operator-*.tgz` from the `publish.assets` list (chart is distributed via OCI; dry-run produces a downloadable artifact).

   Updated prepare exec cmd:
   ```bash
   set -e
   sed -i "s/^version:.*/version: ${nextRelease.version}/" charts/solo-operator/Chart.yaml
   sed -i "s/^appVersion:.*/appVersion: \"${nextRelease.version}\"/" charts/solo-operator/Chart.yaml
   echo "${nextRelease.version}" > .release-version
   ```

---

## Critical File Paths

| File | Action |
|------|--------|
| `.github/workflows/zxc-helm-check.yaml` | **Create** |
| `.github/workflows/zxc-release.yaml` | **Create** |
| `.github/workflows/flow-release-deploy-artifact.yaml` | **Modify** — delegate to zxc-release.yaml |
| `.github/workflows/flow-pull-request-checks.yaml` | **Modify** — add helm-check job |
| `.releaserc.json` | **Modify** — remove helm package, add [skip ci] |

Existing utilities to reuse:
- `task helm:sync-crds` — `Taskfile.yml` task, syncs CRDs before lint/package
- `azure/setup-helm` action — already pinned in `flow-release-deploy-artifact.yaml`
- `step-security/harden-runner` — already pinned in existing workflows
- `actions/checkout@de0fac2e...` — already pinned in existing workflows

---

## Verification

1. **PR helm check**: Open a test PR → confirm the new `Helm Check` job appears and passes
2. **Dry-run release**: `workflow_dispatch` with `dry-run-enabled: true` → confirm `helm-chart-<version>` artifact appears in the run summary
3. **Real release**: `workflow_dispatch` with `dry-run-enabled: false` → confirm chart pushed to `oci://ghcr.io/hashgraph/charts/solo-operator:<version>` and JFrog
4. **Skip-CI guard**: After a real release, confirm the version-bump commit does NOT trigger a new workflow run (check the commit message contains `[skip ci]`)
5. **Lint locally**: `task helm:sync-crds && helm lint charts/solo-operator`
