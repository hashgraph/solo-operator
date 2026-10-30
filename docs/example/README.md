# Deploy a 4-node test network

## Prerequisites

### Docker Desktop Resources

Increase Docker Desktop memory allocation:
- Set Memory to at least 8GB (12GB recommended)
- Set CPUs to at least 4

### Setup Cluster and Operator

From the `docs/example` directory:

```shell
# Create the kind cluster and install the operator via Helm (skipped when already running)
task cluster
```

Or run the operator locally from the project root: `task kind:reset`, then `task run` in another terminal.

## Workflow

From the `docs/example` directory:

```shell
# Install MetalLB and RustFS (object storage for stream uploaders)
task setup

# Deploy full 4-node network (orbit + all nodes + genesis), waits for pods ready
task deploy

# Or deploy node0 only + genesis (lightweight, e.g. for beacon testing)
task deploy:single

# Check status
task status

# Tail consensus node logs
task logs
```

## Block Node (optional)

```shell
# Set up kind + operator + RustFS, deploy the block node first, then the network with node0 streaming to it,
# then a mirror monitor (pinger) that sends transfers to node0 so blocks flow
task deploy:blocknode

# Remove the pinger, the block node, its PVCs, and reset node0's block-nodes config
task destroy:blocknode
```

The block node is deployed before the consensus nodes so node0 starts with `block-nodes.json` and the stream properties already in place. TSS needs a threshold of nodes streaming blocks (`BOTH`), so all four nodes get the block-node properties. Only node0 is connected to the block node (`FILE_AND_GRPC`); node1 to node3 write block files only (`FILE`). Config changes on a running consensus node need a pod restart, because the files are `subPath` mounts. `deploy:blocknode` runs `task deploy BLOCKNODE=true`, which also applies `helmcapsule/block-nodes-config.yaml` and `helmcapsule/applicationproperties.yaml` for every node.

## Proxy Sidecars (optional)

Each node has its own `HAProxyCapsule` and `EnvoyProxy`. To deploy proxies manually:

```shell
# HAProxy — gRPC load balancer (ports 50211/50212), routes directly to consensus node
kubectl apply -f node0/haproxycapsule.yaml
kubectl apply -f node1/haproxycapsule.yaml
kubectl apply -f node2/haproxycapsule.yaml
kubectl apply -f node3/haproxycapsule.yaml

# EnvoyProxy — gRPC-Web gateway for browser/JS clients (port 8080), routes directly to consensus node
kubectl apply -f node0/envoyproxycapsule.yaml
kubectl apply -f node1/envoyproxycapsule.yaml
kubectl apply -f node2/envoyproxycapsule.yaml
kubectl apply -f node3/envoyproxycapsule.yaml
```

Both proxies auto-derive their upstream from the CR name (e.g. `haproxy-node1` → upstream `node1`, `envoy-node1` → upstream `node1`). Override with `upstreamService` if needed.

## Cleanup

```shell
# Delete the kind cluster
task destroy

# Uninstall MetalLB and RustFS
task cleanup
```

## Data retention

- **PVCs outlive the capsule.** The operator does not set an owner reference on a `ConsensusCapsule`'s PVCs, so deleting the capsule (or its Orbit) never deletes consensus data.
- **Reinstall reuses them.** A capsule with the same namespace and node id picks up the existing PVCs.
- **Delete data on purpose.** Remove it with an explicit `kubectl delete pvc -n <namespace> -l <labels>`.
