# GitLab Intermediate Backend Engineer: MR review interview prep guide

What they told you: *Go concurrency, review an MR in 48h, submit 24h before
the call, then a 90-minute call where you walk through your review and
pair-program fixes. Particular emphasis on **testing** and **concurrency
patterns**.*

---

## 1. What they are actually scoring

| Area | What "strong" looks like |
|------|--------------------------|
| Code understanding | You can explain what the app does *before* the change, then what the MR changes and why, in 2 minutes, without reading the code aloud. |
| Concurrency | You spot races, leaks, deadlocks, missing cancellation and unbounded fan-out, and you know which primitive fixes each (and why that one). |
| Error handling | You notice ignored errors, lost context, `log.Fatal` in library-like code, wrong exit codes, and errors that mislead users. |
| Testing | You find tests that pass for the wrong reason, timing-based flakiness, racy test code and missing cases, and you write deterministic tests that **fail before the fix**. |
| Code quality | Your fixes look like the surrounding code: same naming, same error wrapping style, small diffs. |
| Review behaviour | Kind, specific, prioritised (blocking vs non-blocking), backed by evidence, and asks questions when unsure. |
| Collaboration | You think out loud, ask clarifying questions, take hints well and admit what you don't know. |

The review itself is the first impression. They read it before the call.

---

## 2. Timeline

**Now, before the link arrives (practice days)**
- [ ] Do the practice MR in this folder end to end, with a timer (see README).
- [ ] Re-read section 5 (concurrency cheat sheet) until you can explain each
      pattern without notes.
- [ ] Write the worker pool, errgroup, fan-in and "first error cancels the
      rest" patterns **from memory** at least twice.
- [ ] Practise talking while coding: record yourself fixing one bug.
- [ ] Set up Go (match the MR's `go.mod` version or newer), your editor with
      `gopls`, and Zoom screen-share permissions.

**When the link arrives (you have 48h; submit at least 24h before the call)**
- [ ] Clone it, run `go build ./... && go test ./...` **immediately**. If
      anything doesn't run, email them right away; don't burn the window.
- [ ] Read the MR description and linked issue first. *Why* is this change
      being made?
- [ ] Read the base code (`git show main:path`), then the diff, then the full
      new files.
- [ ] Run the tools (section 4).
- [ ] Write comments, post a summary, submit. Plan on 2 to 3 hours, even
      though they estimate 1.

**Day before the call**
- [ ] Re-read your own review. Rank your top 3 issues and prepare a 1-minute
      explanation of each.
- [ ] Check out a clean branch locally, confirm tests run, close noisy apps,
      increase your editor font size for screen sharing.

---

## 3. How to structure the review (GitLab's guidelines, applied)

From GitLab's code review guidelines (the parts that matter here):

- **Understand why the change is necessary first.**
- **Be thorough** to reduce the number of review rounds.
- **Say which ideas you feel strongly about** and which you don't.
- **Offer alternatives, but assume the author already considered them**:
  *"What do you think about…?"*
- **Check out the branch and test locally.** Your testing may reveal missing
  automated tests.
- **If you don't understand something, say so.**
- **Use Conventional Comments.** Mark optional items `(non-blocking)`.
- **End with a summary note** and say clearly whether changes are required.
- Be kind, explicit and humble; ask questions; make suggestions, not demands.
- Balance: *"doing things right"* vs *"doing things right now"*. Don't demand
  a big refactor for an urgent fix.

### Conventional Comments format

```
<label> [decorations]: <subject>

[discussion]
```

| Label | Use for |
|-------|---------|
| `issue` | A concrete problem. Pair it with a suggestion where you can. |
| `suggestion` | A proposed improvement. Say whether it is blocking. |
| `question` | Something you're unsure about. Also the polite way to raise a possible bug. |
| `thought` | An idea worth considering, no action required. |
| `nitpick` | Trivial and preference-based. Always non-blocking. |
| `todo` | A small, necessary change. |
| `chore` | Process work, such as "add `-race` to CI". |
| `praise` | Something done well. Use it sincerely, once or twice. |
| `note` | Context the author should be aware of. |

Decorations: `(blocking)`, `(non-blocking)`, `(if-minor)`, `(security)`,
`(test)`.

### A great inline comment has 4 parts

1. **What**: the problem, in one line.
2. **Why it matters**: the concrete consequence or scenario.
3. **Evidence**: race detector output, a failing test, repro steps.
4. **Suggestion**: code (use GitLab's ```` ```suggestion ```` block for small
   changes) or a question about the approach.

Example:

````
issue (blocking): data race on `m.stats`

`runOnce` now runs in its own goroutine on every tick, so two runs can
increment `m.stats` at the same time, and `Stats()` reads it from the HTTP
handler without holding `m.mu`. `go test -race ./...` reports it:

```
WARNING: DATA RACE
Write at 0x00c0001a4108 by goroutine 31:
  .../monitor.(*Monitor).runOnce() monitor.go:88
Previous read at 0x00c0001a4108 by goroutine 22:
  .../monitor.(*Monitor).Stats() monitor.go:82
```

What do you think about updating `stats` under `m.mu` together with `m.last`
(and `RLock` in `Stats()`)? That also keeps `Checks`/`Failures` consistent
with each other in a single snapshot.
````

### The summary note (post it last, as a top-level comment)

```
Thanks @author, running the checks concurrently is a great improvement and the
MR is nicely described. 🙌

I ran it locally with `go test -race` and tried a few edge cases. A few things
need to change before we merge:

Blocking
1. `CheckAll` returns results in completion order; the order test was changed
   to sort instead of fixing it (link).
2. `-concurrency 0` deadlocks / negative panics (link).
3. `FirstFailure` leaks goroutines and doesn't cancel in-flight requests (link).
4. Data race on `stats` + overlapping runs can write stale results (link).
5. Graceful shutdown panics: `Stop()` is called twice (link).
6. `-once` exits 0 when the config can't be loaded (link).

Tests
- Timing-based and racy assertions; suggestions inline.

Non-blocking
- Consider splitting `-once` and `/stats` into follow-up MRs.
- Add `go test -race` to CI (I can open an issue).

Happy to pair on any of these. Setting to "Changes requested" for now.
```

### Order of reading

1. MR description and linked issue: what problem, which constraints.
2. `go.mod`: the Go version matters (loop variables, `min`/`max`, range over
   int, `testing/synctest`).
3. Base code: how it worked before, what invariants and doc comments exist.
4. The diff, commit by commit. **A commit titled "fix flaky test" is a giant
   hint**: check whether it fixed the test or hid the bug.
5. The full new files, since the diff hides context.
6. Tests last: do they test the new behaviour? Would they fail if the code
   were wrong?

---

## 4. Tools to run (and paste output from)

```shell
go build ./... && go vet ./...
go test ./...
go test -race -count=1 ./...          # the single most important command
go test -race -count=50 -run TestX ./pkg/   # shake out flakiness
go test -cpu=1,2,8 -count=5 ./...     # different GOMAXPROCS
go test -run TestX -v -timeout 10s ./...   # deadlocks become a stack dump
go test -cover ./... ; go test -coverprofile=c.out ./... && go tool cover -func=c.out
staticcheck ./...                     # if installed
GODEBUG=... / SIGQUIT (Ctrl+\) on a hung binary dumps all goroutines
```

Also **run the binary** and try hostile inputs: missing config, 0 or negative
flags, Ctrl+C, slow or failing endpoints. Then look at the exit code
(`echo $?`).

---

## 5. Go concurrency cheat sheet (know this cold)

### Primitives and when to use them

| Need | Use |
|------|-----|
| Protect shared state | `sync.Mutex` / `sync.RWMutex`, kept close to the data; never copy it (`go vet` copylocks) |
| Simple counters or flags | `sync/atomic` types: `atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]` |
| Wait for N goroutines | `sync.WaitGroup`: call **`Add` before `go`**, `defer wg.Done()` (Go 1.25 adds `wg.Go(func)`) |
| Fan-out with the first error cancelling the rest | `errgroup.WithContext` + `g.SetLimit(n)` |
| Bounded concurrency | Worker pool (N goroutines ranging over a jobs channel), a semaphore channel, `errgroup.SetLimit`, or `x/sync/semaphore` |
| Do something once | `sync.Once`, `sync.OnceFunc`, `sync.OnceValue` |
| Cancellation and deadlines | `context.Context`: pass it as the first parameter, `defer cancel()`, check `ctx.Err()`, `select` on `ctx.Done()` |
| Signal "done" to many | `close(ch)`, closed only by the sender or owner and only once |
| Periodic work | `time.NewTicker` + `defer ticker.Stop()`; slow receivers drop ticks |

### Bug patterns to scan for

1. **Data race**: shared variable written by more than one goroutine without
   synchronisation. That includes slices (`append`), maps (they crash:
   *concurrent map writes*), struct fields, and test counters inside
   `httptest` handlers.
2. **`wg.Add` inside the goroutine**: `Wait` can return before `Add` runs.
3. **Goroutine leak**: a send or receive on an unbuffered channel that nobody
   will ever service after an early `return`; a goroutine without a cancel
   path; a ticker that is never stopped.
4. **Deadlock**: unbuffered or zero-capacity semaphore; locking the same mutex
   twice (not reentrant); taking two locks in different orders; waiting on a
   WaitGroup while holding a lock the workers need.
5. **Double close or send on closed channel**: panics. Close only from the
   sender, once (`sync.Once` if you must). Receivers never close.
6. **Missing cancellation**: `context.Background()` deep in code; `ctx` not
   passed into `http.NewRequestWithContext`; `time.Sleep` instead of
   `select { case <-time.After(d): case <-ctx.Done(): }`.
7. **Unbounded fan-out**: one goroutine (and connection) per item over N
   items, which exhausts file descriptors and hits rate limits.
8. **Ordering lost**: results collected via `append` or a channel, but callers
   expect input order. Write to `results[i]` instead.
9. **Overlapping periodic jobs**: `go work()` on every tick. Runs pile up,
   and stale results overwrite fresh ones.
10. **Fire-and-forget at shutdown**: goroutines still running after `Run`
    returns; the caller can't wait; writes after close.
11. **Loop variable capture**: a bug only if `go.mod` < 1.22. **Check
    `go.mod` before flagging it.**
12. **Mutex copied by value** (value receiver, returning a struct containing a
    mutex).
13. **Holding a lock during I/O** (an HTTP call inside `mu.Lock()`). Correct,
    but it serialises everything.
14. **`select` with `default` in a loop**: a busy spin that burns CPU.
15. **`time.After` in a hot loop** (before Go 1.23 the timers leak until they
    fire). Use `time.NewTimer` + `Stop`.

### Patterns to write from memory

**Ordered results with bounded concurrency (worker pool)**

```go
func process(ctx context.Context, items []Item, n int) []Result {
	results := make([]Result, len(items))
	jobs := make(chan int)

	var wg sync.WaitGroup
	for range min(n, len(items)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = do(ctx, items[i]) // distinct index per goroutine: no lock
			}
		}()
	}

	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}
```

If dispatch must stop on cancel:
`select { case jobs <- i: case <-ctx.Done(): break loop }`.

**errgroup: first error cancels the rest, with a limit**

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(n)
results := make([]Result, len(items))
for i, it := range items {
	g.Go(func() error {
		r, err := do(ctx, it)
		if err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
		results[i] = r
		return nil
	})
}
if err := g.Wait(); err != nil { // first non-nil error; ctx cancelled for the others
	return nil, err
}
```

**Semaphore that respects cancellation**

```go
sem := make(chan struct{}, n) // n must be > 0
select {
case sem <- struct{}{}:
case <-ctx.Done():
	return ctx.Err()
}
defer func() { <-sem }()
```

**First result, no leak**

```go
ctx, cancel := context.WithCancel(ctx)
defer cancel()
ch := make(chan Result, len(items)) // buffered: senders never block after we return
for _, it := range items {
	go func() { ch <- do(ctx, it) }()
}
for range items {
	if r := <-ch; r.Err != nil {
		return &r // cancel() runs; remaining goroutines finish and exit
	}
}
```

**Periodic loop done right**

```go
func (m *M) Run(ctx context.Context) {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		m.runOnce(ctx) // synchronous: no overlap; slow runs drop ticks
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
```

**Idempotent stop (only if you can't just use ctx)**

```go
func (m *M) Stop() { m.stopOnce.Do(func() { close(m.stop) }) }
```

### "Why this primitive?" answers

- **Mutex vs channel**: *"Share memory by communicating"* is the guideline,
  but a mutex is simpler for protecting a struct's state. Use channels to
  transfer ownership or to signal events.
- **Atomic vs mutex**: atomics for independent single values. A mutex when
  several fields must change together and readers need a consistent snapshot.
- **RWMutex**: only when reads clearly dominate and the critical section is
  non-trivial. Otherwise a plain `Mutex` is simpler and often faster.
- **Buffered channel**: to decouple sender from receiver, or to guarantee
  senders can't block (buffer = number of sends). Not as a "fix" for an
  unexplained deadlock.

---

## 6. Error handling checklist

- Every `err` is checked; no `_ =` without a comment explaining why.
- Wrap with context: `fmt.Errorf("loading endpoints: %w", err)`. Use `%w`, not
  `%v`, when callers might `errors.Is` or `errors.As`.
- Errors are returned, not logged and then returned (that logs twice), and
  not `log.Fatal`ed outside `main`. `log.Fatal` and `os.Exit` skip defers.
- Exit codes: a CLI used as a CI gate **must** exit non-zero on failure,
  including config errors.
- `context.Canceled` and `DeadlineExceeded` are often expected during
  shutdown. Don't report them as failures or alerts.
- Nil-pointer risks: a returned pointer to a result whose `Err` is nil but
  whose status is bad; `resp` used when `err != nil`.
- `defer resp.Body.Close()` only after checking `err`; drain the body to reuse
  the connection.
- Validate inputs at the boundary (flags and config): `concurrency > 0`,
  `interval > 0`. `time.NewTicker` panics on a non-positive duration.
- Panics in goroutines crash the whole process. Know when `recover` is
  warranted (rarely; mainly in servers' per-request goroutines).

---

## 7. Testing: what they want to see

**In review, ask of every test:** *Would this test fail if the code were
broken?* If not, it isn't testing the behaviour.

Red flags:
- `time.Sleep` for synchronisation, and `elapsed < X` assertions (flaky in CI).
- Shared counters in `httptest` handlers without atomics (a race in test code).
- A test changed to accommodate new behaviour, such as sorting results or
  loosening an assertion. Ask whether the behaviour change was intended.
- Only the happy path: no empty input, zero or negative config, cancellation,
  every error path, or concurrent access.
- Asserting `> 0` instead of exact values.
- No `-race` in CI.

Techniques:
- **Table-driven tests** with `t.Run`. `t.Parallel()` only when the cases are
  independent.
- `httptest.NewServer` + `t.Cleanup(srv.Close)`. Control latency per
  endpoint, and make the slow ones block on `r.Context().Done()`.
- **Make concurrency bugs deterministic**: give endpoints different latency so
  completion order differs from input order; track the maximum in flight with
  atomics to test a limit (not wall-clock time).
- **Detecting hangs**: run the call in a goroutine and
  `select { case <-done: case <-time.After(5*time.Second): t.Fatal("deadlock") }`.
- **Detecting leaks**: `runtime.NumGoroutine()` before and after (with
  polling), or `go.uber.org/goleak` (`defer goleak.VerifyNone(t)`).
- **Poll, don't sleep**: a `waitFor(cond, timeout)` helper.
- **`testing/synctest`** (Go 1.25, experimental in 1.24): fake time and
  `synctest.Wait()` for deterministic tests of timers, tickers and goroutines.
  Mention it if `go.mod` allows it.
- Always finish with `go test -race -count=N`.
- **Red, then green**: in the live session, write or restore the failing test
  *first*, run it and show it fail, then fix. Interviewers love this.

---

## 8. Live session (90 minutes): how to behave

Rough shape: 5 minutes intros, 20 to 30 minutes walking through your review,
45 to 60 minutes pairing on fixes, 5 to 10 minutes for your questions.

**Walkthrough**
- Start with a 1-minute summary: *"The service does X. The MR does Y to solve
  Z. Overall it's a good direction. My main concerns are A, B, C, in that
  order."*
- Show, don't tell: run `go test -race` live and reproduce the shutdown panic.
- Be open about uncertainty: *"I flagged this as a question because I wasn't
  sure whether callers rely on the order."*

**Coding**
- Restate the task, ask a clarifying question, and **propose a plan before
  typing**: *"I'll first write a test that reproduces the race, then …"*
- Narrate decisions and trade-offs: *"I'm using a mutex rather than atomics
  because these three counters must stay consistent."*
- Small steps. Run tests often. Commit when green.
- If stuck: say what you're thinking, simplify, and ask. Taking a hint well is
  a positive signal.
- Match the code style: the same error wrapping, naming and doc comments.
  Run `gofmt` and `goimports` on save.
- Don't gold-plate. Fix the thing, show it works, then mention what else
  you'd do.

**Phrases that work**
- "Before I change this, let me write a test that fails because of it."
- "There are two options: X is simpler, Y is more scalable. Given the scale,
  I'd pick X. What do you think?"
- "I'd treat that as non-blocking and open a follow-up issue."
- "I'm not 100% sure how `errgroup` handles panics. I believe it doesn't
  recover them. I'd check the docs."

---

## 9. Likely questions

**Concurrency**
1. What's a data race vs a race condition? (A data race is unsynchronised
   concurrent access with at least one write. A race condition is a logic
   bug that depends on timing; it can exist with no data race.)
2. How does the race detector work, and what are its limits? (It instruments
   memory accesses and uses happens-before. It only finds races on executed
   paths and costs roughly 2 to 20 times the CPU.)
3. Buffered vs unbuffered channels? What happens when you send on a closed
   channel, close twice, or read from a nil channel?
4. How do you prevent goroutine leaks? How do you detect them?
5. Explain `context` propagation. What happens if you don't call `cancel`?
6. Why must `wg.Add` happen before `go`?
7. Mutex vs channel: when would you use each?
8. How would you limit concurrency? Compare a worker pool, a semaphore and
   `errgroup.SetLimit`.
9. What changed in Go 1.22 for loop variables? 1.23 for timers? 1.25 for
   `WaitGroup.Go` and `synctest`?
10. What's GOMAXPROCS? What does the scheduler do with blocking syscalls vs
    network I/O?
11. How does `select` choose when several cases are ready? (Uniformly at
    random.)
12. What's the memory model's happens-before guarantee for channels and
    mutexes?

**Testing**
13. How would you test that something runs concurrently, without timing?
14. How do you make a flaky test deterministic?
15. Unit vs integration tests here: what would you mock, if anything?
    (`httptest` is real HTTP, so no mocks are needed.)
16. What does `t.Cleanup` give you over `defer`? What does `t.Parallel` do
    with subtests and loop variables?

**Errors and design**
17. `errors.Is` vs `errors.As`; `%w` vs `%v`; sentinel errors vs typed errors.
18. When is `panic` acceptable?
19. How would you shut this service down gracefully?
20. Would you split this MR? How?

**GitLab-specific and behavioural**
21. How do you balance thoroughness against review speed (OMRT and OMA KPIs)?
    (Prioritise blocking issues, approve with non-blocking comments, follow
    up in issues.)
22. Tell us about a review where you disagreed with the author. How was it
    resolved?
23. Tell us about a production incident you debugged. (On-call is part of
    the job.)
24. How do you receive critical feedback on your own MR?
25. Which GitLab value resonates with you? (CREDIT: Collaboration, Results
    for Customers, Efficiency, Diversity Inclusion & Belonging, Iteration,
    Transparency. **Iteration** maps directly to small MRs.)
26. How do you work asynchronously across time zones? (GitLab is all-remote
    and handbook-first.)

**Questions you can ask them**
- What does a typical MR in the team look like (size, number of reviewers)?
- How is the team's on-call rotation structured?
- What's the biggest source of technical debt the team is working on?
- How do new engineers become reviewers and then maintainers?

---

## 10. Tips

- Leave **no more than ~12 to 15 inline comments**. Merge duplicates and put
  the rest in the summary. Quality beats volume.
- Add **one or two sincere `praise:` comments**. They show judgement and
  match GitLab's "be kind" guideline.
- **Every blocking comment should come with evidence** (a race trace, a repro
  command, or a failing test you wrote locally).
- **Watch for the trap**: the code that *looks* wrong but isn't (loop
  variables in Go 1.22 and later). Saying "this is fine because…" shows depth.
- **Commit history is part of the review**: "fix flaky test" commits, debug
  leftovers, unrelated changes.
- **Check the docs and comments against the code.** A stale doc comment is
  often where a behaviour change hides.
- **Mention scale and production**: "what happens at 10,000 endpoints, on
  shutdown, when the network is down, with bad config?"
- **Suggest process fixes**: `-race` in CI, splitting the MR, a follow-up
  issue for non-blocking work.
- Be ready to push code to the MR branch during the call: keep the git
  basics fresh (`git switch -c`, `git add -p`, `git commit`, `git push`).
- Sleep well. A calm, communicative candidate who finds 6 issues beats a
  stressed one who finds 10.
