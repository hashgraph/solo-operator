# Sphere Operator Design

## Overview

Sphere Operator automates deployment and lifecycle management of Hedera consensus networks on Kubernetes. It provides a declarative API for deploying complete networks including consensus nodes, mirror node, explorer, and relay components.

## Architecture

### Custom Resource Definitions

#### Sphere
Top-level resource representing a complete Hedera network deployment.

**API Version**: `operator.solo.hedera.com/v1alpha1`  
**Kind**: `Sphere`  
**Scope**: Namespaced

#### ConsensusCapsule
Represents a single Hedera consensus node.

**API Version**: `operator.solo.hedera.com/v1alpha1`  
**Kind**: `ConsensusCapsule`  
**Scope**: Namespaced

#### EnvoyCapsule
Represents an Envoy proxy for HTTP/gRPC routing.

**API Version**: `operator.solo.hedera.com/v1alpha1`  
**Kind**: `EnvoyCapsule`  
**Scope**: Namespaced

#### NetworkOperation
Handles network-level operations (genesis, upgrade, node management).

**API Version**: `operator.solo.hedera.com/v1alpha1`  
**Kind**: `NetworkOperation`  
**Scope**: Namespaced

### Resource Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                           Sphere CR                              │
│  (Top-level network configuration)                              │
└────────────┬────────────────────────────────────────────────────┘
             │ creates
             ├──────────────┬──────────────┬──────────────┐
             ▼              ▼              ▼              ▼
    ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐
    │ Consensus  │  │ Consensus  │  │  Envoy     │  │HelmRelease │
    │ Capsule-0  │  │ Capsule-1  │  │  Capsule   │  │ (Mirror)   │
    └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └────────────┘
          │               │               │
          │ creates       │ creates       │ creates
          ▼               ▼               ▼
    ┌────────────┐  ┌────────────┐  ┌────────────┐
    │StatefulSet │  │StatefulSet │  │ Deployment │
    │   (Pod)    │  │   (Pod)    │  │   (Pod)    │
    └────────────┘  └────────────┘  └────────────┘


┌─────────────────────────────────────────────────────────────────┐
│              NetworkOperation Creation Paths                     │
└─────────────────────────────────────────────────────────────────┘

Path 1: Operator-Initiated (Config Changes)
───────────────────────────────────────────
  User updates Capsule CR
         │
         ▼
  CapsuleController detects change
         │
         ▼
  Creates NetworkOperation CR ──────┐
         │                          │
         │                          ▼
         │                  NetworkOperationController
         │                          │
         │                          ▼
         │                  Submits transaction to blockchain
         │                  (via Hedera Go SDK)
         │                          │
         │                          ▼
         │                  Waits for consensus
         │                          │
         │                          ▼
         │                  Updates NetworkOperation status
         │                          │
         ▼◄─────────────────────────┘
  Applies config to K8s resources
  (StatefulSet/Deployment)


Path 2: User-Initiated (Ledger-Only Operations)
────────────────────────────────────────────────
  User creates NetworkOperation CR
         │
         ▼
  NetworkOperationController
         │
         ▼
  Submits transaction to blockchain
  (via Hedera Go SDK)
         │
         ▼
  Waits for consensus
         │
         ▼
  Updates NetworkOperation status
         │
         ▼
  DONE (No K8s config changes)


Key Differences:
• Operator-initiated: Capsule change → NetworkOperation → Blockchain → K8s update
• User-initiated: NetworkOperation → Blockchain only (no K8s changes)
```

### NetworkOperation Usage Patterns

**Operator-Initiated** (Automatic):
- Triggered by Capsule CR spec changes
- Ensures blockchain consensus before applying K8s configuration
- CapsuleController creates NetworkOperation CR
- Includes configuration diff in spec
- After blockchain consensus, CapsuleController applies changes

**User-Initiated** (Manual):
- User directly creates NetworkOperation CR
- Performs ledger transactions only
- No Kubernetes configuration changes
- Use cases:
  - Account creation
  - Token operations
  - Smart contract deployment
  - Custom network transactions
- NetworkOperationController only submits transaction and updates status

## Sphere CRD Specification

### Spec

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Sphere
metadata:
  name: my-network
  namespace: hedera
spec:
  # Deployment Identity
  deploymentName: string (required)
  
  # Cloud Configuration
  cloud:
    provider: string (required) # gcp, aws
    region: string (required)
    serviceAccount:
      kubernetes: string (required)
      cloud: string (required) # email for GCP, ARN for AWS
  
  # Cluster Reference
  cluster:
    name: string (required)
    context: string (required)
  
  # Network Configuration
  network:
    chainId: int (default: 298)
    ledgerId: string (computed from chainId)
    realm: string (default: "0")
    shard: string (default: "0")
  
  # Consensus Configuration
  consensus:
    nodeCount: int (required, default: 4)
    version: string (required, default: "v0.60.1")
    
    # Network Settings (settings.txt)
    networkSettings: map[string]string (required)
    
    # Application Configuration (application.properties)
    applicationConfiguration: map[string]string (required)
    
    # Permission Configuration (api-permission.properties)
    permissionConfiguration: map[string]string (required)
    
    # Genesis Configuration
    genesisThrottles: string # JSON content
    
    # Logging
    log4j2XML: string (required)
    
    # Resources per node
    resources:
      requests:
        cpu: string (default: "2")
        memory: string (default: "16Gi")
      limits:
        cpu: string (default: "6")
        memory: string (default: "48Gi")
      javaHeap:
        min: string (default: "16g")
        max: string (default: "19g")
    
    # Provisioning
    provisioningModel: string (default: "InitContainerDinD")
    
    # Ports
    gossipPort: int (default: 50211)
    gossipPortTLS: int (default: 50212)
    
    # Stake
    defaultStake: int (default: 500)
  
  # Storage Configuration
  storage:
    type: string (required) # gcs_only, s3_only
    
    # Stream Storage
    stream:
      endpoint: string (required)
      bucketName: string (required)
      accessKeyId: string (required)
      accessKeySecret: string (required)
    
    # Backup Storage
    backup:
      endpoint: string (required)
      bucketName: string (required)
      accessKeyId: string (required)
      accessKeySecret: string (required)
  
  # Database Configuration
  database:
    host: string (required)
    port: int (default: 5432)
    name: string (required)
    owner:
      username: string (required)
      password: string (required)
    mirror:
      username: string (required)
      password: string (required)
  
  # Component Configuration
  components:
    mirrorNode:
      enabled: bool (default: true)
      helmChart: string (required)
      version: string
      pinger:
        enabled: bool (default: true)
        tps: float (default: 0.05)
      replicas: int (default: 2)
    
    explorer:
      enabled: bool (default: true)
      helmChart: string (required)
      version: string (default: "24.15.0")
      productName: string
      extendedProductName: string
      description: string
      cryptoName: string
      cryptoSymbol: string (default: "ξ")
    
    relay:
      enabled: bool (default: true)
      helmChart: string (required)
      websocket:
        enabled: bool (default: true)
        version: string (default: "0.69.0")
    
    mirrorApiProxy:
      enabled: bool (default: true)
      type: string (default: "envoy") # envoy, nginx
      image: string
      tag: string
  
  # Domain
  domain: string (default: "test.hashsphere.internal")
  
  # Telemetry
  telemetry:
    metrics:
      enabled: bool (default: false)
      interval: string (default: "5s")
      port: int (default: 9998)
      labels: map[string]string
      remoteWrite: bool (default: false)
    logging:
      enabled: bool (default: false)
      labels: map[string]string
  
  # Delayed Application
  applyTimestamp: string # RFC3339 timestamp, e.g., "2024-01-15T10:00:00Z"
  
  # Deletion Protection
  immutable: bool (default: false) # If true, prevents deletion without manual cleanup
```

### Status

```yaml
status:
  phase: string # Pending, Initializing, Deploying, Running, Failed, Terminating
  
  conditions: []Condition
  
  # Deployment Progress
  deployment:
    currentStep: string
    totalSteps: int
    completedSteps: int
    steps: []Step
  
  # Component Status
  consensusNodes:
    ready: int
    total: int
    nodes: []NodeStatus
  
  mirrorNode:
    phase: string
    ready: bool
    helmRelease: string
  
  explorer:
    phase: string
    ready: bool
    helmRelease: string
  
  relay:
    phase: string
    ready: bool
    helmRelease: string
  
  # Network Information
  network:
    addressBookHash: string
    systemAccounts: []string
    genesisTime: string
  
  # Timestamps
  startTime: string
  lastReconcileTime: string
  
  observedGeneration: int64
```

### Step Object

```yaml
Step:
  name: string # Step name (e.g., "ValidateSpec", "GenerateKeys")
  phase: string # Pending, Running, Completed, Failed, Skipped
  message: string # Human-readable status message
  startTime: string # RFC3339 timestamp
  completionTime: string # RFC3339 timestamp
  duration: string # Duration (e.g., "2m30s")
  error: string # Error message if failed
  retryCount: int # Number of retries attempted
```

### Step Example

```yaml
status:
  phase: Deploying
  deployment:
    currentStep: "DeployConsensusNodes"
    totalSteps: 10
    completedSteps: 3
    steps:
      - name: ValidateSpec
        phase: Completed
        message: "Spec validation successful"
        startTime: "2024-01-15T10:00:00Z"
        completionTime: "2024-01-15T10:00:02Z"
        duration: "2s"
        retryCount: 0
      
      - name: CreateInfrastructure
        phase: Completed
        message: "Created service account, ConfigMaps, and secrets"
        startTime: "2024-01-15T10:00:02Z"
        completionTime: "2024-01-15T10:00:15Z"
        duration: "13s"
        retryCount: 0
      
      - name: GenerateNodeKeys
        phase: Completed
        message: "Generated keys for 4 nodes"
        startTime: "2024-01-15T10:00:15Z"
        completionTime: "2024-01-15T10:00:45Z"
        duration: "30s"
        retryCount: 0
      
      - name: DeployConsensusNodes
        phase: Running
        message: "Creating ConsensusCapsule CRs (2/4 ready)"
        startTime: "2024-01-15T10:00:45Z"
        retryCount: 0
      
      - name: InitializeNetwork
        phase: Pending
        message: "Waiting for consensus nodes"
        retryCount: 0
```

## Controller Design

### SphereController

Orchestrates complete network deployment lifecycle.

**Reconciliation Flow**:

1. **Validate Spec**
   - Check required fields
   - Validate version compatibility
   - Verify resource availability
   - Check applyTimestamp (if set, requeue until time reached)

2. **Create Infrastructure**
   - Service account with workload identity
   - ConfigMaps for configuration assets
   - Secrets for credentials

3. **Generate Node Keys**
   - Generate Ed25519 gossip keys per node
   - Generate TLS certificates per node
   - Store in secrets

4. **Deploy Consensus Nodes**
   - Create ConsensusCapsule CR per node
   - ConsensusCapsuleController creates StatefulSets
   - Wait for nodes to be ready

5. **Initialize Network**
   - Start consensus nodes
   - Wait for network consensus

6. **Initialize Database**
   - Create PostgreSQL extensions
   - Create roles and schemas
   - Set permissions

7. **Process Addressbook**
   - Download addressbook from network
   - Generate certificate hashes
   - Merge and upload addressbook
   - Create sysfiles ConfigMap

8. **Initialize Accounts**
   - Initialize system accounts (0.0.2, treasury)
   - Generate and store account keys

9. **Deploy Components**
   - Mirror node (via HelmRelease)
   - Mirror API proxy (via EnvoyCapsule)
   - Explorer (via HelmRelease)
   - Relay (via HelmRelease)

10. **Mark Running**
    - Update status to Running
    - Set completion timestamp

**Error Handling**:
- Exponential backoff retry
- Status condition updates
- Manual intervention via annotations
- Idempotent operations

**Deletion**:
- Check if spec.immutable is true
- If immutable, add finalizer and require manual annotation to proceed
- Delete ConsensusCapsule CRs
- Delete EnvoyCapsule CRs
- Delete HelmRelease CRs
- Delete secrets and ConfigMaps
- Optional PVC deletion

**Immutable Deletion Flow**:
```yaml
# Sphere with immutable protection
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Sphere
metadata:
  name: prod-network
  finalizers:
    - sphere.operator.solo.hedera.com/immutable-protection
spec:
  immutable: true
  # ... rest of spec

# To delete, operator must add annotation
kubectl annotate sphere prod-network sphere.operator.solo.hedera.com/allow-deletion="true"
kubectl delete sphere prod-network
```

**Finalizer Logic**:
1. If spec.immutable is true, add finalizer on creation
2. On deletion, check for annotation `sphere.operator.solo.hedera.com/allow-deletion="true"`
3. If annotation missing, requeue and log warning
4. If annotation present, proceed with cleanup and remove finalizer

### ConsensusCapsuleController

Manages individual consensus node lifecycle.

**Responsibilities**:
- Create PVCs for node storage (logs, stats, states, upgrade, record-stream, events-stream)
- Create StatefulSet for node
- Manage node configuration
- Handle node upgrades
- Monitor node health

### EnvoyCapsuleController

Manages Envoy proxy for API routing.

**Responsibilities**:
- Create Envoy deployment
- Configure routing rules
- Handle proxy upgrades
- Monitor proxy health

### NetworkOperationController

Handles network-level operations by coordinating Capsule changes with blockchain transactions.

**Operation Modes**:
- `NetworkGenesis`: Initialize network
- `NetworkUpgrade`: Upgrade network
- `NodeAdd`: Add node
- `NodeDelete`: Remove node
- `NodeUpdate`: Update node

**Responsibilities**:
- Submit transactions to blockchain via Hedera Go SDK
- Monitor transaction status
- Update NetworkOperation status

**Workflow**:
1. User/Controller creates NetworkOperation CR
2. NetworkOperationController reads operation spec
3. Controller submits transaction to blockchain using Go SDK
4. Controller monitors transaction consensus
5. Controller updates NetworkOperation status

## Capsule and NetworkOperation Relationship

### Configuration Change Flow

**Operator-Initiated Changes**:
1. Operator updates Capsule CR spec (e.g., ConsensusCapsule version)
2. CapsuleController detects spec change
3. CapsuleController creates NetworkOperation CR
4. NetworkOperationController submits transaction to blockchain via Go SDK
5. Blockchain reaches consensus on change
6. CapsuleController applies configuration change to StatefulSet/Deployment
7. Pods restart with new configuration

**Example: Node Upgrade**
```yaml
# 1. Operator updates ConsensusCapsule
apiVersion: operator.solo.hedera.com/v1alpha1
kind: ConsensusCapsule
metadata:
  name: node-0
spec:
  version: v0.61.0  # Changed from v0.60.1
  # ... other fields

---
# 2. ConsensusCapsuleController creates NetworkOperation
apiVersion: operator.solo.hedera.com/v1alpha1
kind: NetworkOperation
metadata:
  name: node-0-upgrade
  ownerReferences:
    - apiVersion: operator.solo.hedera.com/v1alpha1
      kind: ConsensusCapsule
      name: node-0
spec:
  mode: NodeUpdate
  nodeOperation:
    nodeId: 0
    newVersion: v0.61.0
  # Configuration diff that triggered this operation
  configurationDiff:
    field: spec.version
    oldValue: v0.60.1
    newValue: v0.61.0
    timestamp: "2024-01-15T10:30:00Z"

---
# 3. NetworkOperationController submits transaction
# Uses Hedera Go SDK to submit NodeUpdateTransaction
# Waits for consensus

---
# 4. ConsensusCapsuleController applies change
# Updates StatefulSet with new image
# Waits for pod to be ready
```

**Reconciliation Loop**:
```
CapsuleController:
  Watch Capsule CR spec changes
  ↓
  Detect configuration drift (generate diff)
  ↓
  Create NetworkOperation CR with diff reference
  ↓
  Wait for NetworkOperation.status = Success
  ↓
  Apply configuration to Kubernetes resources
  ↓
  Update Capsule status

NetworkOperationController:
  Watch NetworkOperation CR
  ↓
  Read configuration diff from spec
  ↓
  Submit transaction via Hedera Go SDK
  ↓
  Poll transaction status
  ↓
  Update NetworkOperation status with transaction ID
```

**Benefits**:
- Declarative configuration changes
- Blockchain consensus before Kubernetes changes
- Audit trail via NetworkOperation CRs
- Automatic rollback on transaction failure

## Delayed Application with applyTimestamp

### Implementation

Kubernetes controllers can implement delayed reconciliation using `RequeueAfter`:

```go
func (r *SphereReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    sphere := &Sphere{}
    if err := r.Get(ctx, req.NamespacedName, sphere); err != nil {
        return ctrl.Result{}, err
    }
    
    // Check if applyTimestamp is set
    if sphere.Spec.ApplyTimestamp != "" {
        applyTime, err := time.Parse(time.RFC3339, sphere.Spec.ApplyTimestamp)
        if err != nil {
            return ctrl.Result{}, err
        }
        
        now := time.Now()
        if now.Before(applyTime) {
            // Requeue at exact apply time
            delay := applyTime.Sub(now)
            return ctrl.Result{RequeueAfter: delay}, nil
        }
    }
    
    // Proceed with normal reconciliation
    // ...
}
```

### Timing Precision

**Accuracy**: ~1-5 seconds
- Kubernetes reconciliation is not real-time
- Controller runtime uses work queues with rate limiting
- System load and API server latency affect timing

**Best Practices**:
- Use for minute-level precision (e.g., maintenance windows)
- Not suitable for sub-second precision
- Add status field to track actual application time

### Status Tracking

```yaml
status:
  phase: string
  scheduledApplyTime: string # From spec.applyTimestamp
  actualApplyTime: string # When reconciliation actually started
  applyDelay: string # Difference between scheduled and actual
```

### Example Usage

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Sphere
metadata:
  name: dev-network
spec:
  # Schedule deployment for 2AM UTC
  applyTimestamp: "2024-01-16T02:00:00Z"
  
  deploymentName: dev-network
  consensus:
    nodeCount: 4
    version: v0.60.1
  # ... rest of spec
```

### Limitations

1. **Not Guaranteed**: Kubernetes doesn't guarantee exact timing
2. **Controller Restarts**: If controller restarts, reconciliation may be delayed
3. **Clock Skew**: Relies on controller's system clock
4. **Past Timestamps**: If timestamp is in the past, applies immediately

## Processor Framework

### KeyGenerationProcessor
- Generate Ed25519 keys for gossip
- Generate TLS certificates
- Store in secrets: `node-{id}-gossip-keys`, `node-{id}-tls-keys`

### DatabaseInitProcessor
- Connect to PostgreSQL
- Run initialization SQL
- Create extensions: pg_stat_statements, pg_trgm
- Create roles: owner, mirror user
- Create schemas and set permissions

### AddressbookProcessor
- Download addressbook via gRPC
- Parse and process
- Generate cert hashes from node keys
- Merge cert hashes with addressbook
- Upload processed addressbook
- Create ConfigMap: addressBook.json, nodeDetails.json

### AccountInitProcessor
- Initialize system accounts
- Generate Ed25519 account keys
- Store in secrets: `account-key-{accountId}`

### HelmDeploymentProcessor
- Create HelmRelease CRs (Flux CD)
- Generate values from Sphere spec
- Watch HelmRelease status
- Flux handles installation, upgrades, rollbacks

## Resource Management

### Secrets
- `node-{id}-gossip-keys`: Per-node gossip keys
- `node-{id}-tls-keys`: Per-node TLS keys
- `account-key-{accountId}`: System account keys
- `mirror-node-config`: Mirror configuration
- `relay-node-config`: Relay configuration
- `stream-bucket-credentials`: Stream storage credentials
- `backup-bucket-credentials`: Backup storage credentials
- `database-credentials`: PostgreSQL credentials

### ConfigMaps
- `sphere-assets`: Application assets (settings.txt, log4j2.xml, application.properties, throttles.json)
- `sysfiles`: Addressbook and node details

### PersistentVolumeClaims
Per-node PVCs (created by ConsensusCapsuleController):
- `node-{id}-logs`: Node logs
- `node-{id}-stats`: Node statistics
- `node-{id}-states`: Node state data
- `node-{id}-upgrade`: Upgrade artifacts
- `node-{id}-record-stream`: Record stream data
- `node-{id}-events-stream`: Event stream data

## Helm Integration

### Flux CD Helm Controller

Use Flux CD HelmRelease CRs for declarative Helm management:
- SphereController creates HelmRelease CRs
- Flux helm-controller manages Helm operations
- Automatic drift detection and reconciliation
- Built-in rollback on failure

### Deployment Sequencing with Dependencies

SphereController orchestrates deployment sequence:
1. **Consensus Nodes**: ConsensusCapsule CRs create StatefulSets (operator-managed)
2. **Wait for Consensus**: Poll ConsensusCapsule status until all nodes ready
3. **Mirror Node**: Create HelmRelease with no dependencies
4. **Explorer + Relay**: Create HelmRelease CRs with `dependsOn: mirror-node`

```yaml
# Mirror node - waits for consensus via SphereController logic
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: mirror-node
spec:
  chart:
    spec:
      chart: hedera-mirror
      version: 1.0.0
      sourceRef:
        kind: HelmRepository
        name: hedera-charts
  interval: 5m

---
# Explorer depends on mirror node
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: explorer
spec:
  dependsOn:
    - name: mirror-node
  chart:
    spec:
      chart: hiero-explorer-chart
      version: 1.0.0
      sourceRef:
        kind: HelmRepository
        name: hedera-charts

---
# Relay depends on mirror node
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: relay
spec:
  dependsOn:
    - name: mirror-node
  chart:
    spec:
      chart: hedera-json-rpc-relay
      version: 1.0.0
      sourceRef:
        kind: HelmRepository
        name: hedera-charts
```

**Dependency Chain**:
1. Consensus nodes (operator-managed StatefulSets, parallel deployment)
2. SphereController waits for all ConsensusCapsule CRs to be Ready
3. Mirror node HelmRelease created (Flux deploys)
4. Explorer + Relay HelmReleases created with `dependsOn: mirror-node` (Flux enforces order)

**Benefits**:
- Operator controls consensus node deployment timing
- Flux enforces component dependencies automatically
- No manual sequencing logic for Helm components
- Automatic retry on dependency failures

### Required Charts
- `hedera-mirror`: Mirror node (importer, grpc, rest, web3)
- `hiero-explorer-chart`: Blockchain explorer
- `hedera-json-rpc-relay`: JSON-RPC relay

Note: Consensus nodes are deployed via operator-managed StatefulSets, not Helm charts.

### HelmRelease Example

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: mirror-node
  namespace: hedera
spec:
  chart:
    spec:
      chart: hedera-mirror
      version: 1.0.0
      sourceRef:
        kind: HelmRepository
        name: hedera-charts
  interval: 5m
  values:
    # Generated from Sphere spec
    database:
      host: postgres.example.com
      name: mirror_node
    storage:
      type: gcs
      bucket: stream-bucket
```

### Prerequisites
- Flux CD helm-controller installed
- HelmRepository CRs configured

## Observability

### Status Reporting
- Current deployment step
- Progress percentage
- Component health status
- Error messages
- Resource references

### Events
- Phase transitions
- Step completion
- Errors and warnings
- Component deployments

### Metrics
- Deployment duration
- Step duration
- Failure count
- Component health

## Example Usage

```yaml
apiVersion: operator.solo.hedera.com/v1alpha1
kind: Sphere
metadata:
  name: dev-network
  namespace: hedera-dev
spec:
  deploymentName: dev-network
  
  cloud:
    provider: gcp
    region: us-central1
    serviceAccount:
      kubernetes: sphere-sa
      cloud: sphere-sa@project.iam.gserviceaccount.com
  
  cluster:
    name: dev-cluster
    context: gke_project_us-central1_dev-cluster
  
  network:
    chainId: 298
  
  consensus:
    nodeCount: 4
    nodeSize: medium
    version: v0.60.1
    networkSettings:
      deployment: hashsphere
      checkSignedStateFromDisk: "1"
      doUpnp: "false"
    applicationConfiguration:
      hedera.realm: "0"
    permissionConfiguration:
      default: "ALL"
    log4j2XML: |
      <Configuration>
        <Appenders>
          <Console name="Console" target="SYSTEM_OUT"/>
        </Appenders>
        <Loggers>
          <Root level="info">
            <AppenderRef ref="Console"/>
          </Root>
        </Loggers>
      </Configuration>
  
  storage:
    type: gcs_only
    stream:
      endpoint: https://storage.googleapis.com
      bucketName: dev-stream-bucket
      accessKeyId: GOOG...
      accessKeySecret: secret...
    backup:
      endpoint: https://storage.googleapis.com
      bucketName: dev-backup-bucket
      accessKeyId: GOOG...
      accessKeySecret: secret...
  
  database:
    host: 10.0.0.5
    port: 5432
    name: mirror_node
    owner:
      username: postgres
      password: secret
    mirror:
      username: mirror_node
      password: secret
  
  components:
    mirrorNode:
      enabled: true
      helmChart: oci://registry/hedera-mirror
    explorer:
      enabled: true
      helmChart: oci://registry/hiero-explorer-chart
      productName: Dev Network
    relay:
      enabled: true
      helmChart: oci://registry/hedera-json-rpc-relay
```

## Implementation Plan

### Phase 1: Core CRD (Week 1)
- Define Sphere CRD
- Implement SphereController skeleton
- Basic status reporting
- Create ConsensusCapsule CRs

### Phase 2: Deployment (Weeks 2-3)
- Key generation processor
- Database initialization processor
- Addressbook processor
- Account initialization processor

### Phase 3: Components (Week 4)
- Flux CD HelmRelease integration
- Mirror node deployment
- Explorer deployment
- Relay deployment

### Phase 4: Production (Week 5)
- E2E tests
- Documentation
- Performance optimization
- Security hardening
