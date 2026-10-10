## What does this MR do and why?

With a few dozen endpoints configured, a full monitoring run takes longer than
the check interval, because every endpoint is checked one after the other. On
our staging config (40 endpoints, 5s timeout) a single slow endpoint can push
the run past the 30s interval, so `/status` shows stale data.

This MR:

- Checks endpoints **concurrently** in `Checker.CheckAll`, limited by a new
  `-concurrency` flag (default `10`).
- Adds a `-once` mode so the binary can be used as a health gate in CI jobs: it
  exits non-zero as soon as any endpoint is unhealthy (`Checker.FirstFailure`).
- Runs each monitoring cycle in its own goroutine so a slow run can't delay the
  next scheduled one.
- Exposes run statistics (`runs`, `checks`, `failures`) on a new `/stats`
  endpoint.
- Adds `Monitor.Stop()` for a clean shutdown.

Closes #12

## Screenshots or screen recordings

| Before | After |
|--------|-------|
| 40 endpoints: full run ~31s | 40 endpoints: full run ~3.4s |

```shell
$ curl -s localhost:8080/stats
{"runs":2,"checks":80,"failures":3}
```

## How to set up and validate locally

1. `go run ./cmd/monitor -config endpoints.example.json -interval 10s`
2. `curl localhost:8080/status` and `curl localhost:8080/stats`
3. `go run ./cmd/monitor -config endpoints.example.json -once; echo $?`
4. `go test ./...`

## MR acceptance checklist

- [x] I have self-reviewed this MR per the code review guidelines.
- [x] Tests added for the new behaviour.
- [x] `go test ./...` passes locally and in CI.
- [x] Documentation (README) updated.
- [ ] Changelog entry — not needed, internal tool.

/label ~"type::feature" ~backend
