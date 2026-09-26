# Node-name convention migration (node&lt;id+1&gt; → node&lt;id&gt;)

Rollout plan for switching the consensus gossip key-file naming convention from
1-based (`node<id+1>`) to 0-based (`node<id>`), coordinated with the consensus
node (CN) key loader.

- **Gate:** `orbit.spec.consensus.featureGates.nodeNameZeroBased` (see
  [orbit_types.go](../../api/v1alpha1/orbit_types.go)).
- **Encoder:** `common.FormatNodeName(nodeId, zeroBased)` in
  [node.go](../../internal/common/node.go) — the single source of truth for the
  convention. Weaver mirrors it as `ConsensusNodeName(id, zeroBased)`.
- **Coordination:** weaver key-management epic (weaver #1169); tracking issue #1336.

## Why a feature gate

The CN's key loader determines the on-disk filenames it reads at boot. Changing
the naming convention is only safe in lockstep with a CN version that loads the
new names, so the switch is a transitional **feature gate** (graduates
Alpha→Beta→GA, then removed) rather than permanent configuration.

## What physically changes on flip

Both affected artifacts are **operator-generated at deploy time**, not persisted
re-keyable state:

- **Private-key mount path** (StatefulSet projected volume):
  `s-private-node<id+1>.pem` → `s-private-node<id>.pem`
  ([deployment.go](../../internal/resources/consensus/deployment.go)).
- **Public-keys ConfigMap keys**:
  `s-public-node<id+1>.pem` → `s-public-node<id>.pem`
  ([public_keys.go](../../internal/operations/genesis/public_keys.go)).

The source secrets keep arbitrary names; the operator normalizes them. A flip is
therefore **naming-only** — no re-keying and no PVC data migration. Applying it
needs only a pod recreate (which an infra-upgrade performs anyway) plus a
public-keys regeneration. This is what keeps the migration cheap to automate.

## Division of labor

- **Operator** — makes the flip automatic on redeploy: owns the default
  convention and guarantees the change is self-applying (pod template updated,
  public-keys ConfigMap regenerated).
- **Weaver** — orchestrates during infra-upgrade: clears stale overrides,
  triggers regeneration, and verifies the rollout. No Orbit edit in the happy path.

## Phases

### Alpha — mechanism + inert gate (issue #1336, weaver #1169)

- **Operator:** `ConsensusFeatureGates.NodeNameZeroBased` (bool, default false);
  `FormatNodeName`; both sites routed through it. Behavior byte-identical (1-based).
- **Weaver:** mirrors `ConsensusNodeName(id, zeroBased)`; may set the gate but
  leaves it false.
- **Automation:** none — forward-compat seam only.
- **Exit:** merged + released; CN 0-based loader lands upstream.

### Beta — operator owns the default; gate becomes an override

- **Operator:**
  - Gate → `*bool` override (nil = use operator default); drop the kubebuilder default.
  - Add a `defaultNodeNameZeroBased` compile-time constant (still false until the
    paired CN baseline loads 0-based names).
  - `IsNodeNameZeroBased()` = override when set, else the constant.
  - Guarantee the flip is self-applying: a pure gate/default change must alter the
    StatefulSet pod template so the RollingUpdate recreates the pod, and must
    regenerate the public-keys ConfigMap on reconcile — with no image change required.
  - Surface the resolved convention (Orbit status or a log line with a
    `NodeNameConventionResolved` reason) so weaver and humans can verify which
    convention is live.
- **Weaver:**
  - Stop writing the gate by default; reserve it for explicit exceptions/rollback.
  - Add an idempotent migration subcommand, invoked during infra-upgrade, that:
    1. clears stale explicit overrides that would pin the old convention,
    2. triggers NetworkGenesis/public-keys regeneration,
    3. waits for and verifies the rollout reports the new convention.
- **Payoff:** the operator release cut alongside the 0-based CN baseline sets
  `defaultNodeNameZeroBased = true`. Redeploying that operator during
  infra-upgrade flips naming fleet-wide with **zero Orbit edits**; the weaver
  command only cleans residue and verifies.
- **Exit:** 0-based verified in canary; override exercised for rollback; migration
  command proven idempotent.

### GA — remove the gate

- **Operator:** delete `NodeNameZeroBased` (and `ConsensusFeatureGates` if it is
  the only gate), hardcode `FormatNodeName` to 0-based, remove override plumbing,
  regenerate CRD + deepcopy.
- **Weaver:** drop gate-writing/override handling; `ConsensusNodeName` hardcodes
  0-based. The migration command stays — it is idempotent and only acts when
  required, so it no-ops once every cluster is already 0-based.
- **Exit:** no cluster runs a pre-0-based CN; deprecation window elapsed.

## Rollback

- Redeploy the prior operator (default false) + prior CN; pods recreate; naming
  reverts.
- The `*bool` override (Beta onward) is the manual escape hatch in either
  direction — e.g. pin a single cluster while the fleet migrates.
