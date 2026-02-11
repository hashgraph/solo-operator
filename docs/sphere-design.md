# Hedera Network Operator Design

## Overview

The Hedera Network Operator automates deployment and lifecycle management of Hedera consensus networks across multiple Kubernetes clusters. It uses a declarative API with three core concepts: **Orbit** (network-wide configuration), **Capsules** (component deployments), and **NetworkOperations** (blockchain transactions).

## Core Concepts

### Orbit
**Cluster-scoped** resource containing network-wide configuration synchronized across all clusters.

**Purpose**:
- Genesis configuration and network settings
- Addressbook management (node endpoints, certificates)
- Network-wide consensus parameters
- Messenger designation (one operator submits blockchain transactions)

**Key Characteristics**:
- Shared across all operators in all clusters
- Single source of truth for network state
- Contains addressbook synchronized from blockchain
- Designates one operator as "messenger"

### Capsule
**Namespaced** resource representing a deployable component.

**Types**:
- **ConsensusCapsule**: Hedera consensus node (StatefulSet)
- **EnvoyCapsule**: Envoy proxy (Deployment)
- **HAProxyCapsule**: HAProxy load balancer (Deployment)
- **HelmCapsule**: Helm-managed component (HelmRelease CR)

**Purpose**:
- Define component configuration
- Trigger operator to create/update Kubernetes resources
- Maintain desired state of component

### NetworkOperation
**Namespaced** resource representing a blockchain transaction.

**Creation Paths**:
1. **Operator-initiated**: Created automatically when Capsule changes require blockchain consensus
2. **User-initiated**: Created manually for ledger-only operations (accounts, tokens, contracts)

**Purpose**:
- Submit transactions to Hedera network via Go SDK
- Track transaction status and consensus
- Coordinate blockchain state with Kubernetes state

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Orbit (Cluster-scoped)                  │
│  • Genesis configuration                                        │
│  • Addressbook (synced from blockchain)                         │
│  • Network settings                                             │
│  • Messenger designation                                        │
└────────────────────────┬────────────────────────────────────────┘
                         │ references
                         │
        ┌────────────────┼────────────────┐
        │                │                │
        ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Consensus   │  │    Envoy     │  │     Helm     │
│   Capsule    │  │   Capsule    │  │   Capsule    │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       │ creates         │ creates         │ creates
       ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ StatefulSet  │  │ Deployment   │  │ HelmRelease  │
│   (Pods)     │  │   (Pods)     │  │   (Flux)     │
└──────────────┘  └──────────────┘  └──────────────┘
```

## Change Management Flow

### Operator-Initiated Changes

```
1. User updates Capsule CR
         ↓
2. Operator detects change (compares old vs new spec)
         ↓
3. Operator determines required actions:
   • Blockchain transaction needed?
   • Config update needed?
   • Pod restart needed?
         ↓
4. If messenger: Create NetworkOperation CR
         ↓
5. NetworkOperation Controller submits transaction
   (via Hedera Go SDK)
         ↓
6. Wait for blockchain consensus
         ↓
7. Operator applies Kubernetes changes:
   • Update ConfigMaps
   • Update StatefulSet/Deployment
   • Trigger pod restart if needed
         ↓
8. Update Capsule status
```

### User-Initiated Operations

```
1. User creates NetworkOperation CR
         ↓
2. NetworkOperation Controller (messenger only)
         ↓
3. Submit transaction to blockchain
   (via Hedera Go SDK)
         ↓
4. Wait for consensus
         ↓
5. Update NetworkOperation status
         ↓
6. DONE (no Kubernetes changes)
```

## Messenger Pattern

### Problem
Multiple operators across clusters must coordinate blockchain transactions to avoid conflicts.

### Solution
Each operator deployment is configured as messenger or non-messenger via deployment configuration (not in Orbit CR).

### Implementation

**Operator Deployment Configuration**:
```yaml
# Messenger operator (us-central1 cluster)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hedera-operator
  namespace: hedera-system
spec:
  template:
    spec:
      containers:
      - name: operator
        image: hedera-operator:latest
        env:
        - name: TX_MODE
          value: "messenger"  # This operator submits transactions
        - name: OPERATOR_ID
          value: "us-central1-operator"

---
# Non-messenger operator (us-east1 cluster)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hedera-operator
  namespace: hedera-system
spec:
  template:
    spec:
      containers:
      - name: operator
        image: hedera-operator:latest
        env:
        - name: TX_MODE
          value: "observer"  # This operator only watches
        - name: OPERATOR_ID
          value: "us-east1-operator"
```

**Orbit CR** (cluster-agnostic):
```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Orbit
metadata:
  name: my-network
spec:
  genesis:
    # ... genesis config
  addressbook:
    # ... node endpoints
  # No messenger designation - operators know their role from deployment config
```

**Behavior**:
- **Messenger operator**: Creates and submits NetworkOperations
- **Observer operators**: Watch NetworkOperation status, apply K8s changes after consensus
- **Leader election**: Within messenger cluster for HA (multiple replicas)

**Enforcement**:
- Operator checks `TX_MODE` environment variable
- Only messenger mode creates NetworkOperation CRs
- All operators watch NetworkOperation status
- Kubernetes RBAC can restrict NetworkOperation creation to messenger namespace

**Challenges**:
- Ensuring exactly one messenger deployment configured
- Handling messenger cluster failure
- Coordinating messenger role change

**Proposed Solution**:
- Manual configuration via Helm values or deployment manifests
- Monitoring alerts if no NetworkOperations created within threshold
- Documented runbook for messenger failover

## Consensus Node Awareness

### Freeze Detection

Operator monitors consensus node status to detect freeze state before upgrades.

**Upgrade Flow**:
```
1. NetworkOperation created for upgrade
         ↓
2. Messenger submits freeze transaction
         ↓
3. Operator polls nodes for freeze status
         ↓
4. All nodes frozen?
         ↓
5. Apply upgrade (update StatefulSet)
         ↓
6. Wait for pods to restart
         ↓
7. Verify network resumes consensus
```

## Delayed Reconciliation

### Use Case
Schedule changes for maintenance windows (e.g., 2 AM UTC).

### Implementation
```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: ConsensusCapsule
metadata:
  name: node-0
  annotations:
    operator.solo.hedera.com/reconcile-at: "2024-01-16T02:00:00Z"
spec:
  version: v0.61.0  # Upgrade scheduled for 2 AM
```

**Behavior**:
- Operator detects annotation
- Requeues reconciliation until specified time
- Proceeds with normal change flow at scheduled time

**Precision**: ~1-5 seconds (Kubernetes reconciliation timing)

## Change Orchestration

### Diff-Based Decision Making

Operator compares old and new Capsule specs to determine required actions.

**Example: Version Upgrade**
```
Old: spec.version = v0.60.1
New: spec.version = v0.61.0

Actions Required:
1. Blockchain transaction (NodeUpdate)
2. Update StatefulSet image
3. Rolling restart pods
```

**Example: Config Change**
```
Old: spec.config.hedera.realm = "0"
New: spec.config.hedera.realm = "1"

Actions Required:
1. Update ConfigMap
2. Restart pods (no blockchain transaction)
```

### Orchestration Steps

1. **Detect Change**: Watch Capsule CR for spec updates
2. **Generate Diff**: Compare old vs new spec
3. **Plan Actions**: Determine required operations
4. **Create NetworkOperation** (if blockchain transaction needed)
5. **Wait for Consensus**: Monitor NetworkOperation status
6. **Apply K8s Changes**: Update ConfigMaps, StatefulSets, Deployments
7. **Verify Health**: Check pod status, node consensus
8. **Update Status**: Reflect changes in Capsule status

## Addressbook Synchronization

### Challenge
Addressbook must stay synchronized across all clusters and match blockchain state.

### Flow
```
1. Consensus nodes running
         ↓
2. Operator downloads addressbook from network (gRPC)
         ↓
3. Operator updates Orbit with addressbook data
         ↓
4. Orbit synced to all clusters (external mechanism)
         ↓
5. All operators have consistent addressbook view
```

**Sync Mechanism** (external to operator):
- Git repository with Orbit CRs
- Flux CD or ArgoCD syncing across clusters
- Manual kubectl apply to all clusters

## Multi-Cluster Deployment

### Scenario
Network spans 3 Kubernetes clusters (us-central1, us-east1, eu-west1).

### Setup
```yaml
# Orbit (same in all clusters, no messenger designation)
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Orbit
metadata:
  name: prod-network
spec:
  genesis:
    # ... genesis config
  addressbook:
    # ... node endpoints

---
# us-central1 cluster (messenger operator)
apiVersion: operator.solo.hedera.com/v1alpha1
kind: ConsensusCapsule
metadata:
  name: node-0
  namespace: hedera
spec:
  orbit: prod-network
  nodeId: 0
  # ... config

---
# us-east1 cluster (observer operator)
apiVersion: operator.solo.hedera.com/v1alpha1
kind: ConsensusCapsule
metadata:
  name: node-1
  namespace: hedera
spec:
  orbit: prod-network
  nodeId: 1
  # ... config
```

**Coordination**:
- Orbit synced across clusters (external tool)
- us-central1 operator configured as messenger (deployment env var)
- us-east1 operator configured as observer (deployment env var)
- Messenger creates NetworkOperations
- All operators watch NetworkOperation status
- Each operator manages local Capsules

## Status Tracking

### Capsule Status
```yaml
status:
  phase: Pending | Running | Updating | Failed
  conditions:
    - type: Ready
      status: "True"
      reason: PodsRunning
  observedGeneration: 5
  appliedSpec:
    version: v0.60.1  # Currently running
  pendingSpec:
    version: v0.61.0  # Waiting for NetworkOperation
  networkOperation: node-0-upgrade  # Reference to pending operation
```

### NetworkOperation Status
```yaml
status:
  phase: Pending | Submitted | Consensus | Success | Failed
  transactionId: "0.0.3@1234567890.123456789"
  consensusTimestamp: "2024-01-15T10:30:45Z"
  error: ""  # If failed
```

## Benefits

- **Declarative**: All configuration in CRs
- **Coordinated**: Blockchain consensus before K8s changes
- **Multi-Cluster**: Single network across multiple clusters
- **Auditable**: NetworkOperation CRs provide transaction history
- **Flexible**: Supports both operator and user-initiated operations
