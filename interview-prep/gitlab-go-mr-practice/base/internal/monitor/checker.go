package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Checker performs HTTP health checks against endpoints.
type Checker struct {
	client *http.Client
}

// NewChecker returns a Checker whose requests time out after timeout.
func NewChecker(timeout time.Duration) *Checker {
	return &Checker{
		client: &http.Client{Timeout: timeout},
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
	results := make([]Result, 0, len(eps))

	for _, ep := range eps {
		results = append(results, c.Check(ctx, ep))
	}

	return results
}

func (r Result) withErr(err error) Result {
	r.Err = err
	r.Error = err.Error()
	return r
}
