package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Endpoint is a single HTTP endpoint to be checked.
type Endpoint struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Result is the outcome of checking a single endpoint.
type Result struct {
	Endpoint   Endpoint      `json:"endpoint"`
	StatusCode int           `json:"status_code,omitempty"`
	Latency    time.Duration `json:"latency"`
	Err        error         `json:"-"`
	Error      string        `json:"error,omitempty"`
	CheckedAt  time.Time     `json:"checked_at"`
}

// Healthy reports whether the endpoint responded with a non-error status code.
func (r Result) Healthy() bool {
	return r.Err == nil && r.StatusCode >= 200 && r.StatusCode < 400
}

// LoadEndpoints reads a JSON list of endpoints from path.
func LoadEndpoints(path string) ([]Endpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading endpoints file: %w", err)
	}

	var endpoints []Endpoint
	if err := json.Unmarshal(data, &endpoints); err != nil {
		return nil, fmt.Errorf("parsing endpoints file: %w", err)
	}

	if len(endpoints) == 0 {
		return nil, errors.New("endpoints file contains no endpoints")
	}

	for i, ep := range endpoints {
		if ep.Name == "" || ep.URL == "" {
			return nil, fmt.Errorf("endpoint %d: name and url are required", i)
		}
	}

	return endpoints, nil
}
