# UC Test Scenarios

Structured test scenarios for the Upgrade Controller (UC) sidecar, covering
happy-path and failure-path behaviours. Each scenario follows a consistent
**Prepare → Trigger → Verify** format with fully executable commands.

For initial setup (building the binary, installing CRDs, creating scratch
directories) see the [Testing Guide](testing-guide.md) §1–§3.

---

## Prerequisites

All scenarios assume the environment from [testing-guide.md §3](testing-guide.md#3-run-the-sidecar-against-scratch-directories):

```bash
export UCDIR=$(mktemp -d)
mkdir -p "$UCDIR/current" "$UCDIR/previous" "$UCDIR/events" "$UCDIR/crs" "$UCDIR/backup" "$UCDIR/logs"
```

A **fake CN metrics endpoint** is used by many scenarios. Set it up once and
leave it running; individual scenarios update the served file as needed:

```bash
mkdir -p /tmp/fake-metrics
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
cd /tmp/fake-metrics && python3 -m http.server 9999 &
METRICS_PID=$!
```

The **UC base command** runs the UC in the background with output captured to a
log file so that Verify steps can grep it. Copy this block before starting a
scenario (adjust flags per scenario):

```bash
./bin/uc \
  --mode cluster-only \
  --node-id 0 \
  --orbit my-orbit \
  --namespace solo \
  --pod-name <pod-name> \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path  "$UCDIR/current" \
  --previous-upgrade-path "$UCDIR/previous" \
  --events-path  "$UCDIR/events" \
  --cr-persist-path "$UCDIR/crs" \
  --backup-path  "$UCDIR/backup" \
  --socket-path  "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url  "http://localhost:9999/metrics" \
  --platform-poll-interval 5s \
  --poll-interval 5s \
  --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

Replace `<pod-name>` with a real pod name in the cluster when testing pod
condition patching (H3, F3). For scenarios that do not patch pods, any value
works (the patch will fail harmlessly).

**Cleanup between scenarios:**

```bash
kill $UC_PID 2>/dev/null; wait $UC_PID 2>/dev/null
rm -rf "$UCDIR"
export UCDIR=$(mktemp -d)
mkdir -p "$UCDIR/current" "$UCDIR/previous" "$UCDIR/events" "$UCDIR/crs" "$UCDIR/backup" "$UCDIR/logs"
```

---

## Happy-Path Scenarios

### H1. Metrics scrape detects ACTIVE

**Component:** PlatformStatusMonitor

**Prepare:**
```bash
# Serve ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 6
```

**Verify:**
```bash
# 1. Check UC log for PlatformStatusChanged transition
grep PlatformStatusChanged "$UCDIR/uc.log"
# Expected: ... to=ACTIVE code=2

# 2. Query the consensus status endpoint
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.metrics'
# Expected: {"code": 2, "status": "ACTIVE", "scrapedAt": "...", "fresh": true}

# 3. Check Prometheus metrics
curl -s http://localhost:9990/metrics | grep 'uc_consensus_status.*source="metrics"'
# Expected: uc_consensus_status{source="metrics",node_id="0"} 2

kill $UC_PID 2>/dev/null
```

---

### H2. Platform status transition (STARTING_UP → ACTIVE)

**Component:** PlatformStatusMonitor

**Prepare:**
```bash
# Serve STARTING_UP (code 1)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 1.0
EOF

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6
```

**Trigger:**
```bash
# Switch to ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
sleep 6
```

**Verify:**
```bash
# 1. Two transitions logged
grep PlatformStatusChanged "$UCDIR/uc.log"
# Expected: first line shows to=STARTING_UP, second shows from=STARTING_UP to=ACTIVE

# 2. Effective status is ACTIVE
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.effective'
# Expected: {"code": 2, "status": "ACTIVE", "source": "metrics"}

# 3. Events trail shows both transitions
cat "$UCDIR/events/platform-status.jsonl" | jq .
# Expected: two JSONL entries with reason PlatformStatusChanged

kill $UC_PID 2>/dev/null
```

---

### H3. Pod condition patched on status change

**Component:** PlatformStatusMonitor

**Prepare:**
```bash
# Serve ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF

# Get a real pod name from the cluster
POD_NAME=$(kubectl get pods -n solo -l app.kubernetes.io/part-of=consensus-node -o jsonpath='{.items[0].metadata.name}')

# Start the UC with the real pod name
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name "$POD_NAME" \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 6
```

**Verify:**
```bash
# 1. Pod condition exists with ACTIVE
kubectl get pod "$POD_NAME" -n solo -o jsonpath='{.status.conditions}' | \
  jq '.[] | select(.type=="operator.solo.hiero.org/platform-status")'
# Expected: reason="ACTIVE", status="True", message contains "code 2"

# 2. Transition to FREEZE_COMPLETE
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 6.0
EOF
sleep 6

kubectl get pod "$POD_NAME" -n solo -o jsonpath='{.status.conditions}' | \
  jq '.[] | select(.type=="operator.solo.hiero.org/platform-status")'
# Expected: reason="FREEZE_COMPLETE"

kill $UC_PID 2>/dev/null
```

---

### H4. Log monitor detects PLATFORM_STATUS transition

**Component:** LogMonitor

**Prepare:**
```bash
# Ensure log file does not exist yet
rm -f "$UCDIR/logs/swirlds.log"

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2
```

**Trigger:**
```bash
echo '2026-07-23T10:00:00.000Z PLATFORM_STATUS {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}' \
  >> "$UCDIR/logs/swirlds.log"
sleep 2
```

**Verify:**
```bash
# 1. Log monitor detected the transition
grep LogStatusChanged "$UCDIR/uc.log"
# Expected: from=STARTING_UP to=ACTIVE code=2

# 2. Control-plane shows log status
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.log'
# Expected: {"code": 2, "status": "ACTIVE", "fileModifiedAt": "..."}

# 3. Prometheus metric
curl -s http://localhost:9990/metrics | grep 'uc_consensus_status.*source="log"'
# Expected: uc_consensus_status{source="log",node_id="0"} 2

kill $UC_PID 2>/dev/null
```

---

### H5. Log monitor persists status across restart

**Component:** LogMonitor

**Prepare:**
```bash
# Run H4 first so the log monitor has detected ACTIVE, then:
cat "$UCDIR/crs/log-monitor-status.json"
# Expected: {"code":2,"status":"ACTIVE"}
```

**Trigger:**
```bash
# Stop the UC
kill $UC_PID 2>/dev/null; wait $UC_PID 2>/dev/null

# Remove the log file (simulate fresh start without log history)
rm -f "$UCDIR/logs/swirlds.log"

# Restart the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2
```

**Verify:**
```bash
# 1. Log monitor restored from persist file
grep LogMonitorRestored "$UCDIR/uc.log"
# Expected: code=2 status=ACTIVE

# 2. Log status available immediately on startup
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.log'
# Expected: {"code": 2, "status": "ACTIVE", ...} — no new log line needed

kill $UC_PID 2>/dev/null
```

---

### H6. Effective source reconciliation (metrics fresh)

**Component:** ConsensusStatusCollector

**Prepare:**
```bash
# Serve ACTIVE (code 2) from metrics
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF

# Write a log line with BEHIND (code 4) — different from metrics
mkdir -p "$UCDIR/logs"
echo '2026-07-23T10:00:00.000Z PLATFORM_STATUS {"oldStatus":"STARTING_UP","newStatus":"BEHIND"}' \
  > "$UCDIR/logs/swirlds.log"

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 6
```

**Verify:**
```bash
# 1. Effective uses metrics (fresh, takes priority over log)
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq .
# Expected: effective.code=2, effective.source="metrics", metrics.fresh=true, log.code=4

# 2. Prometheus effective gauge is 2 (from metrics, not log's 4)
curl -s http://localhost:9990/metrics | grep 'source="effective"'
# Expected: uc_consensus_status{source="effective",node_id="0"} 2

kill $UC_PID 2>/dev/null
```

---

### H7. Effective source falls back to log (metrics stale)

**Component:** ConsensusStatusCollector

**Prepare:**
```bash
# Run H6 first so both monitors have data, then:
```

**Trigger:**
```bash
# Kill the fake metrics endpoint
kill $METRICS_PID 2>/dev/null

# Wait past the staleness window (15s default) + one poll interval
sleep 21
```

**Verify:**
```bash
# 1. Metrics are stale, effective falls back to log
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq .
# Expected: metrics.fresh=false, effective.source="log", effective.code=4 (BEHIND)

# 2. Prometheus effective gauge changed to log value
curl -s http://localhost:9990/metrics | grep 'source="effective"'
# Expected: uc_consensus_status{source="effective",node_id="0"} 4

kill $UC_PID 2>/dev/null

# Restart the fake metrics endpoint for subsequent scenarios
cd /tmp/fake-metrics && python3 -m http.server 9999 &
METRICS_PID=$!
```

---

### H8. /consensus/status returns compact JSON

**Component:** Control-plane handler

**Prepare:**
```bash
# Serve ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF

# Write a log transition
mkdir -p "$UCDIR/logs"
echo '2026-07-23T10:00:00.000Z PLATFORM_STATUS {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}' \
  > "$UCDIR/logs/swirlds.log"

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6
```

**Trigger:**
```bash
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq .
```

**Verify:**
```bash
# 1. Response has all required top-level fields
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | \
  jq 'keys' 
# Expected: ["effective","log","metrics","nodeId","now","snapshotAt","staleness"]

# 2. Timestamps are recent (within last 60s)
NOW=$(date +%s)
SNAP=$(curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | \
  jq -r '.now' | xargs -I{} date -j -f "%Y-%m-%dT%H:%M:%SZ" {} +%s 2>/dev/null || \
  date -d "$(curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq -r '.now')" +%s)
echo "Time delta: $(( NOW - SNAP ))s"
# Expected: delta < 60

# 3. scrapedAt present in metrics
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | \
  jq '.metrics.scrapedAt'
# Expected: non-null RFC3339 timestamp

# 4. fileModifiedAt present in log
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | \
  jq '.log.fileModifiedAt'
# Expected: non-null RFC3339 timestamp

# 5. staleness is "15s"
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | \
  jq -r '.staleness'
# Expected: 15s

kill $UC_PID 2>/dev/null
```

---

### H9. /metrics exposes uc_consensus_status gauges

**Component:** Metrics server

**Prepare:**
```bash
# Serve ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6
```

**Trigger:**
```bash
curl -s http://localhost:9990/metrics > "$UCDIR/metrics-output.txt"
```

**Verify:**
```bash
# 1. Three source labels for uc_consensus_status
grep 'uc_consensus_status' "$UCDIR/metrics-output.txt"
# Expected: three lines with source="metrics", source="log", source="effective"

# 2. Scrape and log timestamp gauges
grep 'uc_platform_last_scrape_timestamp' "$UCDIR/metrics-output.txt"
grep 'uc_consensus_log_timestamp' "$UCDIR/metrics-output.txt"
# Expected: non-zero Unix timestamps

# 3. Self-health metrics
grep 'uc_build_info' "$UCDIR/metrics-output.txt"
grep 'uc_up' "$UCDIR/metrics-output.txt"
grep 'uc_monitor_up' "$UCDIR/metrics-output.txt"
# Expected: uc_up 1, uc_monitor_up entries for each monitor

# 4. Go runtime and process metrics
grep 'go_goroutines' "$UCDIR/metrics-output.txt"
grep 'process_open_fds' "$UCDIR/metrics-output.txt"
# Expected: present

kill $UC_PID 2>/dev/null
```

---

### H10. Operator rolls up capsule phase to Active

**Component:** Operator reconciler (ConsensusCapsule)

**Prepare:**
```bash
# Start the operator
task dev &
sleep 10

# Get a real consensus pod name
POD_NAME=$(kubectl get pods -n solo -l app.kubernetes.io/part-of=consensus-node \
  -o jsonpath='{.items[0].metadata.name}')

# Serve STARTING_UP (code 1)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 1.0
EOF

# Start the UC with the real pod name
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name "$POD_NAME" \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6
```

**Trigger:**
```bash
# Switch to ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
sleep 10
```

**Verify:**
```bash
# 1. Capsule phase rolled up to Active
kubectl get consensuscapsule -n solo \
  -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,PLATFORM:.status.platformStatus
# Expected: PHASE=Active, PLATFORM=ACTIVE

# 2. platformStatusCode is 2
kubectl get consensuscapsule -n solo \
  -o jsonpath='{.items[0].status.platformStatusCode}'
# Expected: 2

kill $UC_PID 2>/dev/null
```

---

### H11. Execute CR succeeds on capsule phase=Active

**Component:** NetworkUpgradeExecute reconciler

**Prepare:**
```bash
# Ensure H10 is complete — capsule is in phase=Active

# Create a Prepare CR (see testing-guide.md §4a)
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML
touch "$UCDIR/current/execute_immediate.mf"
sleep 10
```

**Trigger:**
```bash
# Wait for the operator to drive the upgrade through to the ACTIVE gate
# The Execute reconciler polls capsule.status.phase — capsule is Active
sleep 30
```

**Verify:**
```bash
# 1. Execute CR succeeded
kubectl get networkupgradeexecutes -n solo \
  -o jsonpath='{.items[0].status.phase}'
# Expected: Succeeded

# 2. NodesUpgraded condition is True
kubectl get networkupgradeexecutes -n solo -o yaml | grep -A3 NodesUpgraded
# Expected: status: "True", message about ACTIVE

kill $UC_PID 2>/dev/null
```

---

### H12. Sticky-Active: transient dip keeps phase=Active

**Component:** Operator reconciler (ConsensusCapsule)

**Prepare:**
```bash
# Ensure H10 is complete — capsule is in phase=Active, platformStatus=ACTIVE
```

**Trigger:**
```bash
# Dip to BEHIND (code 4)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 4.0
EOF
sleep 10
```

**Verify:**
```bash
# 1. Phase stays Active (sticky), platformStatus shows BEHIND
kubectl get consensuscapsule -n solo \
  -o custom-columns=PHASE:.status.phase,PLATFORM:.status.platformStatus
# Expected: PHASE=Active (sticky), PLATFORM=BEHIND

# 2. Return to ACTIVE — phase stays Active
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
sleep 10
kubectl get consensuscapsule -n solo \
  -o custom-columns=PHASE:.status.phase,PLATFORM:.status.platformStatus
# Expected: PHASE=Active, PLATFORM=ACTIVE

# 3. RECONNECT_COMPLETE (code 10) — phase still Active (sticky)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 10.0
EOF
sleep 10
kubectl get consensuscapsule -n solo \
  -o custom-columns=PHASE:.status.phase,PLATFORM:.status.platformStatus
# Expected: PHASE=Active (sticky), PLATFORM=RECONNECT_COMPLETE

kill $UC_PID 2>/dev/null
```

---

## Failure-Path Scenarios

### F1. CN metrics endpoint unreachable

**Component:** PlatformStatusMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Ensure NO metrics endpoint is running on port 9999
kill $METRICS_PID 2>/dev/null

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 6
```

**Verify:**
```bash
# 1. Connectivity error logged
grep PlatformMetricsUnreachable "$UCDIR/uc.log"
# Expected: connection refused error

# 2. /status shows ConnectivityError
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq '."platform-status-monitor"'
# Expected: ConnectivityError with reason PlatformMetricsUnreachable

# 3. Consensus status shows UNKNOWN
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.metrics'
# Expected: code=-1, status="UNKNOWN", fresh=false

# 4. Self-heal: start the metrics endpoint
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
cd /tmp/fake-metrics && python3 -m http.server 9999 &
METRICS_PID=$!
sleep 6

# 5. ConnectivityError clears, metrics.code becomes 2
grep PlatformStatusChanged "$UCDIR/uc.log" | tail -1
# Expected: to=ACTIVE code=2

curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.metrics'
# Expected: code=2, status="ACTIVE", fresh=true

kill $UC_PID 2>/dev/null
```

---

### F2. CN metrics returns no platform_PlatformStatus sample

**Component:** PlatformStatusMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Serve a metrics endpoint with no platform status sample
cat > /tmp/fake-metrics/metrics <<'EOF'
# HELP some_other_metric A different metric
# TYPE some_other_metric gauge
some_other_metric 42
EOF

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 6
```

**Verify:**
```bash
# 1. No PlatformMetricsUnreachable — endpoint is up, just missing the sample
grep PlatformMetricsUnreachable "$UCDIR/uc.log" && echo "FAIL: should not log unreachable" || echo "OK: no unreachable log"

# 2. No ConnectivityError in /status (endpoint is reachable)
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq '."platform-status-monitor"'
# Expected: no ConnectivityError

# 3. Metrics code is UNKNOWN, fresh is false
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.metrics'
# Expected: code=-1, fresh=false

# 4. Self-heal: add the platform status sample
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
sleep 6

curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.metrics'
# Expected: code=2, status="ACTIVE", fresh=true

kill $UC_PID 2>/dev/null
```

---

### F3. Pod condition patch fails (RBAC missing)

**Component:** PlatformStatusMonitor
**Recovery:** self-heal after RBAC fix

**Prepare:**
```bash
# Create a ServiceAccount with limited permissions (no pods/status patch)
kubectl create serviceaccount uc-test-nopatch -n solo 2>/dev/null
kubectl create role uc-test-nopatch -n solo \
  --verb=get,list --resource=pods 2>/dev/null
kubectl create rolebinding uc-test-nopatch -n solo \
  --role=uc-test-nopatch --serviceaccount=solo:uc-test-nopatch 2>/dev/null

# Get a real pod name
POD_NAME=$(kubectl get pods -n solo -l app.kubernetes.io/part-of=consensus-node \
  -o jsonpath='{.items[0].metadata.name}')

# Serve ACTIVE (code 2)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF

# Start the UC with the restricted kubeconfig
# (Use --as for impersonation, or generate a restricted kubeconfig)
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name "$POD_NAME" \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 6
```

**Verify:**
```bash
# 1. Patch failure logged on every poll tick
grep PlatformStatusPatchFailed "$UCDIR/uc.log"
# Expected: ERROR log with forbidden / RBAC error

# 2. Monitor keeps running — scrape still works
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.metrics'
# Expected: code=2 (scrape succeeds, only the patch fails)

# 3. Grant RBAC permission
kubectl create role uc-test-patch -n solo \
  --verb=patch --resource=pods/status 2>/dev/null
kubectl create rolebinding uc-test-patch -n solo \
  --role=uc-test-patch --serviceaccount=solo:uc-test-nopatch 2>/dev/null
sleep 6

# 4. Patch succeeds, ERROR logs stop
kubectl get pod "$POD_NAME" -n solo -o jsonpath='{.status.conditions}' | \
  jq '.[] | select(.type=="operator.solo.hiero.org/platform-status")'
# Expected: condition present with reason="ACTIVE"

kill $UC_PID 2>/dev/null

# Cleanup
kubectl delete rolebinding uc-test-nopatch uc-test-patch -n solo 2>/dev/null
kubectl delete role uc-test-nopatch uc-test-patch -n solo 2>/dev/null
kubectl delete serviceaccount uc-test-nopatch -n solo 2>/dev/null
```

---

### F4. Consensus log file absent

**Component:** LogMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Ensure the log file does NOT exist
rm -f "$UCDIR/logs/swirlds.log"

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
sleep 3
```

**Verify:**
```bash
# 1. ConnectivityError logged
grep LogFileUnreadable "$UCDIR/uc.log"
# Expected: connectivity error on the log monitor

# 2. /status shows ConnectivityError
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq '."log-monitor"'
# Expected: ConnectivityError with reason LogFileUnreadable

# 3. Log status is UNKNOWN
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.log'
# Expected: code=-1, status="UNKNOWN"

# 4. Self-heal: create the log file with a status line
echo '2026-07-23T10:00:00.000Z PLATFORM_STATUS {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}' \
  > "$UCDIR/logs/swirlds.log"
sleep 2

# 5. ConnectivityError clears
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.log'
# Expected: code=2, status="ACTIVE"

kill $UC_PID 2>/dev/null
```

---

### F5. Log file rotation (file shrinks)

**Component:** LogMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2

# Write several lines to advance the monitor's read offset
for i in 1 2 3 4 5; do
  echo "2026-07-23T10:00:0${i}.000Z PLATFORM_STATUS {\"oldStatus\":\"STARTING_UP\",\"newStatus\":\"ACTIVE\"}" \
    >> "$UCDIR/logs/swirlds.log"
done
sleep 2
```

**Trigger:**
```bash
# Truncate and write a new, shorter file (simulating log rotation)
echo '2026-07-23T11:00:00.000Z PLATFORM_STATUS {"oldStatus":"ACTIVE","newStatus":"FREEZE_COMPLETE"}' \
  > "$UCDIR/logs/swirlds.log"
sleep 2
```

**Verify:**
```bash
# 1. Monitor detects rotation (file size < offset), resets, re-scans
grep LogStatusChanged "$UCDIR/uc.log" | tail -1
# Expected: from=ACTIVE to=FREEZE_COMPLETE

# 2. Log status updated to FREEZE_COMPLETE
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.log'
# Expected: code=6, status="FREEZE_COMPLETE"

kill $UC_PID 2>/dev/null
```

---

### F6. Log persist file corrupt

**Component:** LogMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Write a corrupt persist file
echo 'NOT_VALID_JSON{{{' > "$UCDIR/crs/log-monitor-status.json"

# Write a valid status line in the log file
mkdir -p "$UCDIR/logs"
echo '2026-07-23T10:00:00.000Z PLATFORM_STATUS {"oldStatus":"STARTING_UP","newStatus":"ACTIVE"}' \
  > "$UCDIR/logs/swirlds.log"
```

**Trigger:**
```bash
# Start the UC — it will try to load the corrupt persist file
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 3
```

**Verify:**
```bash
# 1. No crash — UC started successfully
kill -0 $UC_PID 2>/dev/null && echo "OK: UC running" || echo "FAIL: UC crashed"

# 2. seedFromTail recovered status from the log file
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/consensus/status | jq '.log'
# Expected: code=2, status="ACTIVE" (recovered from log, not persist file)

# 3. Persist file overwritten with valid JSON
cat "$UCDIR/crs/log-monitor-status.json"
# Expected: valid JSON {"code":2,"status":"ACTIVE"}

kill $UC_PID 2>/dev/null
```

---

### F7. Marker directory absent

**Component:** MarkerMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Remove the marker directory
rm -rf "$UCDIR/current"

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
```

**Trigger:**
```bash
# Wait two poll intervals — monitor polls but finds nothing
sleep 12
```

**Verify:**
```bash
# 1. No crash or panic
kill -0 $UC_PID 2>/dev/null && echo "OK: UC running" || echo "FAIL: UC crashed"

# 2. No marker-related log lines
grep -c MarkerDetected "$UCDIR/uc.log"
# Expected: 0

# 3. Self-heal: recreate directory and drop a marker
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML
touch "$UCDIR/current/execute_immediate.mf"
sleep 6

# 4. Marker detected and CR delivered
grep MarkerDetected "$UCDIR/uc.log"
grep CRDelivered "$UCDIR/uc.log"
# Expected: both lines present

kill $UC_PID 2>/dev/null
```

---

### F8. Invalid manifest (bad schemaVersion)

**Component:** MarkerMonitor / factory
**Recovery:** manual intervention

**Prepare:**
```bash
# Create a manifest with an invalid schema version
mkdir -p "$UCDIR/current/manifests"
echo 'schemaVersion: 99' > "$UCDIR/current/manifests/consensus-node-components.yaml"

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2
```

**Trigger:**
```bash
touch "$UCDIR/current/execute_immediate.mf"
sleep 6
```

**Verify:**
```bash
# 1. ManifestInvalid error logged
grep ManifestInvalid "$UCDIR/uc.log"
# Expected: ERROR log indicating invalid schema version

# 2. No CR created
grep CRDelivered "$UCDIR/uc.log" && echo "FAIL: CR should not be delivered" || echo "OK: no CR delivered"

# 3. No retry spam on subsequent ticks (dedup skips)
FIRST_COUNT=$(grep -c ManifestInvalid "$UCDIR/uc.log")
sleep 12
SECOND_COUNT=$(grep -c ManifestInvalid "$UCDIR/uc.log")
echo "Errors: $FIRST_COUNT → $SECOND_COUNT"
# Expected: count stays the same (no retry spam)

# 4. Manual fix: replace manifest, touch a fresh marker to mint a new operationId
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML
rm "$UCDIR/current/execute_immediate.mf"
touch "$UCDIR/current/execute_immediate.mf"
sleep 6

grep CRDelivered "$UCDIR/uc.log"
# Expected: now a CRDelivered line appears

kill $UC_PID 2>/dev/null
```

---

### F9. K8s API unreachable during CR create

**Component:** Factory / deliverCR
**Recovery:** self-heal

**Prepare:**
```bash
# Set up valid marker and manifest
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML

# Start the UC with a kubeconfig pointing to an unreachable API server
cat > "$UCDIR/bad-kubeconfig.yaml" <<'EOF'
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:1
  name: unreachable
contexts:
- context:
    cluster: unreachable
    namespace: solo
  name: unreachable
current-context: unreachable
EOF

./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$UCDIR/bad-kubeconfig.yaml" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2
```

**Trigger:**
```bash
touch "$UCDIR/current/execute_immediate.mf"
sleep 10
```

**Verify:**
```bash
# 1. CRCreateFailed error logged
grep CRCreateFailed "$UCDIR/uc.log"
# Expected: ERROR log on each poll tick

# 2. Persisted file written to disk (persist-before-send)
ls "$UCDIR/crs"/*.yaml 2>/dev/null
# Expected: a .prepare.yaml file persisted to disk

# 3. Stop the UC and restart with a valid kubeconfig
kill $UC_PID 2>/dev/null; wait $UC_PID 2>/dev/null

./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 10

# 4. RecoverPersistedCRs finds and re-sends the CR
grep RecoverPersistedCRs "$UCDIR/uc.log"
# Expected: recovery picks up the persisted file

kill $UC_PID 2>/dev/null
```

---

### F10. CR admission rejected (ValidatingAdmissionPolicy)

**Component:** Factory / deliverCR
**Recovery:** self-heal after fix

**Prepare:**
```bash
# Install a ValidatingAdmissionPolicy that rejects NetworkUpgradePrepare creates
cat <<'EOF' | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: deny-prepare-test
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
    - apiGroups: ["operator.solo.hiero.org"]
      apiVersions: ["v1alpha1"]
      operations: ["CREATE"]
      resources: ["networkupgradeprepares"]
  validations:
  - expression: "false"
    message: "test: all prepares rejected"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: deny-prepare-test
spec:
  policyName: deny-prepare-test
  validationActions: [Deny]
EOF

# Set up valid marker and manifest
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2
```

**Trigger:**
```bash
touch "$UCDIR/current/execute_immediate.mf"
sleep 10
```

**Verify:**
```bash
# 1. Rejection logged
grep CRRejected "$UCDIR/uc.log"
# Expected: ERROR log with the rejection reason

# 2. No retry spam (5-minute cooldown)
FIRST_COUNT=$(grep -c CRRejected "$UCDIR/uc.log")
sleep 12
SECOND_COUNT=$(grep -c CRRejected "$UCDIR/uc.log")
echo "Rejections: $FIRST_COUNT → $SECOND_COUNT"
# Expected: count stays the same during cooldown

# 3. Remove the admission policy
kubectl delete validatingadmissionpolicybinding deny-prepare-test 2>/dev/null
kubectl delete validatingadmissionpolicy deny-prepare-test 2>/dev/null

# 4. Wait past cooldown (or restart UC for immediate retry)
kill $UC_PID 2>/dev/null; wait $UC_PID 2>/dev/null
rm "$UCDIR/current/execute_immediate.mf"
touch "$UCDIR/current/execute_immediate.mf"

./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 10

grep CRDelivered "$UCDIR/uc.log"
# Expected: CR delivered successfully

kill $UC_PID 2>/dev/null
```

---

### F11. UC crash between persist and create (crash window)

**Component:** CRStore / recovery
**Recovery:** self-heal

**Prepare:**
```bash
# Set up valid marker and manifest, run UC, let it create the CR
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML

./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 2
touch "$UCDIR/current/execute_immediate.mf"
sleep 10

# Stop the UC and clean the cluster
kill $UC_PID 2>/dev/null; wait $UC_PID 2>/dev/null
kubectl delete networkupgradeprepares --all -n solo 2>/dev/null
sleep 2

# Simulate a crash after persist, before create: place a persisted CR file
# (In a real test, inject a sleep between persist and create, then kill -9)
kubectl get networkupgradeprepares -n solo 2>/dev/null
# Expected: no resources — we deleted them

# Create a mock persisted file to simulate the crash window
cat > "$UCDIR/crs/mock-crash.prepare.yaml" <<'YAML'
apiVersion: operator.solo.hiero.org/v1alpha1
kind: NetworkUpgradePrepare
metadata:
  name: mock-crash-prepare
  namespace: solo
spec:
  orbitRef: my-orbit
  operationId: mock-crash-op
  images:
    consensusNode:
      version: "0.75.0"
YAML
```

**Trigger:**
```bash
# Restart the UC — recovery should find and re-send the persisted CR
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 10
```

**Verify:**
```bash
# 1. Recovery logged
grep -E "RecoverPersistedCRs|CRDelivered" "$UCDIR/uc.log"
# Expected: recovery picks up the persisted file and re-sends

# 2. CR exists in the cluster
kubectl get networkupgradeprepares -n solo
# Expected: the CR exists

# 3. Persisted file cleaned up after successful create
ls "$UCDIR/crs"/*.yaml 2>/dev/null
# Expected: no .yaml files (cleaned up after delivery)

kill $UC_PID 2>/dev/null
```

---

### F12. UC crash after create but before persist-file delete

**Component:** CRStore / recovery
**Recovery:** self-heal

**Prepare:**
```bash
# Create a CR in the cluster
cat <<'EOF' | kubectl apply -f -
apiVersion: operator.solo.hiero.org/v1alpha1
kind: NetworkUpgradePrepare
metadata:
  name: stale-test-prepare
  namespace: solo
spec:
  orbitRef: my-orbit
  operationId: stale-test-op
  images:
    consensusNode:
      version: "0.75.0"
EOF

# Simulate a stale persisted file (crash after create, before cleanup)
cat <<'YAML' > "$UCDIR/crs/stale-test-op.prepare.yaml"
apiVersion: operator.solo.hiero.org/v1alpha1
kind: NetworkUpgradePrepare
metadata:
  name: stale-test-prepare
  namespace: solo
spec:
  orbitRef: my-orbit
  operationId: stale-test-op
  images:
    consensusNode:
      version: "0.75.0"
YAML
```

**Trigger:**
```bash
# Start the UC — recovery should find the CR already in the cluster
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 10
```

**Verify:**
```bash
# 1. Recovery finds CR already exists in cluster
grep -i "stale\|already" "$UCDIR/uc.log" | head -5
# Expected: log indicating the disk copy is stale (CR already in cluster)

# 2. Stale file deleted — no duplicate CR created
ls "$UCDIR/crs"/*.yaml 2>/dev/null
# Expected: no .yaml files

# 3. Only one CR in cluster (no duplicate)
kubectl get networkupgradeprepares -n solo -o name | wc -l
# Expected: 1 (not 2)

kill $UC_PID 2>/dev/null
kubectl delete networkupgradeprepares stale-test-prepare -n solo 2>/dev/null
```

---

### F13. Watch disconnect in UpgradeOutcomeMonitor

**Component:** UpgradeOutcomeMonitor
**Recovery:** self-heal

**Prepare:**
```bash
# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 3

# Confirm the upgrade-outcome-monitor is running
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq '."upgrade-outcome-monitor"'
# Expected: shows running state
```

**Trigger:**
```bash
# Simulate a watch disconnect by restarting the API server
# (On kind: docker restart <kind-control-plane-container>)
CONTAINER=$(docker ps --filter name=kind-control-plane -q)
docker restart "$CONTAINER"
sleep 15
```

**Verify:**
```bash
# 1. Monitor reconnects (List+Watch loop with backoff)
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq '."upgrade-outcome-monitor"'
# Expected: state returns to "running" after reconnect

# 2. No crash — UC still alive
kill -0 $UC_PID 2>/dev/null && echo "OK: UC running" || echo "FAIL: UC crashed"

kill $UC_PID 2>/dev/null
```

---

### F14. Metrics server port conflict

**Component:** Metrics server
**Recovery:** manual intervention

**Prepare:**
```bash
# Occupy port 9990
python3 -c "
import http.server
http.server.HTTPServer(('', 9990), http.server.SimpleHTTPRequestHandler).serve_forever()
" &
BLOCKER_PID=$!
sleep 1
```

**Trigger:**
```bash
# Start the UC — metrics server will fail to bind
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1
# NOTE: no & — this should exit quickly
```

**Verify:**
```bash
# 1. UC exited with an error
echo "Exit code: $?"
# Expected: non-zero

# 2. MetricsServerFailed reason in logs
grep MetricsServerFailed "$UCDIR/uc.log"
# Expected: error about address already in use

# 3. Manual fix: kill the blocker and restart
kill $BLOCKER_PID 2>/dev/null
sleep 1

./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 3

# 4. UC starts successfully
kill -0 $UC_PID 2>/dev/null && echo "OK: UC running" || echo "FAIL: UC not running"
curl -s http://localhost:9990/metrics | head -1
# Expected: metrics response

kill $UC_PID 2>/dev/null
```

---

### F15. CATASTROPHIC_FAILURE fast-fails Execute

**Component:** NetworkUpgradeExecute reconciler
**Recovery:** manual intervention

**Prepare:**
```bash
# Start the operator
task dev &
sleep 10

# Get a real pod name
POD_NAME=$(kubectl get pods -n solo -l app.kubernetes.io/part-of=consensus-node \
  -o jsonpath='{.items[0].metadata.name}')

# Serve STARTING_UP (code 1)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 1.0
EOF

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name "$POD_NAME" \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6

# Drive an Execute CR to the post-upgrade waiting phase
# (Follow testing-guide.md §4 to create Prepare → Freeze → Execute)
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML
touch "$UCDIR/current/execute_immediate.mf"
sleep 30
```

**Trigger:**
```bash
# Switch to CATASTROPHIC_FAILURE (code 11)
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 11.0
EOF
sleep 15
```

**Verify:**
```bash
# 1. Execute CR fast-failed (does not wait for NodeActiveTimeout)
kubectl get networkupgradeexecutes -n solo \
  -o jsonpath='{.items[0].status.phase}'
# Expected: Failed

# 2. NodesUpgraded condition is False
kubectl get networkupgradeexecutes -n solo -o yaml | grep -A3 NodesUpgraded
# Expected: status: "False"

# 3. Warning event emitted
kubectl get events -n solo --field-selector reason=UpgradeFailed
# Expected: warning event about CATASTROPHIC_FAILURE

# 4. Capsule phase is Failed
kubectl get consensuscapsule -n solo \
  -o jsonpath='{.items[0].status.phase}'
# Expected: Failed

kill $UC_PID 2>/dev/null
```

---

### F16. NodeActiveTimeout exceeded (CN never reaches ACTIVE)

**Component:** NetworkUpgradeExecute reconciler
**Recovery:** self-heal if CN recovers

**Prepare:**
```bash
# Start the operator
task dev &
sleep 10

# Get a real pod name
POD_NAME=$(kubectl get pods -n solo -l app.kubernetes.io/part-of=consensus-node \
  -o jsonpath='{.items[0].metadata.name}')

# Serve STARTING_UP (code 1) — CN never reaches ACTIVE
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 1.0
EOF

# Start the UC
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name "$POD_NAME" \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6

# Drive the Execute CR to post-upgrade waiting phase
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML
touch "$UCDIR/current/execute_immediate.mf"
```

**Trigger:**
```bash
# Wait past NodeActiveTimeout (default 10 minutes)
# For testing, you can lower the timeout by setting --node-active-timeout 1m
sleep 660
```

**Verify:**
```bash
# 1. NodeActiveDeadlineExceeded condition set
kubectl get networkupgradeexecutes -n solo -o yaml | grep -A5 NodeActiveDeadlineExceeded
# Expected: status: "True", message about elapsed time

# 2. Warning events repeated
kubectl get events -n solo --field-selector reason=NodeActiveDeadlineExceeded
# Expected: warning events

# 3. Execute CR is NOT Failed — still waiting (non-terminal)
kubectl get networkupgradeexecutes -n solo \
  -o jsonpath='{.items[0].status.phase}'
# Expected: NOT "Failed" — still in waiting phase

# 4. Self-heal: CN reaches ACTIVE
cat > /tmp/fake-metrics/metrics <<'EOF'
# TYPE platform_PlatformStatus gauge
platform_PlatformStatus 2.0
EOF
sleep 15

kubectl get networkupgradeexecutes -n solo \
  -o jsonpath='{.items[0].status.phase}'
# Expected: Succeeded

kill $UC_PID 2>/dev/null
```

---

### F17. ConsensusConfig CR never becomes Valid (cluster-only)

**Component:** ProvisionerProxy
**Recovery:** depends on failure mode

**Prepare:**
```bash
# Start the operator
task dev &
sleep 10

# Get a real pod name
POD_NAME=$(kubectl get pods -n solo -l app.kubernetes.io/part-of=consensus-node \
  -o jsonpath='{.items[0].metadata.name}')

# Start the UC in cluster-only mode
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name "$POD_NAME" \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 6

# Set up upgrade package with config files
mkdir -p "$UCDIR/current/data/config"
echo "hedera.profiles.active=TEST" > "$UCDIR/current/data/config/application.properties"

# Drive Execute CR to ReadyForProvisionerDaemon phase
mkdir -p "$UCDIR/current/manifests"
cat > "$UCDIR/current/manifests/consensus-node-components.yaml" <<'YAML'
schemaVersion: 1
images:
  consensusNode:
    version: "0.75.0"
    deterministic:
      supported: true
      layerHashes:
        linux/amd64:
          - "sha256:abc123"
    registries:
      - image: "gcr.io/hedera-registry/consensus-node:0.75.0"
YAML
touch "$UCDIR/current/execute_immediate.mf"
sleep 30
```

**Trigger:**
```bash
# The ProvisionerProxy creates ConsensusConfig CRs
# If the operator sets Valid=False, the provisioner reports failure
# Monitor the logs for the outcome
sleep 30
```

**Verify (Valid=False — terminal):**
```bash
# 1. Config CR failure logged
grep ConfigCRFailed "$UCDIR/uc.log"
# Expected: ERROR log with details about why config is invalid

# 2. Manual fix: fix the config, delete the CR, recreate the marker
kubectl delete consensusconfigs --all -n solo 2>/dev/null
rm "$UCDIR/current/execute_immediate.mf"
# Fix the config file content as needed, then:
touch "$UCDIR/current/execute_immediate.mf"
sleep 30
```

**Verify (timeout — config CR never reconciled):**
```bash
# 1. Timeout logged after --config-cr-deploy-timeout (default 20 min)
grep ConfigCRWaitTimeout "$UCDIR/uc.log"
# Expected: timeout warning

kill $UC_PID 2>/dev/null
```

---

### F18. Monitor panic recovery

**Component:** All monitors (daemonkit supervisor)
**Recovery:** self-heal

**Prepare:**
```bash
# Start the UC normally
./bin/uc \
  --mode cluster-only --node-id 0 --orbit my-orbit --namespace solo \
  --pod-name test-pod \
  --kubeconfig "$HOME/.kube/config" \
  --upgrade-path "$UCDIR/current" --previous-upgrade-path "$UCDIR/previous" \
  --events-path "$UCDIR/events" --cr-persist-path "$UCDIR/crs" \
  --backup-path "$UCDIR/backup" --socket-path "$UCDIR/uc.sock" \
  --consensus-log-file "$UCDIR/logs/swirlds.log" \
  --metrics-url "http://localhost:9999/metrics" \
  --platform-poll-interval 5s --poll-interval 5s --log-level debug \
  > "$UCDIR/uc.log" 2>&1 &
UC_PID=$!
sleep 3

# Confirm all monitors are running
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq .
```

**Trigger:**
```bash
# Best tested via unit tests:
go test ./internal/uc/ -run TestPanic -v

# For manual observation: add a temporary panic to a monitor's Run method,
# rebuild, and run. Example (DO NOT commit):
#   func (m *PlatformStatusMonitor) Run(ctx context.Context) error {
#       panic("test panic")
#   }
# Then: task build:binary && repeat the start command above
```

**Verify:**
```bash
# 1. If testing via code modification: UC process stays alive
kill -0 $UC_PID 2>/dev/null && echo "OK: UC running" || echo "FAIL: UC crashed"

# 2. Supervisor restarts the panicking monitor (5s initial, doubling to 5min cap)
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | jq .
# Expected: the monitor shows transient state changes but returns to running

# 3. After 5 consecutive panics, degradation alert logged
grep MonitorDegraded "$UCDIR/uc.log"
# Expected: degradation alert at crash #5

# 4. Other monitors continue running
curl -s --unix-socket "$UCDIR/uc.sock" http://uc/status | \
  jq 'to_entries[] | select(.key != "platform-status-monitor") | {(.key): .value}'
# Expected: all other monitors show running state

kill $UC_PID 2>/dev/null
```
