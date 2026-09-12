// Package config loads process configuration from environment variables.
//
// Phase 1 keeps this deliberately simple (os.Getenv + defaults, no config
// file/Vault/etc.) — see docs/ROADMAP.md Phase 1 for the "walking skeleton
// first" philosophy this whole service layer follows.
package config

import (
	"os"
	"strconv"
	"time"
)

// Env returns the value of the given environment variable, or def if unset
// or empty.
func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// EnvDuration parses the given environment variable as a Go duration
// string (e.g. "15m", "168h"), falling back to def on error or if unset.
func EnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// EnvInt parses the given environment variable as an int, falling back to
// def on error or if unset.
func EnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// EnvBool parses the given environment variable as a bool, falling back to
// def on error or if unset.
func EnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
