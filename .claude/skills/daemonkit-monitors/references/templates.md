<!-- SPDX-License-Identifier: Apache-2.0 -->

# daemonkit templates

Copy-paste starting points. Replace `MyMonitor` and package paths. Snippets show
the daemonkit-relevant shape; elided bodies are marked `/* … */`.

## Monitor

```go
package monitors

import (
    "context"
    "log/slog"
    "sync/atomic"
    "time"

    "github.com/automa-saga/daemonkit"
)

type MyMonitor struct {
    log     *slog.Logger
    connErr atomic.Pointer[daemonkit.StatusError] // only if you implement ConnectivityMonitor
}

func NewMyMonitor(log *slog.Logger) *MyMonitor { return &MyMonitor{log: log} }

func (m *MyMonitor) Name() string { return "my-monitor" }

// Run blocks until ctx is cancelled. Return nil on cancellation; non-nil only on
// unexpected failure (triggers a supervised restart).
func (m *MyMonitor) Run(ctx context.Context) error {
    t := time.NewTicker(5 * time.Second)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil // clean shutdown — NOT ctx.Err()
        case <-t.C:
            if err := m.pollOnce(ctx); err != nil {
                // record for /status + heartbeats, then keep retrying
                m.connErr.Store(&daemonkit.StatusError{
                    Reason:  "PollFailed",
                    Message: err.Error(),
                    Since:   time.Now().Format(time.RFC3339),
                })
                continue
            }
            m.connErr.Store(nil) // recovered
        }
    }
}

func (m *MyMonitor) pollOnce(ctx context.Context) error { /* … */ return nil }

// ConnectivityMonitor (optional): concurrency-safe read.
func (m *MyMonitor) ConnectivityError() *daemonkit.StatusError { return m.connErr.Load() }
```

## Monitor with concurrent sub-loops (daemonkit.Group)

```go
func (m *MyMonitor) Run(ctx context.Context) error {
    g, ctx := daemonkit.WithContext(ctx)
    g.Go("watch", m.watchLoop) // func(ctx context.Context) error
    g.Go("prune", m.pruneLoop)
    return g.Wait() // a child panic/error becomes the Run error → supervised restart
}
```

## Best-effort background goroutine (daemonkit.Go)

```go
// inside Run — a panic here is recovered + logged; it does NOT restart the monitor
daemonkit.Go(ctx, m.log, "metrics", func(ctx context.Context) {
    <-ctx.Done()
})
```

## Daemon main

```go
// imports: context, os, os/signal, syscall, log/slog, time,
// github.com/automa-saga/daemonkit

func run(ctx context.Context, logger *slog.Logger) error {
    tracker := daemonkit.NewStatusTracker()
    monitors := []daemonkit.MonitorRunner{
        NewMyMonitor(logger),
        // more monitors…
    }

    // Top-level fan-out via Group (panic-safe) — never a bare go m.Run(ctx).
    g, ctx := daemonkit.WithContext(ctx)
    for _, m := range monitors {
        g.Go(m.Name(), func(ctx context.Context) error {
            daemonkit.SupervisedMonitor(ctx, m, daemonkit.SupervisorOptions{
                Tracker:           tracker,
                Logger:            logger,
                HeartbeatInterval: 30 * time.Second,
            })
            return nil // SupervisedMonitor returns only on ctx cancellation
        })
    }
    return g.Wait()
}

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
    if err := run(ctx, logger); err != nil {
        logger.Error("daemon exited", "error", err)
        os.Exit(1)
    }
}
```

## ProbableMonitor (optional)

```go
func (m *MyMonitor) RequiredProbe() daemonkit.Probe {
    return daemonkit.NewCompositeProbe("my-monitor",
        &daemonkit.TaggedProbe{
            Inner:      &daemonkit.DiskPermissionProbe{Path: m.dir, Permission: 0o555},
            Reason:     "DataDirUnreadable",
            Resolution: "Verify the data directory is mounted and readable: ls -la " + m.dir,
        },
    )
}
```
