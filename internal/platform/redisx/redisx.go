// Package redisx is the shared Redis client wrapper — used for revoked
// access-token jtis (Auth Service, checked by the Gateway) and basic
// rate-limit counters (Gateway). See
// docs/architecture/microservices.md §2 and §1.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}

// RevokedKey is the key a revoked access token's jti is stored under,
// with a TTL equal to the token's remaining life — see
// docs/architecture/microservices.md §2 ("Revoked/blacklisted access-token
// jtis cached in Redis").
func RevokedKey(jti string) string { return "revoked:" + jti }

func MarkRevoked(ctx context.Context, rdb *redis.Client, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil // already expired anyway
	}
	return rdb.Set(ctx, RevokedKey(jti), "1", ttl).Err()
}

func IsRevoked(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	n, err := rdb.Exists(ctx, RevokedKey(jti)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AllowFixedWindow implements a simple fixed-window rate limiter: at most
// limit calls per window for the given key. It's a deliberately simpler
// stand-in for the token-bucket design in
// docs/architecture/observability-security.md — good enough to demonstrate
// and exercise the pattern in Phase 1; swapping in a token bucket later
// doesn't change any caller of this function.
func AllowFixedWindow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := rdb.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return count <= int64(limit), nil
}

// RateLimitKey builds a per-identity, per-route rate-limit counter key.
func RateLimitKey(identity, route string) string {
	return fmt.Sprintf("ratelimit:%s:%s", route, identity)
}
