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
// which checks at most concurrency endpoints at the same time.
func NewChecker(timeout time.Duration, concurrency int) *Checker {
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
// as eps.
func (c *Checker) CheckAll(ctx context.Context, eps []Endpoint) []Result {
	var (
		results []Result
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	sem := make(chan struct{}, c.concurrency)

	for _, ep := range eps {
		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			result := c.Check(ctx, ep)
			<-sem

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}()
	}

	wg.Wait()

	return results
}

// FirstFailure checks all endpoints concurrently and returns the first
// unhealthy result it sees, or nil if every endpoint is healthy.
func (c *Checker) FirstFailure(ctx context.Context, eps []Endpoint) *Result {
	failures := make(chan Result)
	done := make(chan struct{})

	for _, ep := range eps {
		go func() {
			result := c.Check(ctx, ep)
			if !result.Healthy() {
				failures <- result
			}
			done <- struct{}{}
		}()
	}

	for range eps {
		select {
		case f := <-failures:
			return &f
		case <-done:
		}
	}

	return nil
}

func (r Result) withErr(err error) Result {
	r.Err = err
	r.Error = err.Error()
	return r
}
