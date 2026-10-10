package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DefaultConcurrency is the default number of endpoints checked in parallel.
const DefaultConcurrency = 10

// Checker performs HTTP health checks against endpoints.
type Checker struct {
	client      *http.Client
	concurrency int
}

// NewChecker returns a Checker whose requests time out after timeout and
// which checks at most concurrency endpoints at the same time. A
// non-positive concurrency falls back to DefaultConcurrency.
func NewChecker(timeout time.Duration, concurrency int) *Checker {
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	return &Checker{
		client:      &http.Client{Timeout: timeout},
		concurrency: concurrency,
	}
}

// Check performs a single GET request against ep and reports the result.
func (c *Checker) Check(ctx context.Context, ep Endpoint) Result {
	start := time.Now()
	result := Result{Endpoint: ep, CheckedAt: start}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.URL, nil)
	if err != nil {
		return result.withErr(fmt.Errorf("building request: %w", err))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		result.Latency = time.Since(start)
		return result.withErr(fmt.Errorf("requesting %s: %w", ep.Name, err))
	}
	defer resp.Body.Close()

	// Drain the body so the underlying connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	result.StatusCode = resp.StatusCode
	result.Latency = time.Since(start)

	return result
}

// CheckAll checks every endpoint and returns the results in the same order
// as eps. At most c.concurrency endpoints are checked at the same time.
func (c *Checker) CheckAll(ctx context.Context, eps []Endpoint) []Result {
	results := make([]Result, len(eps))

	c.forEach(ctx, eps, func(i int, r Result) {
		// Each worker writes to its own index, so no locking is needed.
		results[i] = r
	})

	return results
}

// FirstFailure checks the endpoints concurrently and returns the first
// unhealthy result, or nil if every endpoint is healthy. Checks that are
// still in flight when a failure is found are cancelled.
func (c *Checker) FirstFailure(ctx context.Context, eps []Endpoint) *Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		once    sync.Once
		failure *Result
	)

	c.forEach(ctx, eps, func(_ int, r Result) {
		if r.Healthy() {
			return
		}
		once.Do(func() {
			failure = &r
			cancel()
		})
	})

	return failure
}

// forEach checks eps using a bounded pool of workers and calls fn with the
// index and result of every check. fn may be called concurrently. forEach
// returns once every worker has exited, so no goroutines outlive the call.
func (c *Checker) forEach(ctx context.Context, eps []Endpoint, fn func(i int, r Result)) {
	jobs := make(chan int)

	var wg sync.WaitGroup
	for range min(c.concurrency, len(eps)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				fn(i, c.Check(ctx, eps[i]))
			}
		}()
	}

	for i := range eps {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
}

func (r Result) withErr(err error) Result {
	r.Err = err
	r.Error = err.Error()
	return r
}
