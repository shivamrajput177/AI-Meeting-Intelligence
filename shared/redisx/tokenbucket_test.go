package redisx

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// requireRedis skips the test if no Redis is reachable at REDIS_TEST_ADDR
// (default localhost:6379) — the token bucket algorithm's correctness
// lives entirely in a Lua script, so unlike the rest of this package
// there's no meaningful way to test it without a real Redis to run that
// script against. This mirrors how the Postgres-backed repositories
// elsewhere in this repo are verified (a live database), not mocked.
func requireRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := "localhost:6379"
	if v := os.Getenv("REDIS_TEST_ADDR"); v != "" {
		addr = v
	}
	rdb := NewClient(addr)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("skipping: no Redis reachable at %s: %v", addr, err)
	}
	return rdb
}

func TestAllowTokenBucket_BurstThenThrottleThenRefill(t *testing.T) {
	rdb := requireRedis(t)
	key := t.Name() + ":" + time.Now().Format(time.RFC3339Nano)
	defer func() { _ = rdb.Del(context.Background(), key).Err() }()
	ctx := context.Background()

	const capacity = 5
	const refillPerSecond = 1.0

	// A fresh bucket starts full: exactly `capacity` requests succeed
	// back-to-back (the burst), and the next one is throttled.
	for i := 0; i < capacity; i++ {
		allowed, err := AllowTokenBucket(ctx, rdb, key, capacity, refillPerSecond)
		if err != nil {
			t.Fatalf("AllowTokenBucket: %v", err)
		}
		if !allowed {
			t.Fatalf("request %d/%d: expected allowed within burst capacity", i+1, capacity)
		}
	}

	allowed, err := AllowTokenBucket(ctx, rdb, key, capacity, refillPerSecond)
	if err != nil {
		t.Fatalf("AllowTokenBucket: %v", err)
	}
	if allowed {
		t.Fatal("expected the request immediately after exhausting the burst to be throttled")
	}

	// After waiting long enough to refill ~2 tokens, exactly 2 more
	// requests should succeed before throttling again.
	time.Sleep(2100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		allowed, err := AllowTokenBucket(ctx, rdb, key, capacity, refillPerSecond)
		if err != nil {
			t.Fatalf("AllowTokenBucket: %v", err)
		}
		if !allowed {
			t.Fatalf("request %d/2 after refill: expected allowed", i+1)
		}
	}

	allowed, err = AllowTokenBucket(ctx, rdb, key, capacity, refillPerSecond)
	if err != nil {
		t.Fatalf("AllowTokenBucket: %v", err)
	}
	if allowed {
		t.Fatal("expected throttling again after consuming the refilled tokens")
	}
}

func TestAllowTokenBucket_IndependentKeysDoNotShareBudget(t *testing.T) {
	rdb := requireRedis(t)
	ctx := context.Background()
	keyA := t.Name() + ":a:" + time.Now().Format(time.RFC3339Nano)
	keyB := t.Name() + ":b:" + time.Now().Format(time.RFC3339Nano)
	defer func() {
		_ = rdb.Del(ctx, keyA, keyB).Err()
	}()

	// Exhaust keyA's single-token bucket.
	allowed, err := AllowTokenBucket(ctx, rdb, keyA, 1, 1.0)
	if err != nil || !allowed {
		t.Fatalf("keyA first request: allowed=%v err=%v", allowed, err)
	}
	allowed, err = AllowTokenBucket(ctx, rdb, keyA, 1, 1.0)
	if err != nil {
		t.Fatalf("keyA second request: %v", err)
	}
	if allowed {
		t.Fatal("expected keyA's bucket to be exhausted")
	}

	// keyB has never been touched and must be unaffected by keyA's state.
	allowed, err = AllowTokenBucket(ctx, rdb, keyB, 1, 1.0)
	if err != nil || !allowed {
		t.Fatalf("keyB request: expected allowed, got allowed=%v err=%v", allowed, err)
	}
}
