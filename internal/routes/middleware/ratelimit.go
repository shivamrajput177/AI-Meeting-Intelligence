package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/redisx"
)

// RateLimit is a per-IP fixed-window limiter (see redisx.AllowFixedWindow
// for why fixed-window rather than the token-bucket design in
// docs/architecture/observability-security.md). routeLabel identifies the
// route in the Redis key so /auth/login and /auth/signup, say, get
// independent budgets even from the same IP.
func RateLimit(rdb *redis.Client, routeLabel string, limit int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := redisx.RateLimitKey(c.IP(), routeLabel)
		allowed, err := redisx.AllowFixedWindow(c.Context(), rdb, key, limit, ratelimitWindow)
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

const ratelimitWindow = 60 * time.Second
