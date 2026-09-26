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

## Flip lifecycle = versioned migration in solo-provisioner

A gate flip is a **versioned, idempotent migration run by solo-provisioner** — not
a manual `kubectl edit` — executed in the infra-upgrade window alongside the
solo-operator upgrade.

- **The operator upgrade carries the flip.** The release cut for the new era
  defaults the gate on; operator + chart + UC ship lockstep, so it is one
  coordinated version bump.
- **The migration handles residue + safety, idempotently:** clear stale
  overrides, force any regeneration the flip requires, and verify the rollout
  reports the new behavior. Re-runnable; no-ops when already migrated.
- **What the migration requires depends on the gate's coupling:**
  - **Component-coupled** (e.g. node-name ↔ CN version) → requires infra-upgrade
    **and** operator upgrade, because the paired component version and a pod
    recreate are both preconditions.
  - **Operator-only** (an internal reconcile toggle) → requires just the operator
    upgrade; the infra step no-ops.

Keeping every flip on the same migration harness — even operator-only ones — gives
one repeatable, version-gated, auditable path instead of ad-hoc edits.

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

- **Alpha:** explicit gate, default off; solo-provisioner sets it in lockstep.
- **Beta:** operator owns the default; the gate becomes an override; the
  solo-provisioner migration is idempotent and version-gated.
- **GA:** remove the gate and hardcode the behavior; drop gate-writing/override
  handling in solo-provisioner. An idempotent migration may remain — it simply
  no-ops once every cluster has migrated.

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

- **Alpha (this PR):** `ConsensusFeatureGates.NodeNameZeroBased` (bool, default
  false); both sites routed through `FormatNodeName`; behavior byte-identical
  (1-based). solo-provisioner mirrors `ConsensusNodeName` but leaves the gate false.
- **Beta:** operator adds `defaultNodeNameZeroBased` (still false until the paired
  CN baseline loads 0-based); gate becomes a `*bool` override; a pure flip must
  update the pod template (forces RollingUpdate) and regenerate the public-keys
  ConfigMap; operator surfaces the resolved convention for verification. The
  operator release cut alongside the 0-based CN baseline flips the default, so the
  infra-upgrade flips naming fleet-wide with zero Orbit edits. solo-provisioner
  ships a consensus-scope `VersionMigration` (minVersion = that release) that
  clears stale overrides, triggers public-keys regeneration, and verifies the
  rollout.
- **GA:** delete the gate; hardcode `FormatNodeName` to 0-based.

### Rollback

- Redeploy the prior operator (default false) + prior CN; pods recreate; naming
  reverts.
- The `*bool` override (Beta onward) pins either direction — e.g. hold a single
  cluster while the fleet migrates.
