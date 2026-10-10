# ⚠️ ANSWER KEY: spoilers. Finish your own review of the practice MR first.

Practice MR: **"Check links concurrently with a worker pool"** (`concurrent-link-checks` → `main`).

Line numbers refer to the files on the `concurrent-link-checks` branch.

## How the bugs hide (what a good reviewer does)

| Command | Result on the MR branch |
|---|---|
| `go test ./...` (what CI runs) | ✅ passes, 20 out of 20 runs |
| `go vet ./...` | ❌ `checker.go:170: loop variable i captured by func literal` |
| `go test -race ./...` | ❌ 8 data race warnings across 5 tests |
| `./bin/linkcheck -concurrency 0 testdata/sample.md` | prints `0 links checked, 0 broken` and **exits 0**: every link passes silently |
| Fail-fast against 50 broken links | error text `broken link …: <nil>`, and **12 goroutines leaked** |

Finding these by *running* the code, not only reading it, is what separates a strong review.

---

## Blocking issues (most severe first)

### 1. Goroutine leak and blocked sends in fail-fast mode (`checker.go:174`, `:184`, `:203`)
When fail-fast triggers, `CheckAll` returns on the first `errs` receive. After that:
- other workers block forever on `results <- res` or `errs <- …`, since nobody receives anymore;
- the feeder goroutine blocks forever on `jobs <- url`;
- the closer goroutine blocks forever on `wg.Wait()`.

Nothing cancels the in-flight HTTP requests either. In a long-running process (or a library
caller) that's a memory, goroutine and connection leak.

**Suggested comment**
> **issue (blocking):** When fail fast triggers we return on the first `errs` receive, but the
> other workers keep running and then block forever on `results <- res` / `errs <- …`, and the
> feeder blocks on `jobs <- url`. I measured 12 goroutines left behind after a fail-fast run with
> 5 workers. What do you think about deriving `ctx, cancel := context.WithCancel(ctx)`, calling
> `cancel()` on the first failure, and having the feeder `select` on `ctx.Done()`? Then
> `CheckAll` can `wg.Wait()` before returning, so it never outlives its caller. `errgroup`
> gives us this out of the box, if we're happy to add `golang.org/x/sync`.

### 2. Data race on `checked` / `broken` (`checker.go:145`, `:147`; `Stats()` at `:209`)
`Check` now runs on many goroutines, and `c.checked++` is an unsynchronized read-modify-write.
`go test -race` fails. Counts can be lost, so `main` could print "0 broken" while links are
broken. Also, `Stats` accumulates across calls when a `Checker` is reused, and it's redundant
with `report.Summary`.

> **issue (blocking):** `c.checked++` / `c.broken++` race between workers (`go test -race`
> reports it in `TestStats`). Since `report.Write` already computes a `Summary` from the results,
> could we drop `Stats()` and the counters? That removes the shared state instead of locking it.

### 3. The documented ordering contract is broken, and the test was weakened to hide it
- The `CheckAll` doc comment (`:153`) still promises "in the same order as urls", but results are
  appended in completion order.
- The commit "Fix flaky TestCheckAll_PreservesOrder" sorts both slices (`checker_test.go:117-119`),
  so a test named *PreservesOrder* no longer tests order. **Weakening a test to make it pass is a
  big red flag; say so kindly.**
- `main.go:85` now sorts by URL, which changes CLI output from document order to alphabetical
  order. That's a user-visible change the description says didn't happen ("Results are still
  reported in a stable order").

> **issue (blocking):** The test change in ba57cbe makes `TestCheckAll_PreservesOrder` pass by
> sorting, but the doc comment and callers rely on input order, and the CLI output now
> changes from document order to alphabetical. Could we restore the original assertions and
> have workers write to `results[i]` in a pre-sized slice? Each goroutine owns its own index, so
> no lock is needed, and order is preserved for free.

### 4. `concurrency <= 0` checks nothing and reports success (`checker.go:62`)
`WithConcurrency(0)` (or `-concurrency 0`, or a negative value) starts zero workers.
`wg.Wait()` returns immediately and `results` is closed. `CheckAll` returns `nil, nil`, the
feeder goroutine leaks, and **the CLI exits 0, so the pipeline goes green with broken links.**

> **issue (blocking):** With `-concurrency 0` nothing is checked and we exit 0
> (`0 links checked, 0 broken`). Should we clamp to 1 in `New` or reject it with an error?
> I lean towards rejecting it in `run()` with a usage error plus clamping in the library.

### 5. Loop variable captured by the goroutine (`checker.go:170`)
`go.mod` says `go 1.21`, so `i` is shared across iterations. That's a data race, and workers log
the wrong ID. `go vet` catches it. It's cosmetic today (it's only logged), but it's the kind of
thing that becomes a real bug later.

> **issue:** `go.mod` targets 1.21, so `i` is shared between iterations here (race, and wrong
> worker IDs). `go vet ./...` flags it. Pass it as an argument, `go func(worker int) {…}(i)`, or
> bump the `go` directive to 1.22+. Separately, could we add `go vet` and `-race` to CI so this is
> caught automatically?

### 6. Retry ignores `ctx` and sleeps after the last attempt (`checker.go:130-135`)
- `time.Sleep` isn't interruptible. With retries and backoff, Ctrl-C or a job timeout waits for
  the backoff to finish.
- It also sleeps after the final attempt (on the last loop iteration), which is wasted time.
- Network errors (`err != nil`, `status == 0`) are never retried, even though the description
  says "transient failures".
- `Retry-After` on `429` is ignored, and there's no jitter or cap. Those can be non-blocking
  follow-ups.
- `WithMaxRetries(-1)` means the loop never runs, so the result is `status 0, err nil`, reported
  as `FAIL 0`.

> **issue (blocking):** `time.Sleep` here doesn't respect `ctx`, so cancelling (SIGTERM from the
> runner) waits out the backoff. What about a small helper that `select`s on `ctx.Done()` and a
> timer? Also, we sleep after the final attempt. Breaking when `attempt == c.maxRetries` avoids that.

### 7. The fail-fast error loses information (`checker.go:174`)
`res.Err` is **nil** for HTTP failures like 404, so the message is `broken link X: <nil>`. It
uses `%v`, so callers can't `errors.Is`/`errors.As`, and the status code is dropped.

> **issue:** For a 404 `res.Err` is nil, so this prints `broken link …: <nil>`. Could we include
> the status code and use `%w`, or a small `BrokenLinkError` type with `Unwrap()`, so callers can
> inspect it?

### 8. Test problems
| Where | Problem | Fix |
|---|---|---|
| `checker_test.go:173` | `t.Fatalf` inside the httptest handler runs on a server goroutine. `FailNow` must be called from the test goroutine. | `t.Errorf`, or record and assert after |
| `checker_test.go:194` | Wall-clock assertion `elapsed > 300ms` is flaky on a loaded CI runner, and doesn't prove the concurrency limit | Track max in-flight requests with an atomic and assert `1 < max ≤ limit` |
| `checker_test.go:152` | `calls++` shared between the handler and the test without sync (race) | `atomic.Int32` |
| `TestCheck_RetriesTransientErrors` | Sleeps for real (100 + 200 ms) | Make backoff injectable (field or option) |
| `checker_test.go:140` | `results, _ :=` ignores the error, and there's no length check. If `results` were empty, the loop asserts nothing (vacuous pass). | Check the error and `len(results)` |
| `checker_test.go:211` | `TestCheckAll_FailFast` only checks `err != nil`. It doesn't check that we stopped early, the message, or for leaks. | Count requests, use `errors.As`, use goleak |
| `TestStats` | Racy; tests an API we should remove | Delete it, or make it atomic |
| Missing | concurrency 0/negative, empty input, more workers than URLs, retry plus cancellation, cancellation mid-run | Add table cases |
| CI | `.gitlab-ci.yml` runs neither `-race` nor `go vet`, which is why all of this was green | Add both |

---

## Non-blocking suggestions and nits
- **suggestion (non-blocking):** Split the MR. Retries and flags could be a separate MR from the
  worker pool (about 270 changed lines and 4 features in one MR).
- **suggestion (non-blocking):** `errgroup` with `SetLimit` would replace about 40 lines of channel
  plumbing. It's a new dependency, though, so it's the maintainers' call.
- **question:** Should a `CheckAll` error in `main` change the exit code explicitly? Today it is
  printed, and the exit code relies on the racy `Stats()`.
- **nitpick:** `WithLogger(nil)` would panic on the first `Debug`. Default it in `New`.
- **thought:** Hitting many links on the same host with 10 workers may trigger rate limits
  (`429`). A per-host limit would be a good follow-up issue.
- **praise:** Clean functional options, good use of `NewRequestWithContext`, body draining is kept,
  and the README was updated with the new flags.

---

## Suggested summary comment
> Thanks @sam.rivera, cutting the job from 15 minutes to under 2 is a great win and the options
> API reads nicely. 🙌 I found a few concurrency issues that `go test -race` and `go vet` surface
> (our CI runs neither, so they slipped through):
> - **blocking:** data race on the stats counters; goroutine leak when fail-fast triggers;
>   the ordering contract (and its test) was changed; `-concurrency 0` silently passes;
>   retry backoff ignores cancellation.
> - **non-blocking:** fail-fast error message, test flakiness (timing assertions, `t.Fatalf` in a
>   handler), and a suggestion to split retries into a separate MR.
>
> Most of the blocking items share one fix: a cancellable context, plus workers writing to
> `results[i]`, plus `wg.Wait()` before returning. I sketched it in the thread on `CheckAll`.
> Happy to pair on it!

---

## Reference fix
`solution.patch` (in this folder) is a verified fix:
- passes `go vet`;
- passes `go test -race` on 10 out of 10 runs;
- gives 88% coverage in `checker`.

Apply it to compare with your own:
```shell
cd linkcheck && git checkout -b my-solution concurrent-link-checks
git apply ../interview-prep/solution.patch   # adjust the path as needed
go test -race -count=5 ./...
```
The core of it:
```go
func (c *Checker) CheckAll(ctx context.Context, urls []string) ([]Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Each worker writes to its own index, so results needs no locking and
	// keeps the order of urls.
	results := make([]Result, len(urls))
	jobs := make(chan int)

	var (
		wg       sync.WaitGroup
		failOnce sync.Once
		failErr  error
	)
	for w := 0; w < min(c.concurrency, len(urls)); w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			c.logger.Debug("worker started", "worker", worker)
			for i := range jobs {
				results[i] = c.Check(ctx, urls[i])
				if c.failFast && !results[i].OK() {
					failOnce.Do(func() {
						failErr = &BrokenLinkError{Result: results[i]}
						cancel()
					})
				}
			}
		}(w)
	}

feed:
	for i := range urls {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	// Links that were never handed to a worker report why they were skipped.
	for i := range results {
		if results[i].URL == "" {
			results[i] = Result{URL: urls[i], Err: ctx.Err()}
		}
	}

	return results, failErr
}
```
Things to be ready to defend in the live session:
- **Why `results[i]` needs no mutex:** each index is written by exactly one goroutine, and
  `wg.Wait()` happens-before the read.
- **Why `sync.Once`:** several workers can fail at the same time. Only the first error is kept,
  and `cancel` is called once (calling `cancel` twice is safe anyway).
- **Why the feeder `select`s on `ctx.Done()`:** after cancellation, workers may all be busy or
  gone, so a plain send could block forever.
- **Why `min(c.concurrency, len(urls))`:** don't start idle workers. Clamping to ≥1 happens in `New`.
- **Alternative:** `errgroup.WithContext` plus `SetLimit` gives the same result in fewer lines,
  with a new dependency as the trade-off.

## Live-session practice drills (do these with a timer)
1. (15 min) Write a test that fails under `-race` because of the stats counters, then remove the race.
2. (20 min) Write a test proving fail-fast stops early (request counter) and leaves no
   goroutines behind, then fix `CheckAll`.
3. (10 min) Restore `TestCheckAll_PreservesOrder` and make it pass with index-addressed results.
4. (10 min) Make retry backoff respect `ctx`, with a test that uses a 1h backoff and a 50ms timeout.
5. (10 min) Replace the timing assertion with a max-in-flight measurement.
6. Bonus: rewrite `CheckAll` with `errgroup`, and explain the trade-offs out loud.
