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
// config shape rather than sharing one giant struct. Equivalent to
// LoadMerged with a single path.
func Load[T any](path string) (T, error) {
	return LoadMerged[T](path)
}

// LoadMerged reads each of paths in order and decodes it onto the same T,
// so a later path's JSON only overrides the keys it actually sets —
// json.Unmarshal onto an already-populated struct leaves fields absent
// from the new blob untouched, which is exactly the overlay semantics
// this needs. An empty path is skipped, so a single real path behaves
// identically to Load.
//
// This exists for deploy/helm (Phase 5): kubernetes-cicd.md §2 calls for
// a ConfigMap (non-secret settings) plus a separate Secret (DB URL, JWT
// signing key, integration tokens) per service, never one plaintext blob
// — LoadMerged lets each service's main.go pass both mounted files
// (-config for the ConfigMap, -secrets for the Secret) without this
// package growing a second, different config-loading mechanism.
// docker-compose and local dev pass only -config and never notice this
// exists.
func LoadMerged[T any](paths ...string) (T, error) {
	var cfg T
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config %s: %w", path, err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config %s: %w", path, err)
		}
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
