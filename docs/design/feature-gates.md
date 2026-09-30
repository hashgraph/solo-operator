# Feature gates

Feature gates are **temporary switches** on the Orbit CRD for rolling out new
behavior safely. Unlike configuration (permanent desired state), a gate is deleted
once the new behavior is everywhere.

## Configuration vs. feature gate

- **Configuration** — permanent desired state ("what to deploy"): `ledgerId`,
  `chainId`, `provisioningModel`, `requireDigestOnDeploy`,
  `provisionerDaemonEnabled`, telemetry, … Never removed on a schedule.
- **Feature gate** — a temporary switch over "which code path during rollout".
  Removed once the new behavior is everywhere.

## Where gates live

One typed struct per scope, added only when that scope gets its first gate (never
ship an empty struct):

| Scope | Path | Type |
|-------|------|------|
| Consensus | `orbit.spec.consensus.featureGates` | `ConsensusFeatureGates` |
| Network-wide | `orbit.spec.featureGates` | `OrbitFeatureGates` (add with first gate) |
| Other component | `orbit.spec.<component>.featureGates` | `<Component>FeatureGates` |

Scoping keeps a consensus gate out of network-wide config and makes `kubectl
explain` group gates where they apply. Each gate's doc comment states its
**default** and **when to remove it**, so it reads as clearly temporary.

## How a gate works

- Each gate is a plain `bool`, default `false` (the old behavior).
- The **operator only reads it** — it never decides when to flip and never tracks
  component versions.
- **Someone else sets it** to match the deployed component. So changing behavior
  needs no operator upgrade: the new code already ships in the operator (switched
  off), and setting the gate turns it on.

## How a flip happens

A flip is done **in-cluster during the upgrade window** by whoever sets the gate —
the operator is agnostic:

- **solo-provisioner** — when it orchestrates the whole upgrade (component + gate
  together).
- **An in-cluster job** — the HIP jobs capability (`build/jobs/`), for GitOps-only
  setups.

Either way:

- **No operator upgrade to flip** — one fewer deployment.
- **Rides the component upgrade** — for a component-coupled gate the component (e.g.
  the CN) and the gate move together, so a single pod restart applies both: no extra
  restart, no re-keying.
- **Idempotent + version-gated** — it runs only when the version crosses the point
  where the new behavior is needed, so it is safe to re-run and correct for fresh
  installs too.

solo-provisioner packages this as a migration (`internal/migration`): a step with
`Applies` (version check → idempotency), `Execute` (set the gate + verify), and
`Rollback`. An in-cluster job runs the same logic.

## Removing a gate

A gate has just two states in the operator: it exists (default off), or it's gone.
Once every cluster runs the new behavior, delete the gate and make it the only code
path. Any leftover value is ignored and cleaned up automatically.

## FAQ

**If an operator upgrade can change behavior, why put the gate in Orbit at all?**
So you can flip **without** an operator upgrade — the code ships in the operator
switched off, and setting the gate turns it on. That means one fewer deployment per
flip, plus a per-Orbit switch, a way to test it on one network first, in-place
rollback, and intent that's visible in Git.

**Who can flip the gate?**
Anyone with RBAC to patch the Orbit — the operator doesn't care who. Today that's
solo-provisioner's migration or the HIP jobs capability (`build/jobs/`). Manual
`kubectl` works but is discouraged (it skips the version check).

**Why a plain `bool` and not a nullable `*bool`?**
Because the operator has no auto-flipping default to protect: unset and `false` both
mean "old behavior", and the setter writes `true` to flip. A tri-state `*bool` only
helped an operator-owns-the-default model (which we dropped). Plain `bool` is
simpler and self-documents `default: false`.

**What if the gate and the deployed component disagree?**
The node breaks — the gate must match the component (e.g. gate `false` but a CN that
loads 0-based names can't find its keys). That coupling is inherent, which is why
whoever flips the gate sets it to match the component it deploys.

## Worked example: node-name convention (`nodeNameZeroBased`)

The first gate switches consensus gossip key-file naming from 1-based
(`node<id+1>`) to 0-based (`node<id>`), in lockstep with a consensus-node (CN)
version whose key loader reads the new names (a **component-coupled** gate). This PR
ships the gate, off by default.

- **Gate:** `orbit.spec.consensus.featureGates.nodeNameZeroBased`
  ([orbit_types.go](../../api/v1alpha1/orbit_types.go)).
- **Encoder:** `common.FormatNodeName(nodeId, zeroBased)`
  ([node.go](../../internal/common/node.go)) — the single source of truth;
  solo-provisioner mirrors it as `ConsensusNodeName(id, zeroBased)`.
- **Coordination:** key-management epic (#1164 / #1169); tracking issue #1336.

**What changes on flip** — both artifacts are operator-generated file *names*, not
key material, so a flip is naming-only (no re-keying, no data migration):

- Private-key mount path: `s-private-node<id+1>.pem` → `s-private-node<id>.pem`
  ([deployment.go](../../internal/resources/consensus/deployment.go)).
- Public-keys ConfigMap keys: `s-public-node<id+1>.pem` → `s-public-node<id>.pem`
  ([public_keys.go](../../internal/operations/genesis/public_keys.go)).

**Rollback** — within a compatible window, set the gate back to `false` and redeploy
the prior CN; pods recreate and naming reverts.
