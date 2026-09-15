// Package config loads process configuration from a JSON file on disk
// instead of environment variables. Each service ships a
// configs/<service>.template.json showing the full shape with dev-safe
// defaults; copy it to configs/<service>.json (or pass -config with a
// different path) and edit in real values for your environment.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Load reads the JSON file at path and decodes it into a new T. T is
// normally a small struct owned by the service itself (see
// cmd/auth-service/main.go for an example) — each service defines its own
// config shape rather than sharing one giant struct.
func Load[T any](path string) (T, error) {
	var cfg T

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// ParseDuration parses s as a Go duration string (e.g. "15m", "168h"),
// falling back to def if s is empty or invalid. JSON has no native
// duration type, so fields like token TTLs are stored as plain strings
// and go through this at startup.
func ParseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
