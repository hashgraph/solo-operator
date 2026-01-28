# Solo Operator API Reference

This document provides a comprehensive reference for all Custom Resource Definitions (CRDs) managed by the Solo Operator.

## Table of Contents

- [Orbit](#orbit)
- [ConsensusCapsule](#consensuscapsule)
- [NetworkOperation](#networkoperation)
- [HAProxyCapsule](#haproxycapsule)
- [EnvoyProxy](#envoyproxy)

---

## Orbit

Orbit represents a Hedera network deployment configuration. It is a cluster-scoped resource that defines shared consensus configuration for all nodes in the network.

**API Version:** `operator.solo.hedera.com/v1alpha1`
**Kind:** `Orbit`
**Scope:** Cluster

### Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `deploymentModel` | string | No | Deployment model: `SingleKubernetesCluster` or `MultipleKubernetesClusters` |
| `lifetime` | string | No | Duration for the Orbit (e.g., `24h`, `7d`) |
| `finalizers` | []string | No | Finalizers to attach to all resources |
| `telemetry` | [OrbitTelemetry](#orbittelemetry) | No | Telemetry configuration |
| `consensus` | [OrbitConsensus](#orbitconsensus) | **Yes** | Shared consensus configuration |

### OrbitTelemetry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `metrics` | [OrbitMetrics](#orbitmetrics) | No | Metrics configuration |
| `logging` | [OrbitLogging](#orbitlogging) | No | Logging configuration |

### OrbitMetrics

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable metrics collection |
| `interval` | string | `5s` | Collection interval |
| `port` | int | `9998` | Metrics port |
| `labels` | map[string]string | - | Labels to attach to metrics |
| `remoteWrite` | bool | `false` | Enable remote write |

### OrbitLogging

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable logging |
| `labels` | map[string]string | - | Labels to attach to logs |

### OrbitConsensus

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provisioningModel` | string | No | How nodes are provisioned: `InitContainerDinD`, `InitContainerJava`, `DeterministicContainerImage` |
| `networkSettings` | map[string]string | **Yes** | Network settings (settings.txt) |
| `applicationConfiguration` | map[string]string | **Yes** | Application configuration (application.properties) |
| `permissionConfiguration` | map[string]string | **Yes** | Permission configuration (api-permission.properties) |
| `genesisProperties` | map[string]string | No | Genesis properties (genesis.properties) |
| `genesis` | [OrbitGenesis](#orbitgenesis) | No | Genesis-specific configuration |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase: `Pending`, `Running`, `Succeeded`, `Failed` |
| `conditions` | []Condition | Latest observations |
| `startTime` | string | When the Orbit was started (RFC3339) |
| `expirationTime` | string | When the Orbit will expire (RFC3339) |
| `observedGeneration` | int64 | Generation observed by operator |

### Example

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Orbit
metadata:
  name: my-network
spec:
  lifetime: "24h"
  telemetry:
    metrics:
      enabled: true
      interval: "10s"
  consensus:
    networkSettings:
      ledger.id: "0x03"
      hedera.realm: "0"
      hedera.shard: "0"
    applicationConfiguration:
      hedera.realm: "0"
    permissionConfiguration:
      default: "ALL"
```

---

## ConsensusCapsule

ConsensusCapsule represents a single Hedera consensus node deployment.

**API Version:** `operator.solo.hedera.com/v1alpha1`
**Kind:** `ConsensusCapsule`
**Scope:** Namespaced

### Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `orbit` | string | **Yes** | Name of the Orbit this node belongs to |
| `nodeId` | int64 | **Yes** | Unique node identifier |
| `accountId` | string | **Yes** | Hedera account ID (e.g., `0.0.3`) |
| `weight` | int | **Yes** | Node weight for consensus |
| `log4j2XML` | string | **Yes** | Log4j2 configuration XML |
| `podProperties` | [ConsensusPodProperties](#consensuspodproperties) | **Yes** | Pod configuration |
| `persistentVolumeClaims` | [ConsensusPVCConfig](#consensuspvcconfig) | No | PVC configuration |

### ConsensusPodProperties

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `containers` | [ConsensusContainers](#consensuscontainers) | **Yes** | Container configuration |
| `nodeSelector` | map[string]string | No | Node selector |
| `tolerations` | []Toleration | No | Pod tolerations |
| `affinity` | Affinity | No | Pod affinity rules |

### ConsensusContainers

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `consensusNode` | [ConsensusNodeContainer](#consensusnodecontainer) | **Yes** | Main consensus node container |
| `sidecars` | []SidecarContainer | No | Sidecar containers |

### ConsensusNodeContainer

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **Yes** | Container name |
| `softwareVersion` | [SoftwareVersion](#softwareversion) | **Yes** | Image configuration |
| `resources` | ResourceRequirements | No | CPU/memory resources |
| `env` | []EnvVar | No | Environment variables |

### SoftwareVersion

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repository` | string | **Yes** | Image repository |
| `imageName` | string | **Yes** | Image name |
| `imageTag` | string | **Yes** | Image tag |

### ConsensusPVCConfig

| Field | Type | Description |
|-------|------|-------------|
| `logs` | PersistentVolumeClaimSpec | PVC for logs |
| `stats` | PersistentVolumeClaimSpec | PVC for stats |
| `states` | PersistentVolumeClaimSpec | PVC for state data |
| `upgrade` | PersistentVolumeClaimSpec | PVC for upgrades |
| `recordStream` | PersistentVolumeClaimSpec | PVC for record streams |
| `eventsStream` | PersistentVolumeClaimSpec | PVC for event streams |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase |
| `conditions` | []Condition | Latest observations |
| `podPhase` | string | Pod phase from Kubernetes |
| `observedGeneration` | int64 | Generation observed by operator |

### Example

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: ConsensusCapsule
metadata:
  name: node0
  namespace: hedera-network
spec:
  orbit: my-network
  nodeId: 0
  accountId: "0.0.3"
  weight: 1
  log4j2XML: |
    <Configuration>
      <Appenders>
        <Console name="Console" target="SYSTEM_OUT">
          <PatternLayout pattern="%d{HH:mm:ss.SSS} [%t] %-5level %logger{36} - %msg%n"/>
        </Console>
      </Appenders>
      <Loggers>
        <Root level="info">
          <AppenderRef ref="Console"/>
        </Root>
      </Loggers>
    </Configuration>
  podProperties:
    containers:
      consensusNode:
        name: consensus-node
        softwareVersion:
          repository: hashgraph
          imageName: hedera-node
          imageTag: v0.50.0
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
```

---

## NetworkOperation

NetworkOperation represents a network-level operation such as genesis, upgrade, or node management.

**API Version:** `operator.solo.hedera.com/v1alpha1`
**Kind:** `NetworkOperation`
**Scope:** Namespaced
**Short Name:** `netop`

### Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `orbit` | string | **Yes** | Target Orbit name |
| `mode` | string | **Yes** | Operation mode (see below) |
| `networkUpgrade` | [NetworkUpgradeSpec](#networkupgradespec) | No | Upgrade configuration |
| `proxyUpgrade` | [ProxyUpgradeSpec](#proxyupgradespec) | No | Proxy upgrade configuration |
| `nodeOperation` | [NodeOperationSpec](#nodeoperationspec) | No | Node operation configuration |

### Operation Modes

| Mode | Description |
|------|-------------|
| `NetworkGenesis` | Initialize a new network |
| `NetworkCheck` | Verify network state |
| `NetworkUpgrade` | Upgrade the network |
| `NodeAdd` | Add a new node |
| `NodeDelete` | Remove a node |
| `NodeUpdate` | Update a node |
| `ProxyUpgrade` | Upgrade proxy components |

### NetworkUpgradeSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `workflowStep` | string | **Yes** | Current step: `ExecuteImmediate`, `FreezeScheduled`, `NowFrozen`, `FreezeAbort` |
| `changesets` | [NetworkUpgradeChangesets](#networkupgradechangesets) | **Yes** | Changes to apply |
| `configurationFiles` | [NetworkUpgradeConfigFiles](#networkupgradeconfigfiles) | No | Configuration file changes |
| `addressBook` | []AddressBookEntry | No | Updated address book |

### NodeOperationSpec

| Field | Type | Description |
|-------|------|-------------|
| `nodeAdd` | [NodeAddSpec](#nodeaddspec) | Configuration for adding a node |
| `nodeUpdate` | [NodeUpdateSpec](#nodeupdatespec) | Configuration for updating a node |
| `nodeDelete` | [NodeDeleteSpec](#nodedeletespec) | Configuration for deleting a node |

### NodeAddSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `nodeName` | string | **Yes** | Name of the node |
| `nodeId` | int64 | **Yes** | Unique node ID |
| `accountId` | string | **Yes** | Hedera account ID |
| `endpoints` | []NodeEndpoint | No | Network endpoints |

### NodeDeleteSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `nodeName` | string | **Yes** | Name of the node |
| `nodeId` | int64 | **Yes** | Node ID to delete |

### NodeUpdateSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `nodeName` | string | **Yes** | Name of the node |
| `accountId` | string | No | New account ID |
| `endpoints` | []NodeEndpoint | No | New endpoints |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase |
| `conditions` | []Condition | Latest observations |
| `observedGeneration` | int64 | Generation observed |
| `consensusCapsules` | []string | Capsules to initialize |
| `sequenceNumber` | int | Ordering for reconciliation |
| `upgradedNodes` | []int64 | Node IDs that have been upgraded |

### Example - Genesis

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: NetworkOperation
metadata:
  name: genesis
  namespace: hedera-network
spec:
  orbit: my-network
  mode: NetworkGenesis
```

### Example - Upgrade

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: NetworkOperation
metadata:
  name: upgrade-v51
  namespace: hedera-network
spec:
  orbit: my-network
  mode: NetworkUpgrade
  networkUpgrade:
    workflowStep: ExecuteImmediate
    changesets:
      consensusNodeChangeset:
        softwareVersionSource:
          imageRepositories:
            - repository: hashgraph
              imageName: hedera-node
              imageTag: v0.51.0
          imageVerificationSpec: []
```

### Example - Add Node

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: NetworkOperation
metadata:
  name: add-node5
  namespace: hedera-network
spec:
  orbit: my-network
  mode: NodeAdd
  nodeOperation:
    nodeAdd:
      nodeName: node5
      nodeId: 5
      accountId: "0.0.8"
      endpoints:
        - host: node5.hedera-network.svc.cluster.local
          port: 50111
          type: gossip
```

---

## HAProxyCapsule

HAProxyCapsule represents an HAProxy deployment for load balancing.

**API Version:** `operator.solo.hedera.com/v1alpha1`
**Kind:** `HAProxyCapsule`
**Scope:** Namespaced

### Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | **Yes** | HAProxy image |
| `serviceType` | string | **Yes** | Service type: `ClusterIP`, `NodePort`, `LoadBalancer` |
| `secretName` | string | No | TLS secret name |
| `files` | map[string]string | No | Custom configuration files |
| `replicas` | *int32 | No | Number of replicas |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase |
| `conditions` | []Condition | Latest observations |
| `observedGeneration` | int64 | Generation observed |

### Example

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: HAProxyCapsule
metadata:
  name: haproxy
  namespace: hedera-network
spec:
  image: haproxy:2.8.10-alpine3.20
  serviceType: LoadBalancer
  secretName: tls-secret
```

---

## EnvoyProxy

EnvoyProxy represents an Envoy proxy deployment.

**API Version:** `operator.solo.hedera.com/v1alpha1`
**Kind:** `EnvoyProxy`
**Scope:** Namespaced

### Spec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | `envoyproxy/envoy:v1.21.6` | Envoy image |
| `replicas` | *int32 | `1` | Number of replicas |
| `proxyPort` | int32 | `8080` | Proxy listening port |
| `adminPort` | int32 | `9901` | Admin interface port |
| `files` | map[string]string | - | Custom configuration files |
| `env` | []EnvVar | - | Environment variables |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase |
| `conditions` | []Condition | Latest observations |
| `observedGeneration` | int64 | Generation observed |

### Example

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: EnvoyProxy
metadata:
  name: envoy
  namespace: hedera-network
spec:
  image: envoyproxy/envoy:v1.21.6
  replicas: 2
  proxyPort: 8080
  adminPort: 9901
```

---

## Common Types

### Condition

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Condition type |
| `status` | string | Status: `True`, `False`, `Unknown` |
| `lastTransitionTime` | Time | Last transition time |
| `reason` | string | Machine-readable reason |
| `message` | string | Human-readable message |

### Phase

| Value | Description |
|-------|-------------|
| `Pending` | Resource is being processed |
| `Running` | Resource is active |
| `Succeeded` | Operation completed successfully |
| `Failed` | Operation failed |

---

## Labels

The operator uses the following standard labels:

| Label | Description |
|-------|-------------|
| `app.kubernetes.io/managed-by: solo-operator` | Resource managed by operator |
| `operator.solo.hedera.com/orbit` | Associated Orbit name |
| `operator.solo.hedera.com/node-id` | Node identifier |
