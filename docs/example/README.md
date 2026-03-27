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

## Cleanup

```shell
# Destroy a specific node
task destroy:node1

# Destroy all nodes, configs, and orbit
task destroy

# Uninstall MinIO
task cleanup
```
