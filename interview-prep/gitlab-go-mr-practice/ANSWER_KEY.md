# Answer key (spoilers)

> Submit your own review before you read this.

Line numbers refer to the files on `feature/concurrent-checks`.

## How to find most of these in 2 minutes

```shell
go test -race -count=1 ./...          # 6 data race reports, 2 failing tests
go test -count=20 ./...               # timing-based assertions start to flake
go vet ./...                          # clean: vet does not catch these
go run ./cmd/monitor -config endpoints.example.json   # then Ctrl+C -> panic
go run ./cmd/monitor -config missing.json -once; echo $?   # prints 0
go run ./cmd/monitor -config endpoints.example.json -concurrency 0   # hangs
```

In the interview, say that you ran these and paste the output into your
comments. Showing evidence is worth more than stating an opinion.

---

## Blocking issues

### 1. `CheckAll` no longer returns results in input order (`checker.go:56-85`)

- The doc comment still says "in the same order as eps", but results are
  appended in completion order.
- The second commit, "Fix flaky CheckAll test", **changed the test to sort
  the results** instead of fixing the code. That is the biggest red flag in the
  MR: the test was right, and it caught a regression in behaviour.
- Users see the effect: `/status` order changes from one run to the next.

```
issue (blocking): `CheckAll` now returns results in completion order, but the
doc comment (and `/status` consumers) rely on input order. The follow-up commit
sorts results in the test, which hides the regression instead of fixing it.

suggestion: preallocate `results := make([]Result, len(eps))` and have each
goroutine write `results[i]`. Each index is written by exactly one goroutine,
so the mutex can go too. Could we restore `TestCheckAll_PreservesOrder` with
endpoints of different latency so it actually exercises ordering?
```

### 2. `-concurrency 0` deadlocks and a negative value panics (`checker.go:65`)

- `make(chan struct{}, 0)` is unbuffered, so `sem <- struct{}{}` blocks forever
  and the service hangs without logging anything.
- `make(chan struct{}, -1)` panics with `makechan: size out of range`.
- `NewChecker` is exported, so any caller can pass 0. `DefaultConcurrency`
  exists but nothing applies it.
- Fix: validate the flag in `main` and default or reject `<= 0` in
  `NewChecker`. Add a test for 0 and -1 that guards against a hang with
  `select` and `time.After`.

### 3. `FirstFailure` leaks goroutines and never cancels in-flight work (`checker.go:87-111`)

- `failures` and `done` are unbuffered. Once the function returns on the first
  failure, nothing reads from them again, so every remaining goroutine blocks
  forever on `done <- ...` (or on `failures <- ...` if there is a second
  failure). That is a permanent leak per call.
- `ctx` is not cancelled after the first failure, so the remaining HTTP
  requests keep running for up to `timeout`.
- Unbounded: one goroutine per endpoint, which ignores `-concurrency`, the
  very limit this MR introduces.
- Fix: `ctx, cancel := context.WithCancel(ctx); defer cancel()`, plus either a
  buffered channel `make(chan Result, len(eps))` or a bounded worker pool that
  waits for its workers. `errgroup.WithContext` with `SetLimit` is the
  idiomatic choice when `golang.org/x/sync` is allowed.
- Test: slow endpoints that only finish after 10s, one failing endpoint.
  Assert that the call returns quickly and that `runtime.NumGoroutine()` drops
  back to the starting value. Mention `go.uber.org/goleak` and
  `testing/synctest` (Go 1.25) as better tools.

### 4. Data race on `Monitor.stats` (`monitor.go:81-93`)

- `runOnce` runs in several goroutines (see #5), and each does `m.stats.X++`
  without a lock, while `Stats()` reads the struct from the HTTP handler.
- `go test -race` reports it. Lost updates are possible even without the race
  detector.
- Fix: update `stats` under `m.mu` together with `m.last`, and take `RLock` in
  `Stats()`. `atomic.Int64` per counter also works, but then a reader can see
  `Checks` from one run and `Failures` from another. Updating under one lock
  keeps the snapshot consistent.

### 5. Overlapping runs: `go m.runOnce(ctx)` on every tick (`monitor.go:50,59`)

- If a run takes longer than `interval` (the exact problem the MR description
  describes), runs pile up. Goroutines and HTTP requests grow without bound.
- An older, slower run can finish after a newer one and **overwrite
  `m.last` with stale results**.
- `Run` returns on cancel while runs are still in flight, so they write state
  after shutdown and the caller cannot wait for them.
- Fix: call `runOnce` synchronously. `time.Ticker` already drops ticks when the
  receiver is slow. Concurrency now lives in `CheckAll`, which was the point of
  the MR. If skipping a run is really wanted, use a "running" flag (an
  `atomic.Bool` with CompareAndSwap) rather than spawning goroutines.

### 6. `Stop()` panics when called twice, and `main` always calls it twice (`monitor.go:65`, `main.go:65,103`)

- `defer m.Stop()` plus the explicit `m.Stop()` gives
  `panic: close of closed channel` on every graceful shutdown (SIGINT or
  SIGTERM). Reproduce it with Ctrl+C.
- Design question: why have `Stop` at all? `Run` already takes a `ctx`. Two
  cancellation mechanisms is one too many. Prefer removing `Stop`, or guard it
  with `sync.Once` if it must stay.
- `Run` also no longer has `defer ticker.Stop()`, which the base code had.
  Before Go 1.23 an unstopped ticker is never garbage collected, and `go.mod`
  says 1.22.

### 7. `-once` hides errors (`main.go:43-53`)

- `endpoints, _ := monitor.LoadEndpoints(...)` ignores the error. With a
  missing or invalid config the endpoint list is empty, so `FirstFailure`
  returns nil and **the process exits 0**: a CI health gate that always
  passes. This is the worst bug here from a user's point of view.
- `log.Fatalf` calls `os.Exit` and skips deferred calls (`cancel()`). It also
  bypasses the structured `slog` logger the rest of the program uses.
- For an HTTP 500, `f.Err` is nil, so the message ends in `: <nil>`. Print the
  status code instead.
- `context.WithTimeout(ctx, timeout)` reuses the per-request timeout as the
  deadline for the whole batch. With more endpoints than `-concurrency`,
  queued checks time out and are reported as unhealthy.
- Fix: return an `error` from `checkOnce`, handle it in `main`, and exit 1.

## Test issues (the interview weighs these heavily)

### 8. `TestCheckAll_RunsConcurrently` (`checker_test.go:111-140`)

- `hits++` inside the handler is a data race in the test itself, because
  handlers run concurrently. Use `atomic.Int32`.
- `elapsed > 150ms` is a timing assertion: it flakes on loaded CI runners and
  doesn't prove the limit is respected. Measure the maximum number of requests
  in flight on the server (an atomic counter with a CAS max) and assert
  `1 < max <= limit`.
- Nothing tests concurrency 0, an empty list, or ordering.

### 9. `TestFirstFailure` (`checker_test.go:142-162`)

- Only the happy path. No "all healthy, so nil" case and no empty-list case.
- Can't detect the leak or the missing cancellation (see #3).

### 10. `TestMonitor_Run` (`monitor_test.go:36-57`)

- `time.Sleep(100ms)` for synchronisation. Prefer polling with a deadline, or
  calling `runOnce` directly.
- Reads `Stats()` without waiting for `Run` (and its goroutines) to finish:
  racy, and the race detector flags it.
- Only checks `> 0`. A deterministic test of `runOnce` with one healthy and one
  broken endpoint should assert exact values (`Runs: 2, Checks: 4,
  Failures: 2`).
- Doesn't test that `Run` returns after cancel, or that nothing runs after it
  returns.

### 11. CI doesn't run with `-race` (`.gitlab-ci.yml`)

Pre-existing and not changed by this MR, but now that the MR adds
concurrency, the right follow-up is to add `go test -race ./...` to CI. Label
it `suggestion (non-blocking)` or ask for a follow-up issue.

## Non-blocking and nits (show judgement, don't overdo them)

- `NewChecker(timeout, concurrency)` changes the signature of an exported
  constructor. That is fine for an internal package, but a functional option
  or a `Config` struct would avoid breaking other callers.
- `Stats` mixes lifetime counters with shutdown-cancelled runs, so a SIGTERM
  mid-run counts every endpoint as failed. Skip recording when `ctx.Err() != nil`.
- MR scope: concurrency, the `-once` mode, `/stats` and `Stop` are four
  features in one MR. GitLab's guidelines prefer small MRs that each do one
  thing. Suggest splitting out `-once` and `/stats`.
- The MR description says "40 endpoints ~3.4s", but there is no benchmark and
  the test uses 20. Ask how it was measured.
- The checklist claims "go test passes", but it wasn't run with `-race`.

## Traps: things that look like bugs but aren't

- **Loop variable capture** (`go func() { ... ep ... }()`) is **correct**
  because `go.mod` says `go 1.22`. Since Go 1.22 each iteration has its own
  `ep`. Saying "this would have been a bug before 1.22, and it's fine here
  because go.mod is 1.22" scores points. Flagging it as a bug loses some.
- The `sem` acquire/release around `Check` does correctly bound concurrent
  **requests**. It just doesn't bound **goroutines** (one per endpoint, all
  created up front). That's acceptable for tens of endpoints. Mention it as
  non-blocking.
- `Snapshot()` copying under `RLock` is correct (it is pre-existing code).

---

## The live-coding part: what to fix, in this order

The interviewer will probably say "pick something and fix it". Choose
something that tests concurrency **and** testing:

1. **Ordering plus the concurrency limit in `CheckAll`.** First restore the
   order test, with endpoints of different latency so the fast ones finish
   first. Run it and watch it fail. Then write into `results[i]` and remove
   the mutex. Then add the max-in-flight test. About 15 minutes.
2. **The `FirstFailure` leak.** Add `WithCancel`, a buffered channel or a
   worker pool, and the leak test. About 15 minutes.
3. **Monitor races.** Make `runOnce` synchronous, lock `stats`, remove `Stop`,
   restore `defer ticker.Stop()`, and fix the test. About 10 minutes.

At each step: write or restore the test, run it and see it fail, fix the code,
run `go test -race -count=10 ./...`, then commit.

See `solution/` for one way to do it. It extracts a shared `forEach` worker
pool that both `CheckAll` and `FirstFailure` use.

## Mock interview script (rehearse your answers out loud)

1. "Walk us through what this service does, then what the MR changes."
2. "What was the first thing that caught your eye?" Lead with the reordered
   test (#1), the shutdown panic (#6) or the `-once` exit code (#7). Each is
   concrete and user-visible, and you can reproduce it.
3. "Why does the order test fail intermittently? Show me." Then: "How would you
   make the test fail every time?" (Give endpoints different delays.)
4. "Explain the leak in `FirstFailure`. How would you prove it in a test?"
5. "Mutex, atomics or channels for `Stats`, and why?"
6. "Could you use `errgroup` here? What's the difference between
   `errgroup.Group` and `errgroup.WithContext`? What does `SetLimit` do?"
7. "What happens when a run takes longer than the interval with your fix?"
   (The ticker drops ticks. No overlap, no pile-up.)
8. "Is this loop variable capture a bug?" (It's the Go 1.22 trap.)
9. "How would you get this MR merged faster?" (Split it, add `-race` to CI,
   use Conventional Comments to separate blocking from non-blocking feedback.)
10. "What if there were 100,000 endpoints?" (A worker pool instead of a
    goroutine per endpoint, streaming results, per-host limits, jittered
    scheduling, metrics.)
