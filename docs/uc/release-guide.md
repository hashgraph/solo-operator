# UC Sidecar — Release Guide

The Upgrade Controller (UC) sidecar ships as its own artifact —
`ghcr.io/hashgraph/solo-operator-uc` — versioned **independently** of the operator.
Because UC is released rarely, the release is **hand-cut**: a maintainer chooses the
version at dispatch time instead of deriving it from commit messages. This guide
describes how to cut a UC release.

> The **operator** release is a separate process (semantic-release, `v*` tags) — see
> [Release Management](../../README.md#release-management). Nothing in this guide
> affects it.

## Versioning

- UC tags use the form **`uc-v<MAJOR>.<MINOR>.<PATCH>`** (e.g. `uc-v0.32.0`),
  independent of the operator's `v*` tags.
- The container image is tagged with the bare semver (`:0.32.0`) plus `:latest`.
- The chosen version is stamped into the binary at build time, so `uc version` inside
  the image reports the released `uc-v*` string.

### What version do I enter?

A UC release must be a **single-step increment** of the latest `uc-v*` tag. Given a
previous release of `0.31.1`, exactly three versions are valid:

| Bump  | Enter   | Use when |
|-------|---------|----------|
| patch | `0.31.2` | bug fixes, no behaviour change |
| minor | `0.32.0` | backwards-compatible changes |
| major | `1.0.0`  | breaking changes |

Anything else is **rejected** by the workflow before it builds anything. For example
`0.31.1 → 3.1.1` fails — it skips major versions and doesn't reset minor/patch. The
error message always lists the three versions you may enter.

The **first** release (no `uc-v*` tag yet) is compared against `0.0.0`, so `0.0.1`,
`0.1.0`, or `1.0.0` are the valid choices — enter one explicitly.

## Cutting a release

1. **Dry run first (recommended).** Run the
   [Release: Deploy Artifact (UC Sidecar)](https://github.com/hashgraph/solo-operator/actions/workflows/flow-release-deploy-uc.yaml)
   workflow with **Perform Dry Run** enabled. It computes the version and previews the
   release notes in the job summary — no tag, build, or push.

2. **Choose the version.** Either:
   - leave **version** empty and pick a **bump** (`patch` default / `minor` / `major`),
     which increments the latest `uc-v*` tag automatically; or
   - enter an exact **version** (e.g. `0.32.0`) — it must be one of the valid
     increments above.

3. **Run the workflow** (dry run off). In order, it will:
   - build both-arch UC binaries with the chosen version stamped in;
   - push the multi-arch image to `ghcr.io/hashgraph/solo-operator-uc` and the JFrog
     mirror, tagged `:<version>` and `:latest`;
   - create the annotated git tag `uc-v<version>`;
   - create a GitHub release — marked **not** "latest" so it doesn't displace the
     operator release — with notes generated from `(uc)`-scoped commits since the
     previous UC tag, attaching the binaries and their SHA-256 checksums.

The tag and GitHub release are created **last**, so they only appear once the image
push has succeeded.

## Release notes

Notes are auto-generated from commit subjects with a `(uc)` scope
(`feat(uc): …`, `fix(uc): …`, …) since the previous `uc-v*` tag. Commits without a
`(uc)` scope are omitted; if none are found the release is still cut with a
placeholder note. You can edit the GitHub release body afterwards at any time.

> The `(uc)` scope only flavours the release-notes text — it does **not** gate whether
> a release happens (the manual dispatch is the gate). You never need a `(uc)` commit
> to cut a release.

## Verifying

- **Image:** `docker run --rm ghcr.io/hashgraph/solo-operator-uc:<version> version` —
  it should print `uc-v<version>`.
- **Tag / release:** confirm the `uc-v<version>` tag and its GitHub release exist, with
  the two binaries and their `.sha256` files attached.

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `is not a valid increment … Valid next versions are …` | The version isn't a clean patch/minor/major step from the latest tag — enter one of the three listed values. |
| `is not a valid semantic version` | Malformed input (needs `MAJOR.MINOR.PATCH`, no leading zeros). |
| `tag … already exists` | That `uc-v*` tag was already released — pick the next version. |
| First release | No `uc-v*` tags yet; enter `0.1.0` or `1.0.0` explicitly (or use `bump`, which yields `0.0.1`). |

The workflow requires the `GH_ACCESS_TOKEN`, `JF_ACCESS_TOKEN`, and JFrog registry
secrets/vars, which are configured at the repository/organisation level.
