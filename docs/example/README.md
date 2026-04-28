# Deploy a 4-node test network

## Prerequisites

### Docker Desktop Resources

Increase Docker Desktop memory allocation:
- Set Memory to at least 8GB (12GB recommended)
- Set CPUs to at least 4

### Setup Cluster and Operator

From the project root:

```shell
# Reset kind cluster with fresh CRDs
task kind:reset

# Start solo operator (in another terminal - MUST be running)
task run
```

## Workflow

From the `docs/example` directory:

```shell
# Install MinIO (object storage for stream uploaders)
task setup

# Deploy full 4-node network (orbit + all nodes + genesis)
task deploy

# Check status
task status

# Wait for all pods to be ready
task wait

# Tail consensus node logs
task logs
```

### Deploy individual nodes

```shell
# Deploy a single node (deploys orbit + common first)
task deploy:node0
task deploy:node1
task deploy:node2
task deploy:node3
```

## Proxy Sidecars (optional)

Each node has its own `HAProxyCapsule` and `EnvoyProxy` deployed alongside its `ConsensusCapsule` via `task deploy:nodeX`. To deploy proxies manually:

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
# Destroy a specific node
task destroy:node1

# Destroy all nodes, configs, and orbit
task destroy

# Uninstall MinIO
task cleanup
```
