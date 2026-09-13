package redisx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// tokenBucketScript implements the whole algorithm as one atomic Redis
// operation. This has to be a Lua script (not separate GET/compute/SET
// calls from Go) because token bucket is a read-modify-write: two
// concurrent requests both reading "3 tokens left," both deciding to
// allow, and both writing "2 tokens left" would let more requests through
// than the bucket ever actually had — exactly the race INCR's atomicity
// sidesteps for a simple counter, but a plain GET/SET can't sidestep for
// this. Redis guarantees a script's body runs to completion with no other
// command interleaved, which is what makes this safe.
//
// KEYS[1] = bucket key
// ARGV[1] = capacity (max tokens the bucket can hold — the burst size)
// ARGV[2] = refill rate, in tokens per second (the steady-state allowed rate)
// ARGV[3] = now, unix time in seconds (float, sub-second precision)
// ARGV[4] = tokens requested by this call (always 1 here, but the script
//
//	doesn't need to assume that)
//
// Returns 1 if the request is allowed (and debits the bucket), 0 if not.
var tokenBucketScript = redis.NewScript(`
local tokens_key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call("HMGET", tokens_key, "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

-- First time this key is seen: start full, per the standard token-bucket
-- convention (a new client isn't penalized for a bucket that "hasn't
-- refilled yet").
if tokens == nil then
  tokens = capacity
  last_refill = now
end

local elapsed = math.max(0, now - last_refill)
tokens = math.min(capacity, tokens + elapsed * refill_rate)

local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end

redis.call("HMSET", tokens_key, "tokens", tostring(tokens), "last_refill", tostring(now))
-- A fully-drained bucket takes capacity/refill_rate seconds to refill
-- completely; expiring shortly after that means an idle client's key
-- doesn't linger in Redis forever, while an active client's key is
-- refreshed by this same call before it would ever expire.
redis.call("EXPIRE", tokens_key, math.ceil(capacity / refill_rate) + 1)

return allowed
`)

// AllowTokenBucket reports whether one request against key is allowed
// right now, under a token bucket with the given capacity (max burst) and
// refillPerSecond (steady-state rate). See tokenBucketScript's doc
// comment for why this has to run as a single atomic script rather than
// separate Go-side read/compute/write calls.
func AllowTokenBucket(ctx context.Context, rdb *redis.Client, key string, capacity int, refillPerSecond float64) (bool, error) {
	now := float64(time.Now().UnixNano()) / 1e9
	res, err := tokenBucketScript.Run(ctx, rdb, []string{key}, capacity, refillPerSecond, now, 1).Result()
	if err != nil {
		return false, err
	}
	allowed, _ := res.(int64)
	return allowed == 1, nil
}
