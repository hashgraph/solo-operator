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

# Check status
task status

# Tail consensus node logs
task logs
```

## Block Node (optional)

```shell
# Set up kind + operator + RustFS, deploy the network, then the block node (prints status.outputs)
task deploy:blocknode

# Remove the block node, its PVCs, and reset node0 block-nodes config
task destroy:blocknode
```

node0 only streams to the block node once its ApplicationProperties set the blockStream properties listed in `helmcapsule/block-nodes-config.yaml`.

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
# Destroy all nodes, configs, and orbit
task destroy

# Uninstall MetalLB and RustFS
task cleanup
```
