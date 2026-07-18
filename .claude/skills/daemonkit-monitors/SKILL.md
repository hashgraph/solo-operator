---
name: daemonkit-monitors
description: Write or audit daemons and monitors built on github.com/automa-saga/daemonkit. Use when creating a MonitorRunner or daemon, wiring SupervisedMonitor, or scanning existing daemonkit code for anti-patterns (bare `go func()` in a monitor, monitors launched without SupervisedMonitor, unguarded ConnectivityError data races, raw errgroup instead of daemonkit.Group, Run returning non-nil on ctx cancel) and applying the fixes.
allowed-tools: Read, Grep, Glob, Bash, Edit, Write
---

# daemonkit monitors & daemons

[daemonkit](https://github.com/automa-saga/daemonkit) is a reusable kernel for
long-running daemons: a supervised-monitor restart loop, panic-safe goroutine
helpers, a Unix-socket control plane, and sd_notify integration. This skill
helps you **author** correct daemons/monitors and **audit** existing ones for the
contract violations that cause daemon crashes and data races.

## The non-negotiable rules

1. **A monitor implements `MonitorRunner`** (`Run(ctx) error`, `Name() string`).
   `Run` returns **`nil` on ctx cancellation** (clean shutdown) and non-nil only
   on unexpected failure (triggers a supervised restart). It must be safe to call
   again.
2. **Run every monitor under `SupervisedMonitor`** — never `go m.Run(ctx)` raw.
   The supervisor gives you back-off, degradation alerts, heartbeats, and a panic
   barrier on the Run goroutine.
3. **Never spawn a bare `go func()` in monitor code.** `recover` is per-goroutine,
   so an unguarded panic in a spawned goroutine crashes the whole daemon. Use:
   - `daemonkit.Go(ctx, log, name, fn)` — best-effort work whose failure must NOT
     restart the monitor (panic is recovered + logged, goroutine stops).
   - `daemonkit.Group` / `daemonkit.WithContext` — work whose failure SHOULD
     restart the monitor (a child panic/error surfaces from `Wait` as the Run
     error). Both compose transitively at any nesting depth.
4. **`ConnectivityError()` must be concurrency-safe** — the status/heartbeat
   goroutine reads it while `Run` writes. Back it with
   `atomic.Pointer[daemonkit.StatusError]` or a mutex, never a plain field.

Full details: [references/contract.md](references/contract.md).

## Mode A — author a daemon or monitor

1. Read [references/contract.md](references/contract.md) for the interfaces and
   [references/templates.md](references/templates.md) for copy-paste skeletons.
2. Scaffold the monitor from the template; implement `Run`/`Name`, and only the
   optional interfaces you actually need (`ConnectivityMonitor`,
   `ProbableMonitor`).
3. Wire it in `main` via `SupervisedMonitor` per the daemon template.
4. Verify: `go build ./... && go vet ./...`.

## Mode B — audit existing code and fix

Follow [references/audit-checklist.md](references/audit-checklist.md). Workflow:

1. **Locate** daemonkit code — find importers and MonitorRunner implementations:
   ```bash
   grep -rn "automa-saga/daemonkit" --include='*.go' .
   grep -rnE "func .*Run\(ctx context\.Context\) error" --include='*.go' .
   ```
2. **Scan** — run each detection grep from the checklist.
3. **Report** — present a findings table: **severity | file:line | rule | fix**.
4. **Fix** — apply the checklist's fix for each *confirmed* finding.
5. **Verify** — `go build ./... && go vet ./... && go test -race ./...`, then show
   `git diff` so the user reviews before committing.

Always report findings **before** applying fixes: some detections are starting
points that need a human judgment call (e.g. a bare goroutine that is genuinely
out of scope) — the checklist marks which.
