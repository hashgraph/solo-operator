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
- **HelmCapsule**: Helm-managed component (creates Flux CD HelmRelease CR)

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
│   (Pods)     │  │   (Pods)     │  │ (Flux CD)    │
└──────────────┘  └──────────────┘  └──────┬───────┘
                                            │
                                            │ Flux CD
                                            │ reconciles
                                            ▼
                                    ┌──────────────┐
                                    │ Helm Chart   │
                                    │  Resources   │
                                    └──────────────┘
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
   • Update HelmRelease CR (Flux CD handles Helm deployment)
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
6. **Apply K8s Changes**: Update ConfigMaps, StatefulSets, Deployments, or HelmRelease CRs (Flux CD reconciles)
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

**Note**: Flux CD HelmRelease operator handles all Helm-based component deployments referenced by HelmCapsule CRs.

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

## Migration Strategy

### Phase 1: Manual NetworkOperations (Weeks 1-4)

**Goal**: Operator manages Kubernetes resources, humans manage blockchain transactions.

**Operator Behavior**:
- Watches Capsule CRs for changes
- Detects when blockchain transaction is needed
- **Does NOT create NetworkOperation CRs**
- Updates Capsule status to indicate "waiting for NetworkOperation"
- Waits for freeze state before applying K8s changes

**Human Workflow**:
```
1. User updates ConsensusCapsule (e.g., version upgrade)
         ↓
2. Operator detects change, updates status:
   status.pendingSpec.version = v0.61.0
   status.message = "Waiting for NetworkOperation with freeze timestamp"
         ↓
3. Human creates NetworkOperation CR manually:
   kubectl apply -f networkop-node-0-upgrade.yaml
         ↓
4. Messenger operator submits freeze transaction with timestamp
         ↓
5. Operator waits until freeze timestamp is reached
         ↓
6. Operator polls nodes for freeze status
         ↓
7. All nodes frozen?
         ↓
8. Operator applies K8s changes (update StatefulSet)
         ↓
9. Pods restart with new version
```

**Example Manual NetworkOperation**:
```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: NetworkOperation
metadata:
  name: node-0-upgrade
  namespace: hedera
spec:
  mode: NodeUpdate
  capsuleRef:
    name: node-0
    kind: ConsensusCapsule
  operation:
    nodeId: 0
    newVersion: v0.61.0
    freezeTimestamp: "2024-01-16T02:00:00Z"  # When nodes will freeze
  configurationDiff:
    field: spec.version
    oldValue: v0.60.1
    newValue: v0.61.0
```

**Benefits**:
- Operators gain confidence in freeze detection
- Humans control transaction timing
- Lower risk for initial deployments
- Clear separation of concerns

**Limitations**:
- Manual NetworkOperation creation required
- Humans must monitor freeze status
- Slower upgrade process

### Phase 2: Automated NetworkOperations (Weeks 5+)

**Goal**: Operator fully automates blockchain transactions and K8s changes.

**Operator Behavior**:
- Watches Capsule CRs for changes
- Detects when blockchain transaction is needed
- **Automatically creates NetworkOperation CRs**
- Submits freeze transaction (if messenger)
- Polls nodes for freeze status
- Applies K8s changes when frozen
- Updates Capsule status throughout process

**Automated Workflow**:
```
1. User updates ConsensusCapsule (e.g., version upgrade)
         ↓
2. Operator detects change, generates diff
         ↓
3. Operator creates NetworkOperation CR automatically
         ↓
4. Messenger operator submits freeze transaction
         ↓
5. Operator polls nodes for freeze status
         ↓
6. All nodes frozen?
         ↓
7. Operator applies K8s changes (update StatefulSet)
         ↓
8. Pods restart with new version
         ↓
9. Operator verifies network resumes consensus
```

**Configuration Flag**:
```yaml
# Operator deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hedera-operator
spec:
  template:
    spec:
      containers:
      - name: operator
        env:
        - name: TX_MODE
          value: "messenger"
        - name: AUTO_CREATE_NETOPS
          value: "true"  # Enable automated NetworkOperation creation
```

**Benefits**:
- Fully automated upgrades
- Faster change deployment
- Reduced human error
- Consistent process across clusters

**Risks**:
- Operator bugs could cause network issues
- Less human oversight
- Requires thorough testing

### Migration Checklist

**Before Phase 1**:
- [ ] Deploy operator in all clusters
- [ ] Configure messenger vs observer mode
- [ ] Set up Orbit synchronization (GitOps)
- [ ] Create initial Capsule CRs
- [ ] Document NetworkOperation creation process

**Phase 1 → Phase 2 Transition**:
- [ ] Test automated NetworkOperation creation in dev environment
- [ ] Verify freeze detection works reliably
- [ ] Monitor operator logs for errors
- [ ] Perform test upgrade with automation
- [ ] Set `AUTO_CREATE_NETOPS=true` in production
- [ ] Monitor first automated upgrade closely

**Rollback Plan**:
- Set `AUTO_CREATE_NETOPS=false` to return to manual mode
- Operator continues to manage K8s resources
- Humans resume manual NetworkOperation creation
