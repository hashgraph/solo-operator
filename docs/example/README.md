# Deploy a test network

## Prerequisites

### Docker Desktop Resources

Increase Docker Desktop memory allocation:
2. Set Memory to at least 8GB (12GB recommended)
3. Set CPUs to at least 4

### Setup Cluster and Operator

From the project root:

```shell
# Reset kind cluster with fresh CRDs
task reset

# Start solo operator (in another terminal - MUST be running)
task run
```

## Workflow

From the `docs/example` directory:

```shell
task setup

# Deploy node1 (runs genesis first, then creates pod)
task deploy:node1

# Deploy additional nodes (optional)
task deploy:node2
task deploy:node3
task deploy:node4

# Or deploy all nodes at once
task deploy
```

## Cleanup

```shell
# Destroy specific node
task destroy:node1

# Or destroy all nodes
task destroy

# Cleanup MinIO
task cleanup
```
