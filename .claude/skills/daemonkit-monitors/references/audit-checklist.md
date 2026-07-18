<!-- SPDX-License-Identifier: Apache-2.0 -->

# daemonkit audit checklist

Run each detection from the repo root. Report findings as a table
(**severity | file:line | rule | fix**) **before** applying fixes. Severities:
**critical** = can crash the daemon or cause a data race; **warning** = wrong
behavior; **info** = robustness.

> Detections are starting points, not verdicts. Open each hit and confirm it is
> monitor/daemon code before fixing. Note any false positives in the report.

---

## 1. Bare `go func()` in monitor/daemon code — **critical**

**Detect:**
```bash
grep -rn "go func(" --include='*.go' . | grep -v _test.go
```

**Why:** `recover` is per-goroutine; `SupervisedMonitor` only guards the Run
goroutine. An unrecovered panic in a spawned goroutine crashes the **entire
daemon**.

**Fix** — replace the bare `go func()` with the helper matching intent:
```go
// best-effort (failure must NOT restart the monitor):
daemonkit.Go(ctx, logger, "worker", func(ctx context.Context) { /* … */ })

// essential (failure SHOULD restart the monitor): restructure Run around a Group
g, ctx := daemonkit.WithContext(ctx)
g.Go("worker", func(ctx context.Context) error { /* … */ })
return g.Wait()
```

**False positive:** a bare `go` in `func main()` or test setup that is not under
a monitor is out of scope (though `daemonkit.Go` is still safer).

---

## 2. Monitor launched without SupervisedMonitor — **critical**

**Detect:**
```bash
grep -rnE "go [A-Za-z0-9_.]+\.Run\(" --include='*.go' . | grep -v _test.go
grep -rn "\.Run(ctx)" --include='*.go' . | grep -vi "supervis"
```

**Why:** calling `Run` directly (or `go m.Run(ctx)`) gives no restart, no
back-off, and no panic barrier — one error or panic ends the monitor (or the
daemon) permanently.

**Fix:**
```go
g, ctx := daemonkit.WithContext(ctx)
for _, m := range monitors {
    g.Go(m.Name(), func(ctx context.Context) error {
        daemonkit.SupervisedMonitor(ctx, m, opts)
        return nil
    })
}
return g.Wait()
```

---

## 3. Unguarded ConnectivityError field — **critical (data race)**

**Detect:**
```bash
grep -rn "func.*ConnectivityError() \*.*StatusError" --include='*.go' .
```
For each hit, check whether the returned value is a plain struct field written in
`Run` and read here.

**Why:** the status/heartbeat goroutine reads `ConnectivityError()` concurrently
with `Run` writing it → data race (`go test -race` will flag it).

**Fix** — back it with an atomic (or mutex):
```go
connErr atomic.Pointer[daemonkit.StatusError]
func (m *M) ConnectivityError() *daemonkit.StatusError { return m.connErr.Load() }
// writer: m.connErr.Store(&daemonkit.StatusError{…}) / m.connErr.Store(nil)
```

**Verify:** `go test -race ./...`.

---

## 4. Raw errgroup where daemonkit.Group belongs — **critical**

**Detect:**
```bash
grep -rn "errgroup\." --include='*.go' . | grep -v _test.go
```

**Why:** `golang.org/x/sync/errgroup` does **not** recover panics — a panicking
child crashes the daemon. `daemonkit.Group` wraps each child in a recover barrier.

**Fix:** swap `errgroup.WithContext` / `eg.Go(func() error …)` for
`daemonkit.WithContext` / `g.Go(name, func(ctx) error …)`.

**False positive:** errgroup inside daemonkit itself, or a fan-out that provably
cannot panic and where a panic-as-crash is acceptable — but prefer `Group`.

---

## 5. Run returns non-nil on ctx cancellation — **warning**

**Detect:**
```bash
grep -rn "return ctx.Err()" --include='*.go' .
```
Also read each `Run` for a top-level path that returns a wrapped `ctx.Err()`.

**Why:** `SupervisedMonitor` treats any non-nil return as a crash → it logs
`MonitorCrash` and restarts with back-off on what was a clean shutdown.

**Fix** — return `nil` when the context is cancelled:
```go
case <-ctx.Done():
    return nil
```

---

## 6. Missing RequiredProbe for prerequisites — **info**

**Detect:** MonitorRunner implementations that read a mounted path / need network
at startup but do not implement `RequiredProbe()`:
```bash
grep -rn "os.Open\|os.Stat\|os.ReadFile\|os.WriteFile" --include='*.go' .
```
cross-referenced against types that implement `Run`/`Name` but not
`RequiredProbe`.

**Why:** without a probe the monitor starts before its prerequisite is ready and
crash-loops instead of reporting a clear, actionable "waiting on prerequisite"
state.

**Fix:** implement `RequiredProbe()` returning a `DiskPermissionProbe` /
`DiskWriteTestProbe` (wrapped in `TaggedProbe`) or a `NewCompositeProbe`. See
[templates.md](templates.md).

---

## After fixing

```bash
go build ./... && go vet ./... && go test -race ./...
git diff
```
Show the diff and let the user review before committing.
