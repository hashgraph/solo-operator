# Migration Guide: Java to Go Operator

This guide helps users migrate from the Java-based Solo Operator to the Go-based implementation.

## Overview

The Go-based Solo Operator is a complete rewrite that maintains API compatibility while providing:

- **Faster startup**: ~2 seconds vs ~30+ seconds for JVM
- **Lower memory footprint**: ~15MB vs ~200MB+ for JVM
- **Identical CRD schemas**: No changes to your existing manifests
- **Same reconciliation behavior**: Operations work the same way

## Pre-Migration Checklist

Before migrating, ensure:

- [ ] All NetworkOperations are in `Succeeded` or `Failed` state
- [ ] No active upgrades or node operations in progress
- [ ] Backup of all Orbit and ConsensusCapsule manifests
- [ ] Access to the Kubernetes cluster with admin privileges

## Migration Steps

### Step 1: Scale Down Java Operator

```bash
# Scale down the Java operator deployment
kubectl scale deployment solo-operator -n solo-operator-system --replicas=0

# Verify it's stopped
kubectl get pods -n solo-operator-system
```

### Step 2: Install Go Operator CRDs

The CRD schemas are compatible, but you should update them:

```bash
# Apply the new CRDs
kubectl apply --server-side --force-conflicts -f config/crd/bases/

# Verify CRDs are updated
kubectl get crds | grep solo.hedera.com
```

### Step 3: Deploy Go Operator

```bash
# Deploy the Go operator
kubectl apply -f config/manager/

# Or using Helm (if available)
helm upgrade --install solo-operator ./charts/solo-operator \
  --namespace solo-operator-system \
  --create-namespace
```

### Step 4: Verify Operation

```bash
# Check operator logs
kubectl logs -n solo-operator-system deployment/solo-operator-controller-manager -f

# Verify all resources are being reconciled
kubectl get orbits
kubectl get consensuscapsules -A
kubectl get networkoperations -A
```

## CRD Compatibility

### Unchanged Fields

All existing fields in CRDs remain unchanged:

| CRD | Compatibility |
|-----|---------------|
| Orbit | 100% compatible |
| ConsensusCapsule | 100% compatible |
| NetworkOperation | 100% compatible |
| HAProxyCapsule | 100% compatible |
| EnvoyProxy | 100% compatible |

### Behavior Changes

While APIs are compatible, some behaviors have minor improvements:

1. **Sequencer**: More efficient queue management with identical ordering semantics
2. **Status Updates**: More frequent and accurate status condition updates
3. **Error Handling**: More detailed error messages in conditions

## Rollback Procedure

If you need to rollback to the Java operator:

```bash
# Scale down Go operator
kubectl scale deployment solo-operator-controller-manager \
  -n solo-operator-system --replicas=0

# Restore Java operator CRDs (if different)
kubectl apply -f /path/to/java-operator/crds/

# Scale up Java operator
kubectl scale deployment solo-operator \
  -n solo-operator-system --replicas=1
```

## Configuration Differences

### Environment Variables

The Go operator uses standard controller-runtime configuration:

| Java Operator | Go Operator | Description |
|---------------|-------------|-------------|
| `QUARKUS_LOG_LEVEL` | `--zap-log-level` | Log verbosity |
| `JAVA_OPTS` | N/A | Not needed |
| N/A | `--leader-elect` | Enable leader election |
| N/A | `--metrics-bind-address` | Metrics endpoint |
| N/A | `--health-probe-bind-address` | Health probes |

### Resource Requirements

Recommended resources for the Go operator:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 128Mi
```

Compared to Java operator requirements:

```yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi
```

## Monitoring

### Metrics

The Go operator exposes Prometheus metrics at `/metrics`:

```
# Controller reconciliation metrics
controller_runtime_reconcile_total
controller_runtime_reconcile_errors_total
controller_runtime_reconcile_time_seconds

# Workqueue metrics
workqueue_adds_total
workqueue_depth
workqueue_queue_duration_seconds
```

### Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/healthz` | Liveness probe |
| `/readyz` | Readiness probe |
| `/metrics` | Prometheus metrics |

## Troubleshooting

### Common Issues

#### 1. CRDs Not Updating

If CRDs fail to update due to size:

```bash
# Use server-side apply
kubectl apply --server-side --force-conflicts -f config/crd/bases/
```

#### 2. Resources Not Reconciling

Check operator logs for errors:

```bash
kubectl logs -n solo-operator-system \
  deployment/solo-operator-controller-manager \
  --since=5m | grep -i error
```

#### 3. Sequencer Issues

If NetworkOperations are stuck:

```bash
# Check sequencer state
kubectl get networkoperations -A -o wide

# Verify no operations are in "Running" state without progress
kubectl describe networkoperation <name> -n <namespace>
```

### Debug Mode

Enable debug logging:

```bash
# Edit the deployment
kubectl edit deployment solo-operator-controller-manager \
  -n solo-operator-system

# Add to container args:
# - --zap-log-level=debug
```

## Feature Parity

| Feature | Java | Go | Notes |
|---------|------|-----|-------|
| Orbit management | ✅ | ✅ | Full parity |
| ConsensusCapsule deployment | ✅ | ✅ | Full parity |
| NetworkOperation sequencing | ✅ | ✅ | Full parity |
| Genesis workflow | ✅ | ✅ | Full parity |
| Upgrade workflow | ✅ | ✅ | Full parity |
| Node add/delete/update | ✅ | ✅ | Full parity |
| HAProxy management | ✅ | ✅ | Full parity |
| Envoy management | ✅ | ✅ | Full parity |
| Metrics | ✅ | ✅ | Different format |
| Leader election | ✅ | ✅ | Built-in |

## Support

For issues specific to the Go operator:

1. Check the [GitHub Issues](https://github.com/hashgraph/solo-operator/issues)
2. Review operator logs for detailed error messages
3. Ensure CRDs are properly installed with `kubectl get crds`

## Version Compatibility

| Go Operator Version | Kubernetes | Hedera Node |
|---------------------|------------|-------------|
| v1.0.0 | 1.27+ | v0.50.0+ |

## Changelog

### v1.0.0 (Initial Go Release)

- Complete rewrite in Go using controller-runtime
- Identical CRD schemas for seamless migration
- Improved performance and resource efficiency
- Enhanced error handling and status reporting
