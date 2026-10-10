## What does this MR do and why?

The docs pipeline now has more than 2,000 links, and `linkcheck` checks them
one at a time. The `docs-lint links` job takes more than 15 minutes and often
hits the job timeout.

This MR checks links concurrently with a pool of workers:

- `CheckAll` hands URLs to `-concurrency` workers (default 10).
- Transient failures (`429`, `5xx`) are retried with exponential backoff, so a
  flaky upstream no longer fails the whole pipeline.
- `-fail-fast` stops at the first broken link, for quicker feedback in MR
  pipelines.
- `-v` enables debug logging.
- `Checker.Stats()` exposes how many links were checked and how many were
  broken, for the summary line.

Results are still reported in a stable order, so the job output stays easy to
diff.

## References

Closes #12: "Docs link check takes 15+ minutes".

## Screenshots or screen recordings

| Before | After |
|--------|-------|
| `real 15m42s` | `real 1m51s` |

## How to set up and validate locally

1. `make test`
2. `make build`
3. `./bin/linkcheck -concurrency 20 -v testdata/sample.md`
4. `./bin/linkcheck -fail-fast testdata/sample.md`. It stops at the first
   broken link.

## MR acceptance checklist

- [x] I have self-reviewed this MR.
- [x] Tests cover the change.
- [x] Documentation is updated.
