<!-- SPDX-License-Identifier: Apache-2.0 -->

# daemonkit contract reference

## MonitorRunner (required)

```go
type MonitorRunner interface {
    Run(ctx context.Context) error
    Name() string
}
```

`Run` rules:
- **Return `nil` when `ctx` is cancelled** — clean shutdown, no restart.
- **Return non-nil only on unexpected failure** — triggers a supervised restart
  with back-off.
- **Be safe to call again** after returning an error — the supervisor re-invokes
  `Run`.
- **Do not spawn unguarded goroutines** (see *Panic safety*).

`Name()` returns a stable identifier used in logs and status.

## SupervisedMonitor (run loop)

Run each monitor under the supervisor; it never returns until `ctx` is cancelled:

```go
daemonkit.SupervisedMonitor(ctx, m, daemonkit.SupervisorOptions{
    Tracker:           tracker,          // optional *StatusTracker for /status
    Logger:            logger,           // optional *slog.Logger (nil = silent)
    HeartbeatInterval: 30 * time.Second, // optional; 0 disables heartbeats
})
```

What it provides:
- **Back-off**: 5s → doubling → 5min cap; resets after a 60s stable run.
- **Degradation**: emits `MonitorDegraded` every 5 consecutive crashes.
- **Heartbeats**: periodic `MonitorHeartbeat` (carries health for
  ConnectivityMonitors) so observers can alert on their *absence*.
- **Panic barrier**: a panic on the Run goroutine becomes a crash (back-off +
  restart), not a process exit.

## Panic safety (the #1 source of daemon crashes)

`recover` is per-goroutine. `SupervisedMonitor` only guards the Run goroutine.
Any goroutine a monitor spawns itself owns its recovery. **Never write a bare
`go func(){…}` in monitor code.** Use one of:

### daemonkit.Go — best-effort, fire-and-forget

```go
daemonkit.Go(ctx, logger, "reaper", func(ctx context.Context) {
    // a panic here is recovered + logged (reason=GoroutinePanic); the goroutine stops
})
```

Use when the goroutine's failure must **not** restart the monitor (heartbeats,
metrics). The panic is **swallowed** — the monitor keeps running, so do not use
`Go` for work the monitor depends on.

### daemonkit.Group — failure should restart the monitor

```go
func (m *MyMonitor) Run(ctx context.Context) error {
    g, ctx := daemonkit.WithContext(ctx)
    g.Go("watch", m.watchLoop) // func(context.Context) error
    g.Go("prune", m.pruneLoop)
    return g.Wait()            // child panic/error → Run error → supervised restart
}
```

A child panic becomes the `Wait` error (the first failure cancels the group
context so siblings exit). Both helpers compose **transitively** — a `Go`/
`Group.Go` nested inside another is guarded at every level.

### What neither can catch

- Panics in goroutines spawned by **third-party libraries** you call.
- **Fatal runtime errors** (`concurrent map writes`, stack overflow, OOM,
  all-goroutine deadlock, cgo SIGSEGV) — these are `runtime.throw`, not panics.

Only process isolation contains those.

## ConnectivityMonitor (optional)

For monitors that track their last connectivity error so /status and heartbeats
surface it while `Run` is alive and retrying:

```go
type ConnectivityMonitor interface {
    MonitorRunner
    ConnectivityError() *daemonkit.StatusError
}
```

**Concurrency requirement**: the status/heartbeat goroutine calls
`ConnectivityError()` while `Run` writes the field. Guard it:

```go
type MyMonitor struct {
    connErr atomic.Pointer[daemonkit.StatusError] // NOT a plain field
}

func (m *MyMonitor) ConnectivityError() *daemonkit.StatusError { return m.connErr.Load() }
// writer inside Run: m.connErr.Store(&daemonkit.StatusError{…}) / m.connErr.Store(nil)
```

## ProbableMonitor (optional)

For monitors with external prerequisites (a mounted dir, a writable path) that
must be verified before `Run` starts:

```go
type ProbableMonitor interface {
    MonitorRunner
    RequiredProbe() daemonkit.Probe
}
```

Build the probe from the kit's leaf probes — `DiskPermissionProbe{Path,
Permission}`, `DiskWriteTestProbe{Dir}` — wrapped in `TaggedProbe{Inner, Reason,
Resolution}` for operator-facing diagnostics and combined with
`daemonkit.NewCompositeProbe(name, probes...)`. The component collects these via
`daemonkit.BuildComponentProbe`.

## StatusError / StatusTracker

`StatusError{Reason, Message, Resolution, Since}` is the operator-facing error
descriptor used in /status and heartbeats. `NewStatusTracker()` holds per-monitor
state (`running` / `degraded` / `backoff:<dur>` / `stopped`); pass it to
`SupervisorOptions.Tracker` and expose `Snapshot()` from your control plane.
