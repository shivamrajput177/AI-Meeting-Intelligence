# GitLab Intermediate Backend Engineer: MR review interview prep (Go concurrency)

## 1. What this interview is

| Stage | What happens | What they're judging |
|---|---|---|
| **Async review** (48h window, aim for ~1–3h of work) | You get a link to a self-contained MR, review it, and leave comments on the MR. | Code understanding, what you notice, how you communicate in writing. |
| **Submit at least 24h before the call** | They read your comments in advance. | Reliability. If you can't make the deadline, email them **before** the 24h mark. |
| **90-min call, part 1** (~20–30 min) | You walk them through your review. | Can you explain *why* each item matters, and order items by severity? |
| **90-min call, part 2** (~50–60 min) | You pair with an engineer to fix issues you found, and write and fix tests. | Concurrency fluency, testing approach, idiomatic Go, thinking out loud, collaboration. |

The email names two areas of **particular emphasis: testing and concurrency patterns.**
Most of your preparation should go there.

How they map to GitLab's values:
- **Collaboration**: kind, specific comments, and you ask questions instead of making demands.
- **Results/Iteration**: you separate blocking from non-blocking, and you suggest follow-up issues instead of scope creep.
- **Transparency**: you say "I don't understand this part" when that's true.

---

## 2. Timeline plan

### Now, until the MR link arrives
1. **Go setup:** latest Go installed, `go test -race` works (on macOS it needs Xcode CLT), and `gopls`
   works in your editor. Check `go vet`, and optionally install
   [`staticcheck`](https://staticcheck.dev) and `golangci-lint` (GitLab Go projects use it).
2. **Do the practice MR in this folder for real.** Treat it as the actual interview:
   1. Run `./setup-gitlab-mr.sh <your-gitlab-username>`.
   2. Give yourself 2 hours and leave real comments on GitLab.
   3. Only then open `ANSWER_KEY.md`.
3. **Practice the fix session out loud.** Record yourself (or use a friend) fixing 3 issues in
   the practice MR in 45 minutes while narrating. That is the skill being tested.
4. **Learn the concurrency material in section 5 until you can write it without looking**:
   worker pool with ordered results, cancellation with `context`, the errgroup pattern,
   and a race-free counter.
5. Zoom: camera, microphone and **screen-sharing permissions** set up (macOS needs a restart after granting them).
   Set your editor font large (16–18pt) and clean up your terminal prompt.

### When the link arrives (the 48h window)
Follow the review workflow in section 3. Don't rush; they said quality over speed.
Submit the review, and if the MR's comment box allows it, add a **summary comment** at the end.

### 24h before
Re-read your own comments. For each blocking one, know:
- the **bug**,
- **how to prove it** (a test or command),
- **the fix**, and
- **the test that guards it**.

### Day of
Clone fresh, run `go test ./...` and `go test -race ./...`, and have the MR open in a browser tab.
Close Slack and notifications.

---

## 3. Review workflow (use this on the real MR)

**Step 0: understand before judging (~15 min)**
- Read the MR description, then the linked issue. What problem is this solving? What does it *claim*?
  (Claims that don't match the code are classic interview traps.)
- Read the **base code** on `main` for the touched packages first. Know the existing contracts:
  doc comments like "returns results in the same order", cancellation behavior, and error semantics.
- Look at the commit list. A commit named "fix flaky test" deserves suspicion: was the test
  fixed, or weakened?

**Step 1: run things (~15 min)**
```shell
go build ./...
go vet ./...                      # catches loop-variable capture, copied locks, printf misuse
go test ./...                     # what CI runs
go test -race -count=1 ./...      # concurrency bugs show up here
go test -race -count=20 ./...     # flushes out flaky and order-dependent tests
go test -run TestName -v ./pkg/   # zoom into one test
go test -cover ./...              # where coverage is thin
staticcheck ./...                 # optional, if installed
```
Also check the `go` directive in `go.mod`. If it's **below 1.22**, loop variables are shared
across iterations, so `go func(){ use(i) }()` inside a loop is a bug.
Check what CI runs (`.gitlab-ci.yml`). If CI doesn't use `-race` or `go vet`, that's a valuable
comment.

**Step 2: read the diff file by file (~45 min).** For every goroutine you see, ask:
1. **Who waits for it?** (WaitGroup, channel, errgroup). If nobody does, it may outlive its caller.
2. **How does it stop?** On success, on error, and on context cancellation. Can it block
   forever on a send or receive?
3. **What does it share?** Every variable touched by two goroutines needs a mutex, an atomic,
   a channel, or a distinct index per goroutine.
4. **Who closes each channel?** Only the sender closes, and exactly once. Never close from the receiver side.
5. **What happens with 0 items, 1 item, more workers than items, and concurrency ≤ 0?**
6. **What happens on the first error?** Are the other goroutines cancelled? Are errors lost or
   overwritten? Are they wrapped with `%w`?
7. **Is `ctx` passed down and honored?** Look for `time.Sleep` in place of a `select` on
   `ctx.Done()`, and for a blocking send without a `select` on `ctx.Done()`.
8. **Did the public API or a documented contract change?** Are callers and docs updated?

For tests, ask:
- Does the test fail if you revert the fix? (Mentally, or for real with `git stash`.)
- Timing assertions (`elapsed < 300ms`) and `time.Sleep` for synchronization make tests flaky.
- Is `t.Fatal`/`t.FailNow` called from a goroutine other than the test's own? An httptest
  handler counts as another goroutine. That's not allowed; use `t.Error` or send to a channel.
- Shared counters in handlers need `atomic`.
- Tests that pass vacuously, for example a `for` loop over an empty slice, or ignored errors (`_ =`).
- Missing cases: cancellation, the error path, edge inputs, leaks.

**Step 3: write comments (~30 min).** Write one thread per issue, on the exact line.
Follow the format in section 4.

**Step 4: summary comment.** End with a summary comment, for example:
> Thanks for tackling this, the speedup will make a real difference to the docs pipeline. 🙌
> I've left a few **blocking** comments, mainly around a data race on the stats counters,
> a goroutine leak in fail-fast mode, and the ordering guarantee that `CheckAll` documents but no
> longer provides. Most of them share one root cause, so I suggested a restructure (indexed
> results plus a cancellable context) that fixes them together. The rest are non-blocking
> suggestions or nits. Happy to pair on it!

---

## 4. Writing comments: Conventional Comments

Format: `label (decorations): subject` + discussion.

| Label | Use for |
|---|---|
| `praise:` | Something done well. Include **at least one** genuine praise comment. |
| `issue:` | A real problem. Pair it with a suggestion. |
| `suggestion:` | A proposed improvement. Say whether it's blocking. |
| `question:` | You're unsure. Ask instead of asserting. |
| `nitpick:` / `nit:` | Trivial, preference-level. Always non-blocking. |
| `thought:` | An idea for later. Non-blocking. |
| `chore:` | Something simple that must be done before merge (for example, run `go mod tidy`). |
| `todo:` | A small, necessary change. |
| `note:` | Something the reader should know. Non-blocking. |

Decorations: `(blocking)`, `(non-blocking)`, `(if-minor)`, and topic tags like `(security)` or `(test)`.

A strong comment covers **what**, **why it matters** (a concrete failure scenario),
**how to reproduce**, and **a suggested fix**, ideally with code:

> **issue (blocking):** `c.checked++` and `c.broken++` are written from every worker goroutine
> without synchronization, so this is a data race. `go test -race ./internal/checker/` reports it
> in `TestStats`. Under load the counts can be wrong, and the summary line could say
> "0 broken" while links are broken, letting the pipeline pass.
>
> Since `report.Write` already returns a `Summary` computed from the results, what do you think
> about dropping `Stats()` entirely? That removes the shared state rather than locking it.
> If we want to keep it, `atomic.Int64` would work.

Tone tips: use "we" and "what do you think about…", assume the author already considered the
alternative, and say which points you feel strongly about.

---

## 5. Go concurrency: what you must know cold

### 5.1 Primitives and their rules
- **Unbuffered channel**: send blocks until a receiver is ready, so it's a synchronization point.
  **Buffered channel** (`make(chan T, n)`): send blocks only when the buffer is full.
- **Closing a channel**: only the **sender** closes, exactly once. A send on a closed channel
  **panics**, and so does closing twice. A receive on a closed channel returns the zero value
  immediately. Use `v, ok := <-ch` to detect it; `for range ch` ends on close.
- A **nil channel** blocks forever. In a `select` that's a useful way to disable a case.
- **select**: picks a *random* ready case. With `default` it never blocks.
- **sync.WaitGroup**: call `Add` **before** the `go` statement, never inside the goroutine.
  Use `defer wg.Done()`. Go 1.25 adds `wg.Go(func(){...})`, which does both for you.
- **sync.Mutex / RWMutex**: keep critical sections small, never copy a mutex (`go vet` copylocks),
  and don't hold a lock while blocking on a channel or doing I/O.
- **sync/atomic**: `atomic.Int64` and friends (Go 1.19+) for counters and flags.
- **sync.Once**: run something exactly once, for example "record the first error and cancel".
- **context**: you pass `ctx` as the first argument and never store it in a struct.
  `WithCancel`, `WithTimeout`, `WithDeadline` and `WithCancelCause` derive one; always
  `defer cancel()`. Check `ctx.Done()` in a `select` around every blocking operation.
- **errgroup** (`golang.org/x/sync/errgroup`): `g, ctx := errgroup.WithContext(ctx)`,
  `g.SetLimit(n)`, `g.Go(func() error {...})`, `err := g.Wait()`. It returns the first error and
  cancels `ctx`. It's the idiomatic answer for "run N tasks, stop on first error".
- **Semaphore**: a buffered channel `sem := make(chan struct{}, n)` limits concurrency.

### 5.2 Patterns to be able to write from memory

**Worker pool that preserves order and supports cancellation (stdlib only):**
```go
func process(ctx context.Context, items []string, workers int) ([]Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]Result, len(items)) // each goroutine writes its own index: no lock needed
	jobs := make(chan int)

	var wg sync.WaitGroup
	for w := 0; w < min(workers, len(items)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = work(ctx, items[i])
			}
		}()
	}

loop:
	for i := range items {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break loop
		}
	}
	close(jobs) // the sender (this goroutine) closes
	wg.Wait()   // every worker has exited, so reading results is safe

	return results, ctx.Err()
}
```

**The same thing with errgroup:**
```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(workers)
results := make([]Result, len(items))
for i, item := range items { // Go ≥1.22: i and item are per-iteration
	g.Go(func() error {
		r, err := work(ctx, item)
		if err != nil {
			return fmt.Errorf("processing %s: %w", item, err)
		}
		results[i] = r
		return nil
	})
}
if err := g.Wait(); err != nil {
	return nil, err
}
```

**Context-aware sleep (for retries or backoff):**
```go
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
```

**Fan-in with a closer goroutine:**
```go
out := make(chan Result)
var wg sync.WaitGroup
// ...start producers with wg.Add(1) / defer wg.Done()...
go func() { wg.Wait(); close(out) }() // close only after every sender is done
for r := range out { ... }
```
The trap: if the consumer returns early, producers block on `out <-` forever. Fix it with
`select { case out <- r: case <-ctx.Done(): return }` plus a cancel on early return, or with a
buffered channel sized to the work.

### 5.3 Classic bugs to look for (with names you can say out loud)
| Bug | Symptom | Fix |
|---|---|---|
| **Data race**: shared variable written by several goroutines | `-race` reports it; wrong counts | mutex, atomic, per-index writes, or channels |
| **Loop variable capture** (`go` directive < 1.22) | all goroutines see the last value; `go vet` flags it | pass it as an argument, `i := i`, or bump `go` to 1.22+ |
| **`wg.Add` inside the goroutine** | `Wait` returns early, so you get a "send on closed channel" panic | `Add` before `go` |
| **Goroutine leak**: blocked send or receive after the caller returned | memory and goroutine growth, hung connections | cancel `ctx`, `select` on `ctx.Done()`, buffered channel, or drain |
| **Deadlock**: everyone waiting | `fatal error: all goroutines are asleep` | check send/receive pairing and close order |
| **Sending on a closed channel / double close** | panic | only the sender closes, once |
| **Ignoring `ctx`** (`time.Sleep`, `http.Get` without ctx) | Ctrl-C and timeouts don't work | `NewRequestWithContext`, context-aware sleep |
| **Unbounded goroutines** (one per item) | file-descriptor and memory exhaustion | worker pool, semaphore, `errgroup.SetLimit` |
| **Lost ordering** from appending in completion order | non-deterministic output; broken contract | index-addressed results |
| **First-error only, others dropped silently** | hidden failures | `errors.Join`, or document "first error wins" |
| **Concurrency ≤ 0 → zero workers** | no work done, nothing reported | validate or clamp |
| **Panic in a goroutine** | the whole process crashes; `recover` in `main` doesn't help | recover inside the goroutine if it's needed |
| **`resp.Body` not closed / not drained** | connection and file-descriptor leaks | `defer resp.Body.Close()`, `io.Copy(io.Discard, …)` |
| **`time.After` inside a loop** (before Go 1.23) | timer leak | `time.NewTimer` + `Stop` |

### 5.4 Memory model in one sentence
Without a happens-before edge, another goroutine may never see your write, or may see it torn.
Channel send/receive, mutex unlock/lock, `wg.Done`/`wg.Wait` and `Once` create that edge.
Sleeping does not.

---

## 6. Error handling

- Wrap with context: `fmt.Errorf("checking %s: %w", url, err)`. Use `%w`, **not `%v`**, so callers can
  use `errors.Is` and `errors.As`.
- Error strings are lowercase with no trailing punctuation. Describe *what was being done*
  ("opening config: …") rather than "failed to … failed to …" chains.
- Handle each error **once**: either log it or return it, not both.
- `errors.Is(err, context.Canceled)` and `errors.As(err, &target)`; custom error types with
  `Unwrap()`; `errors.Join` (Go 1.20+) to return several errors.
- Watch for **`%v` with a nil error**, which prints `<nil>`. Also watch for an error that's
  created but never returned, and for `_ =` that hides a failure.
- Decide on partial-results semantics: does the function return partial results plus an error?
  Document it, and test it.
- Exit codes and user-facing behavior: does an error path still make the CLI exit non-zero?

---

## 7. Testing (heavily weighted)

- **Table-driven tests with `t.Run` subtests**, which match most Go codebases. If the repo uses
  `testify` (`require.NoError`, `assert.Equal`), as many GitLab Go projects do, **match it**.
  If it uses the stdlib only, don't introduce testify.
- **`httptest.NewServer`** for HTTP. Never hit the real network in unit tests.
- **`t.Helper()`** in helpers, **`t.Cleanup`**, `t.TempDir()`, and `t.Context()` (Go 1.24+).
- **Determinism**: no assertions on wall-clock time. Prove concurrency by *measuring in-flight
  requests* with an atomic max counter, which also checks that the limit is respected.
  Use channels to coordinate instead of `time.Sleep`.
- **`testing/synctest`** (stable in Go 1.25) runs time-dependent concurrent code on a fake clock.
  It's a nice thing to mention, but only if the repo's Go version supports it.
- **Leak detection**: `go.uber.org/goleak` (`defer goleak.VerifyNone(t)`), or compare
  `runtime.NumGoroutine()` before and after (less reliable).
- **Race detector in CI**: `go test -race ./...`. Recommend it if missing.
- **`-count=N`** to shake out flakes; `-shuffle=on` to catch order-dependent tests.
- Write the **failing test first** in the live session (red), then fix (green). Interviewers love
  that, because it proves the bug.
- `t.Fatal` only from the test goroutine. In handlers or spawned goroutines use `t.Error`, or send the error back.
- Assert on **lengths** before looping over results, otherwise the test can pass vacuously.
- Test the **error paths and edges**: empty input, invalid config, cancellation, first error.
- Make delays injectable (an unexported `backoff` field or an option) so tests don't sleep for real.

---

## 8. The live coding session: how to shine

1. **Restate the goal** ("So we want CheckAll to keep order and stop leaking on fail-fast. Shall I
   start with a failing test?"). Ask what they want to prioritize.
2. **Red → green → refactor.**
   1. Write the test that exposes the bug and run it (`-race`).
   2. Make the minimal fix and run it again.
   3. Clean up.
3. **Narrate trade-offs**: "I could use a mutex around the append, but writing to `results[i]`
   avoids locking entirely and also keeps the order. Does that work for you?"
4. **Small steps, run often.** Don't write 80 lines silently.
5. **Match the existing style**: naming, error-message format, test helpers, comment density.
6. **When stuck, think out loud**: "I'm not sure whether `select` here will prefer… let me check
   the docs or write a tiny experiment." Asking a question is fine; silence isn't.
7. **Accept hints gracefully.** It's collaborative; they may steer you, so follow and build on it.
8. If something is out of scope, say: "I'd open a follow-up issue for X."
9. Keep 5 minutes at the end for your questions.

---

## 9. Likely questions, with short answers

**Concurrency**
- *Difference between buffered and unbuffered channels? When would you buffer?* With an unbuffered
  channel the sender and receiver meet. With a buffered one, the sender continues until the buffer
  is full. Buffer when you know the number of sends (so senders never block, which can avoid leaks),
  or to absorb bursts. Don't buffer to "fix" a deadlock you don't understand.
- *Who should close a channel?* The sender, the only sender, or a coordinator that waits for
  all senders (WaitGroup), then closes.
- *How do you stop a goroutine?* Cancel its context, or close a done channel, and have it `select`
  on that around every blocking operation.
- *How would you detect a goroutine leak?* With goleak in tests, `pprof` goroutine profiles in
  production, and a `NumGoroutine` metric.
- *Mutex vs channel?* Use a mutex to protect state, and channels to transfer ownership or signal.
  "Share memory by communicating", but don't force channels where a mutex is simpler.
- *What does the race detector do, and what are its limits?* It instruments memory accesses at
  runtime, so it only finds races on code paths that actually ran. It costs roughly 5–10× CPU
  and more memory, and it reports no false positives.
- *What's errgroup and why use it?* WaitGroup plus first-error capture plus context cancellation
  plus a concurrency limit.
- *What changed in Go 1.22 loops?* Each iteration gets a fresh variable. That depends on the `go`
  directive in go.mod, not on the toolchain version.
- *How do you limit concurrency?* A worker pool, a semaphore channel, or `errgroup.SetLimit`.
- *How do you preserve order with concurrent work?* Index-addressed results, or a sequence
  number plus reorder.
- *What happens when a goroutine panics?* The whole program crashes unless that same goroutine
  recovers.
- *`context.WithTimeout` vs `http.Client.Timeout`?* The context is per call and propagates
  cancellation. The client timeout is a blanket limit per request, including reading the body.

**Error handling and testing**
- *`%w` vs `%v`?* `%w` keeps the chain for `errors.Is` and `errors.As`.
- *How do you test code that sleeps or retries?* Inject the delay or clock, use `synctest`, or
  use a tiny backoff in tests.
- *How do you make a flaky concurrent test reliable?* Remove timing assertions, coordinate with
  channels, use atomics, and run with `-race -count=100` to verify.
- *What would you add to CI?* `-race`, `go vet` or `golangci-lint`, and coverage reporting.

**Review and process (GitLab-specific)**
- *How do you decide blocking vs non-blocking?* Blocking means correctness, security, data loss,
  a broken contract, or a missing test for new behavior. Non-blocking means style, alternative
  designs, and follow-ups.
- *The author disagrees with your comment. What do you do?* Seek to understand, discuss the
  trade-offs, and if it isn't a correctness issue, defer to the author. For long threads, suggest
  a quick call, then post a summary.
- *This MR is big. What would you suggest?* Split it, for example retries in one MR and
  concurrency in another. GitLab targets about 200 lines.
- *How would you roll this out safely?* A feature flag or a conservative default (concurrency 1
  → raise it), watch error rates, and mind rate limits from target hosts (`429`, `Retry-After`).
- *Performance indicators in the job description*: unit test coverage, Open MR Review Time and
  Open MR Age. Show you care about review speed (be thorough in one pass) and test coverage.

**Behavioral (have stories ready)**
- A review comment you gave that prevented a bug.
- Receiving tough feedback on your code.
- Debugging a production concurrency issue, or any on-call incident (the job has on-call).
- Shipping iteratively (MVC, "minimal viable change").

---

## 10. Questions to ask them
- "How does the team split review load between reviewers and maintainers?"
- "What does on-call look like for this group in the first months?"
- "Which part of the Go codebase would I work in first (Gitaly, Workhorse, runner, agent)?"
- "What separates an intermediate engineer from a senior on this team?"

---

## 11. Common mistakes to avoid
- Only nitpicks (naming, formatting) and missing the race or leak.
- Calling something a bug without a failure scenario. Say *how* it breaks.
- Rewriting the whole thing in comments. Suggest, don't take over.
- Forgetting tests in your suggested fixes.
- Being harsh, or being so polite the severity is unclear. Use `(blocking)`.
- Introducing a new dependency (errgroup or testify) without asking. It's fine to **propose**
  it, but mention the trade-off: "stdlib version below if we'd rather avoid the dependency".
- Not submitting the review on GitLab. Comments you write as part of a review stay **pending**
  (only you can see them) until you select **Submit review**. Afterwards, refresh the MR and check
  that the comment count went up.
