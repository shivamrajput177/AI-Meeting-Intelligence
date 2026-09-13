package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/redisx"
)

// RateLimit is a per-IP token-bucket limiter (redisx.AllowTokenBucket) —
// see that package for why token bucket needs an atomic Lua script rather
// than a plain Redis counter. capacity is the burst size (how many
// requests can fire back-to-back before throttling kicks in);
// refillPerSecond is the steady-state rate the bucket refills at
// afterward. routeLabel identifies the route in the Redis key so
// /auth/login and /auth/signup, say, get independent budgets even from
// the same IP.
func RateLimit(rdb *redis.Client, routeLabel string, capacity int, refillPerSecond float64) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := redisx.RateLimitKey(c.IP(), routeLabel)
		allowed, err := redisx.AllowTokenBucket(c.Context(), rdb, key, capacity, refillPerSecond)
		if err != nil {
			// Same availability-over-strictness call as Auth's revocation
			// check: a Redis hiccup shouldn't 500 every login attempt.
			return c.Next()
		}
		if !allowed {
			return apperr.TooManyRequests("too many requests, try again shortly")
		}
		return c.Next()
	}
}
