# Feature gates

Convention for transitional, rollout-only feature gates on the Orbit CRD: how
they differ from configuration, where they live, and how a flip is executed.

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

## Resolution: operator default + override

- **Operator default** — a compile-time constant in the operator encodes the
  current era's behavior and tracks the paired component baseline the operator
  release ships with.
- **Override** — the Orbit gate is a nullable `*bool`: unset ⇒ operator default;
  set ⇒ force. Resolved via an accessor (e.g. `Orbit.IsNodeNameZeroBased()`).

The override is the manual escape hatch; the default is what makes a flip happen
automatically on operator upgrade.

> Ship boolean gates as a nullable `*bool` with **no** `kubebuilder` default (and
> non-boolean gates likewise optional, with no persisted default), from Alpha. A
> materialized default would persist an explicit value into every Orbit, which
> later reads as a stale override and blocks the operator-default flip — forcing a
> data-cleanup migration. With no persisted default, unset Orbits follow the
> operator default automatically and the flip needs no data rewrite.

### Explicit overrides are a temporary hold, not a permanent pin

An override is only valid **within a component-compatible window** — use it to hold
a single cluster briefly (canary, rollback), not to pin a convention forever.

- **Alpha:** an explicit value equal to the default is harmless.
- **Beta:** once the infra-upgrade brings the new component baseline and the
  operator default flips, an override pinning the **old** value is now incompatible
  with the deployed component (for node-name: the CN loads 0-based names but the
  operator would mount 1-based → the node can't load its keys). The safety-net
  migration's main job is to clear such incompatible overrides (→ unset) so the
  Orbit follows the operator default; the operator may also surface a warning. This
  is why the "optional" migration is worth keeping — it earns its keep for clusters
  that set an explicit value.
- **GA:** the field is removed from the CRD and unread; the apiserver prunes any
  leftover value on next write and behavior is hardcoded. A stale override has no
  effect.

## Flip lifecycle

The flip is **carried by the operator upgrade** and applied when pods recreate
during the infra-upgrade window — not by a manual `kubectl edit`. Because unset
Orbits defer to the operator default (see §Resolution), a redeploy moves the fleet
on its own, with no Orbit data rewrite.

solo-provisioner additionally ships a **versioned, idempotent migration as a safety
net** — optional, not on the critical path, but worth having.

- **The operator upgrade carries the flip.** The release cut for the new era
  defaults the gate on; operator + chart + UC ship lockstep, so it is one
  coordinated version bump.
- **The migration is an optional safety net, idempotent:** verify the rollout
  reports the new behavior, force any regeneration the flip needs, and clean up
  the rare explicit override that would pin the old value. Re-runnable; no-ops when
  nothing needs doing.
- **What the flip requires depends on the gate's coupling:**
  - **Component-coupled** (e.g. node-name ↔ CN version) → requires infra-upgrade
    **and** operator upgrade, because the paired component version and a pod
    recreate are both preconditions.
  - **Operator-only** (an internal reconcile toggle) → requires just the operator
    upgrade; the infra step no-ops.

Running every flip's safety net through the same migration harness gives one
repeatable, version-gated, auditable path.

### How it maps to solo-provisioner's migration framework

solo-provisioner runs migrations through a small registry (see its
`internal/migration` package). Each migration implements `Applies`, `Execute`, and
`Rollback`, is registered per scope in chronological order, and composes into an
automa workflow that rolls back on error. State is passed via a context bag (with
well-known keys such as installed/target version), not held on the migration.

- **Idempotency + version-gating live in `Applies`.** The `VersionMigration` base
  applies only when crossing a boundary — installed `< minVersion` and target
  `>= minVersion` — so it self-skips once the fleet is past that boundary. A gate
  flip registers with `minVersion` = the solo-operator/CN release that defaults the
  gate on.
- **`Execute` does the residue + safety work:** clear stale overrides, trigger any
  regeneration the flip needs, verify the rollout reports the new behavior.
- **`Rollback`** restores (best-effort) for the workflow's rollback-on-error path.
- **Scope decides when it runs:** startup-scope migrations run on every invocation
  before commands; component-scope migrations (e.g. consensus) run during the
  upgrade workflow. A component-coupled gate registers under its component scope,
  and any host-level step is enqueued on the solo-provisioner daemon.

## Graduation

- **Alpha:** nullable gate, operator default off; left unset in the happy path
  (override available). Inert forward-compat seam.
- **Beta:** operator owns the default and flips it in lockstep with the paired
  component; the gate stays a nullable override. An optional idempotent
  solo-provisioner migration provides verification + edge cleanup.
- **GA:** remove the gate and hardcode the behavior; drop override handling in
  solo-provisioner. The optional migration may remain — it simply no-ops once
  every cluster has migrated.

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
needs only a pod recreate (which the infra-upgrade performs anyway) plus a
public-keys regeneration.

### Per-phase specifics

- **Alpha (this PR):** `ConsensusFeatureGates.NodeNameZeroBased` (nullable `*bool`,
  no persisted default); both sites routed through `FormatNodeName`; behavior
  byte-identical (1-based). solo-provisioner mirrors `ConsensusNodeName` and leaves
  the gate unset.
- **Beta:** operator adds `defaultNodeNameZeroBased` (still false until the paired
  CN baseline loads 0-based); a pure flip must update the pod template (forces
  RollingUpdate) and regenerate the public-keys ConfigMap; operator surfaces the
  resolved convention for verification. The operator release cut alongside the
  0-based CN baseline flips the default, so the infra-upgrade flips naming
  fleet-wide with zero Orbit edits — unset Orbits follow automatically, no data
  migration required. solo-provisioner may ship an optional consensus-scope
  `VersionMigration` (minVersion = that release) that verifies the rollout and
  cleans up any explicit override that pins the old value.
- **GA:** delete the gate; hardcode `FormatNodeName` to 0-based.

### Rollback

- Redeploy the prior operator (default false) + prior CN; pods recreate; naming
  reverts.
- The `*bool` override (Beta onward) pins either direction — e.g. hold a single
  cluster while the fleet migrates.
