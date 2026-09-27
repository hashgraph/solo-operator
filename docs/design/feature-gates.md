# Feature gates

Convention for transitional, rollout-only feature gates on the Orbit CRD: how they
differ from configuration, where they live, and how a flip is executed.

## Configuration vs. feature gate

- **Configuration** = permanent desired state ("what to deploy"): `ledgerId`,
  `chainId`, `provisioningModel`, `requireDigestOnDeploy`,
  `provisionerDaemonEnabled`, telemetry, etc. Lives in the spec config fields and
  is not removed on a schedule.
- **Feature gate** = transitional control over "which code path during rollout".
  Graduates Alpha→Beta→GA and is then **removed**.

## Scope-tiered, typed structs

One typed gate struct per scope, added only when that scope gets its first gate —
never ship an empty struct:

| Scope | Path | Type |
|-------|------|------|
| Consensus | `orbit.spec.consensus.featureGates` | `ConsensusFeatureGates` |
| Network-wide | `orbit.spec.featureGates` | `OrbitFeatureGates` (add with first gate) |
| Other component | `orbit.spec.<component>.featureGates` | `<Component>FeatureGates` |

Typing each struct to its scope keeps a consensus gate from leaking into
network-wide config and makes `kubectl explain` group gates where they apply.

## Per-gate doc convention

Every gate's doc comment carries **maturity stage + default + removal criteria**,
so a gate reads as visibly temporary versus permanent config. Example:
`ConsensusFeatureGates.NodeNameZeroBased` in
[orbit_types.go](../../api/v1alpha1/orbit_types.go).

## Resolution: operator is a dumb reader

- The gate is a plain `bool` (default `false` = old behavior). The operator **only
  honors the value** — it does not track component versions, own an era, or flip a
  built-in default. Resolved via an accessor (e.g. `Orbit.IsNodeNameZeroBased()`).
- **solo-provisioner sets the gate** to match the deployed component, on both fresh
  installs and upgrades. All version-coupling ("which CN needs which convention")
  lives there, in one place.

Because the operator has no auto-flipping default, a plain `bool` is enough — a
persisted `false` blocks nothing (solo-provisioner overwrites it to `true` when it
flips), so there is no need for a nullable `*bool` and no data-cleanup migration.

## Flip lifecycle: solo-provisioner sets the gate in-window

The gate's code ships **inert** in the operator (default off). Activating the new
behavior needs only the gate set — so the flip is driven by **solo-provisioner
writing the gate during the upgrade window, with no operator upgrade required**.

- **No operator upgrade to flip.** The already-running operator (any release ≥ the
  one that introduced the gate) reads the new value and acts on it. You upgrade the
  operator only for actual operator changes — one fewer deployment per flip.
- **Window = component upgrade + solo-provisioner migration.** For a
  component-coupled gate, solo-provisioner upgrades the component (e.g. the CN) and
  sets the gate together; the single pod recreate that applies the change rides the
  component's image upgrade, so there is no extra restart and no re-keying.
- **Fresh installs** get the gate set at create time to match the component being
  deployed, so a new cluster is correct from the start.
- **Idempotent + version-gated.** The migration self-skips once past the version
  boundary (see below), so it is safe to re-run.

### How it maps to solo-provisioner's migration framework

solo-provisioner runs migrations through a small registry (see its
`internal/migration` package). Each migration implements `Applies`, `Execute`, and
`Rollback`, is registered per scope in chronological order, and composes into an
automa workflow that rolls back on error. State is passed via a context bag (with
well-known keys such as installed/target version), not held on the migration.

- **Idempotency + version-gating live in `Applies`.** The `VersionMigration` base
  applies only when crossing a boundary — installed `< minVersion` and target
  `>= minVersion` — so it self-skips once past that boundary. A gate flip registers
  with `minVersion` = the component release that introduces the new behavior.
- **`Execute` sets the gate** (and triggers any regeneration the flip needs) and
  verifies the rollout reports the new behavior.
- **`Rollback`** restores the prior gate value (best-effort) for the workflow's
  rollback-on-error path.
- **Scope decides when it runs:** startup-scope migrations run on every invocation
  before commands; component-scope migrations (e.g. consensus) run during the
  upgrade workflow. A component-coupled gate registers under its component scope,
  and any host-level step is enqueued on the solo-provisioner daemon.

## Graduation

- **Alpha:** gate ships inert (default `false`); solo-provisioner may set it to
  validate the new behavior against a compatible component. Forward-compat seam.
- **Beta:** solo-provisioner drives the flip via a version-gated migration that sets
  the gate `true` when it deploys a component `>=` the threshold; the operator stays
  a dumb reader (no operator upgrade required to flip).
- **GA:** remove the gate and hardcode the behavior; drop gate handling in
  solo-provisioner. Leftover gate values are pruned by the apiserver on next write
  and ignored by the operator.

## FAQ — why it is shaped this way

**If an operator upgrade can change behavior, why put the gate in Orbit at all?**
Because the gate lets you flip in the upgrade window **without** an operator
upgrade: the code ships inert in the operator, and setting the gate activates it.
That means one fewer deployment per flip, plus a per-Orbit knob, an Alpha
validation seam, in-place rollback, and GitOps-visible intent — none of which a
binary-only default gives.

**Do we upgrade the operator to flip a gate?**
No. solo-provisioner sets the gate during the window and the already-running
operator honors it. Upgrade the operator only for real operator changes.

**Does the operator own the flip or track component versions?**
No. The operator is a dumb reader: honor the gate, default to old behavior when
unset. All version-coupling lives in solo-provisioner's migration, which knows the
component version it deploys.

**Why a plain `bool` and not a nullable `*bool`?**
Because the operator has no auto-flipping default to protect. `unset` and `false`
both mean "old behavior"; solo-provisioner writes `true` to flip. A tri-state
`*bool` only mattered in an operator-default-auto-flip model (where a persisted
`false` would block the flip) — which we dropped. Plain `bool` is simpler and
self-documents `default: false`.

**What if the gate and the deployed component disagree (e.g. gate `false` but the
CN loads 0-based)?** The node breaks — the gate must match the component. That
coupling is inherent, not a property of the field type, which is exactly why
solo-provisioner sets the gate to match the component it deploys.

**Does flipping cause downtime or re-keying?**
No re-keying — the affected artifacts are operator-generated file *names*, not key
material. The single pod recreate that applies the new names rides the component's
image upgrade, so there is no extra restart.

## Worked example: node-name convention (`nodeNameZeroBased`)

The first gate switches consensus gossip key-file naming from 1-based
(`node<id+1>`) to 0-based (`node<id>`), in lockstep with a consensus-node (CN)
version whose key loader reads the new names. It is a **component-coupled** gate.

- **Gate:** `orbit.spec.consensus.featureGates.nodeNameZeroBased`
  ([orbit_types.go](../../api/v1alpha1/orbit_types.go)).
- **Encoder:** `common.FormatNodeName(nodeId, zeroBased)`
  ([node.go](../../internal/common/node.go)) — the single source of truth;
  solo-provisioner mirrors it as `ConsensusNodeName(id, zeroBased)`.
- **Coordination:** key-management epic (#1164 / #1169); tracking issue #1336.

### What physically changes on flip

Both affected artifacts are **operator-generated at deploy time**, not persisted
re-keyable state:

- **Private-key mount path** (StatefulSet projected volume):
  `s-private-node<id+1>.pem` → `s-private-node<id>.pem`
  ([deployment.go](../../internal/resources/consensus/deployment.go)).
- **Public-keys ConfigMap keys**: `s-public-node<id+1>.pem` → `s-public-node<id>.pem`
  ([public_keys.go](../../internal/operations/genesis/public_keys.go)).

The source secrets keep arbitrary names; the operator normalizes them. A flip is
therefore **naming-only** — no re-keying and no PVC data migration. Applying it
needs only a pod recreate (which the CN image upgrade performs anyway) plus a
public-keys regeneration.

### Per-phase specifics

- **Alpha (this PR):** `ConsensusFeatureGates.NodeNameZeroBased` (plain `bool`,
  default `false`); both sites routed through `FormatNodeName`; behavior
  byte-identical (1-based). solo-provisioner mirrors `ConsensusNodeName` and leaves
  the gate `false`.
- **Beta:** solo-provisioner ships a consensus-scope `VersionMigration`
  (`minVersion` = the CN release that loads 0-based names) that sets the gate `true`
  and verifies the rollout, alongside the CN upgrade — no operator upgrade needed to
  flip. The operator remains a dumb reader.
- **GA:** delete the gate; hardcode `FormatNodeName` to 0-based.

### Rollback

- Within a compatible window, solo-provisioner sets the gate back to `false` (and
  redeploys the prior CN); pods recreate and naming reverts.
