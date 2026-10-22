# DAB host operations (`solo-provisioner consensus node`)

The commands a **node operator runs on a host** to set up, join, reconfigure or remove a consensus
node, plus the key handling and the one-time prerequisites. These are **not event-driven**: they have
no CN event trigger, which is why they live here and not in the
[event-processing design](dab-event-handling.md). The upgrade-time key swap
and firewall changes stay with the UC, operator and daemon; **image changes are never made here** —
they go through the network upgrade.

> **Live document / holding area.** This started as content split out of the event-processing design
> so that doc could stay focused on "event in, custom resource out." Some sections are fuller than
> others; TODOs mark what still needs design. It will grow into the provisioner/solo-weaver
> implementation guide.

## Before you start (prerequisites)

1. **The `cn-gossip` firewall rule exists** (mainnet), created by `solo-provisioner` at provisioning.
   The daemon only creates it as a fallback.
2. **Key Secrets follow the provisioner naming** (solo-weaver PR 1198). The fingerprint in the name
   is the SHA-384 of the certificate DER. A new key creates a new Secret beside the old one and never
   overwrites it.
3. **`orbit.spec.consensus.canAdoptRosterFromVersion` is set** when the network should automate key
   swaps and firewall changes (see "When the operator acts" in the
   [event-processing design](dab-event-handling.md#when-the-operator-acts)).

> The CN-side roster export settings (where the CN writes the candidate roster, the marker files)
> move with the transport. See the [event-stream doc](dab-cn-uc-event-stream.md).

## The commands

These install, join, reconfigure and remove a CN on its host. They never submit DAB transactions.
All are idempotent and have a `--check` mode that prints the plan without changing anything.

| Command | Used for | What it does |
|---|---|---|
| `install` | A node of a new network (genesis) | Unchanged. |
| `join` | A new node, or a node moved to another host, joining an existing network | See [Joining a node](#joining-a-node-nodecreate). |
| `reconfigure` | A node already on this host | In-place, same-image changes. The DAB flow needs nothing from it today; it may grow later. |
| `uninstall` | A node that left the roster | Removes the CN from its host. See [uninstall](#uninstall). |

### `NodeUpdate` needs no new command

The operator swaps the key inside the upgrade patch, so the node operator has nothing to run on the
host for a normal `NodeUpdate`:

- Before the freeze, check the staged key matches the certificate sent in `NodeUpdate` with
  `consensus keys verify`.
- Where automation is not enabled for the version, repoint the signing Secret by hand after the CN
  adopts the roster.
- A change of gossip IP needs nothing on the node's own host — the peers' hosts follow through the
  firewall plan.

**Capsule fields that follow a `NodeUpdate`.** The candidate roster carries only node ID, weight,
gossip certificate and gossip endpoints. Other changes are patched on the capsule:

| `NodeUpdate` field | Capsule field | What follows |
|---|---|---|
| `gossip_ca_certificate` | `secrets.secretResources.signingPrivateKey` / `signingPublicCertificate` | Repointed in the upgrade patch (operator). |
| `account_id` | `spec.accountId` | **Must follow, in the same patch as the images** — pod label, Helm network map and Cheetah folders all use it. Staged as `pendingAccountId`. |
| `gossip_endpoint` | `externalAddress`, `gossipPort` | Address feeds status/generated values; a new gossip port also changes the capsule's Service. |
| `service_endpoint` | `grpcPort`, Service, proxies | Out of scope (HIP-1495). |
| `grpc_certificate_hash` | `secrets.secretResources.grpcTls*` | Same mechanism as the signing key, out of scope. |
| `description`, `admin_key`, `decline_reward`, `grpc_proxy_endpoint` | none | Nothing in the capsule reads them. |

`nodeId`, `nodeAlias` and `weight` never follow a `NodeUpdate`.

## Joining a node (`NodeCreate`)

Existing nodes run the normal upgrade (handled in the event-processing design). The **new** node is
set up here, on its own host, and starts from a recent state of the network.

**Genesis and joining are different.** Genesis creates a network, once. A node joining an existing
network is not part of it. A network can start with one genesis node and grow only through
`NodeCreate`. Nodes run in separate clusters around the world, so the operator cannot discover them
from Kubernetes Services; a peer supplies what the new node needs. Where all nodes share one cluster,
the operator can build the roster from the capsules (see cluster-only mode in the event-processing
design).

### Flow

| # | Who | What |
|---|---|---|
| 1 | New node operator | Generates the node's keys with `consensus keys`. Private keys stay on the operator's side. |
| 2 | New node operator | Gives the council, offline: gossip certificate, admin public key, node account, gossip and service endpoints, gRPC certificate hash. The account must already exist. |
| 3 | Council | Submits `NodeCreate`, signed with the node's admin key. The receipt carries the **node ID**. Must land before `PREPARE_UPGRADE`. |
| 4 | New node operator | Imports the keys with that node ID. |
| 5 | Council | Sends `PREPARE_UPGRADE` and `FREEZE_UPGRADE`. Existing nodes upgrade and adopt the new roster. The new node is not running; `NodeCreate` sets weight 0, so consensus is unaffected. The upgrade must raise the software version, or the CN will not adopt the roster. |
| 6 | New node operator | Collects from a peer: the roster and a recent saved state. Gets node details from the mirror node. |
| 7 | New node operator | Runs `consensus node join` with those inputs. |
| 8 | New node | Starts, connects to its peers and catches up. |

### The roster (step 6)

The new node is outside the network and cannot produce a candidate roster, and the package predates
it. A peer supplies:

- **The roster entries** (gossip certificate, gossip endpoints, weight): the candidate roster an
  existing node exported, or its active roster after the upgrade. The mirror node's REST API does not
  publish the gossip certificate or gossip endpoints — its `node_cert_hash` is the TLS certificate
  hash and its `service_endpoints` are service endpoints. Compare the roster across **more than one**
  peer, because the mirror node cannot prove gossip certificates.
- **The node details** (account ID, admin key, service endpoints, stake) come from the mirror node.

### The state (step 6)

A peer supplies a recent saved state, so the new node only catches up on the gap instead of receiving
the whole state by reconnect.

- Any peer's state will do — all peers hold the same state and hash.
- It must be from **after** the upgrade restart: its active roster must contain the new node. A node
  not in the roster it starts from cannot load its signing certificate and exits.
- A node saves a state every 900 s by default (`state.saveStatePeriod`), so a recent one exists.
- The CN looks for saved states at
  `data/saved/com.hedera.services.ServicesMain/<nodeId>/123/<round>` under its home directory. Unpack
  the downloaded state there, under the **new** node's ID, **before** the CN starts; the CN loads the
  latest it finds and verifies it.
- With a state, the CN reads its roster from the state and needs no `genesis-network.json`. Without
  one it takes the genesis path, with the roster supplied as that file — slower, but works.

### `consensus node join` (step 7)

1. Check the roster holds this node ID with a matching certificate fingerprint, and that node IDs,
   account IDs and stake agree with the mirror node.
2. Unpack the state into the capsule's `saved` volume at that path, after checking its checksum. The
   capsule's genesis-init container sees that directory and does not wait for a genesis file.
3. Set up the `cn-gossip` rule from the roster: every peer's gossip address, and **not** its own.
4. Check each peer's gossip port is reachable. A refusal usually means that peer has not opened this
   node's address yet.
5. Create the capsule at the post-upgrade version (with the roster as its genesis file only when no
   state was given), start it, and report status until `ACTIVE`.

**The new node.** Its first applied-roster baseline is seeded from the supplied roster, so it matches
the network. Its UC starts with no backlog and creates nothing until the first upgrade. No Execute CR
exists for it, so **starting it is the node operator's step** — a join CR would be a rare-event cost
for little gain.

## Moving a node to another host

A move is a `NodeUpdate` on the roster side, with the node's process starting elsewhere:

1. On the new host, generate new keys, then submit a `NodeUpdate` with the new certificate and new
   gossip and service endpoints. Must land before `PREPARE_UPGRADE`. The node ID and account stay.
2. **Peers** follow the normal firewall plan: the new address opens first, the old one closes last.
3. **The new host** starts like a new node — a state and roster from a peer — using `consensus node
   join` with the **existing** node ID. There is no `NodeCreate`.
4. **The old host** is removed with `consensus node uninstall`, once the freeze has begun. Running it
   earlier would take a voting node offline for days.

There is no special signal for a move. Until the old node is removed, the operator treats it as any
node whose key does not match its roster entry: it raises `RosterKeyMismatch`, and the old pod
restarts on the old key and the CN fails to load it. That does no harm — the roster entry now belongs
to the new host.

> **TODO — a cleaner move signal.** A `consensus node update --move` mark could let Execute take the
> removed-node offline path and end `NodeMoved=True`, instead of relying on the mismatch warning.
> Decide whether this is worth a dedicated signal.

## `uninstall`

Removes the node from this cluster. It does **not** wait for the node to leave the roster; the node
operator decides when to run it.

1. Make sure the capsule is offline, then remove it.
2. Keep the capsule's volumes and key Secrets by default. `--purge-data` removes the volumes; key
   Secrets stay until a `keys delete` command exists.
3. Update `state.yaml` and remove this node from the daemon's configuration.

It does not touch the firewall, the operator or anything else on the host.

## Open items

- `keys delete`, if old key Secrets should ever be cleaned up. Not needed yet.
- The `--move` signal above.
- `reconfigure` scope as the DAB flow grows.

## References

- [dab-event-handling.md](dab-event-handling.md) — the
  event-processing design these commands support.
- [dab-cn-uc-event-stream.md](dab-cn-uc-event-stream.md) — the CN→UC transport.
- `hashgraph/solo-weaver` PR 1198 (key import/export and Secret naming), issue 1305 (`cn-gossip`
  rule).
