package middleware

import (
	"net"
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/redisx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// RateLimit is a per-IP token-bucket limiter (redisx.AllowTokenBucket) —
// see that package for why token bucket needs an atomic Lua script rather
// than a plain Redis counter. capacity is the burst size (how many
// requests can fire back-to-back before throttling kicks in);
// refillPerSecond is the steady-state rate the bucket refills at
// afterward. routeLabel identifies the route in the Redis key so
// /auth/login and /auth/signup, say, get independent budgets even from
// the same IP.
func RateLimit(rdb *redis.Client, routeLabel string, capacity int, refillPerSecond float64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// r.RemoteAddr is "ip:port"; strip the port so the same client
			// on different ephemeral ports shares one budget. Falls back to
			// the raw value if it's ever not in host:port form (harmless —
			// just means the key is slightly less clean, not wrong).
			identity, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				identity = r.RemoteAddr
			}

			key := redisx.RateLimitKey(identity, routeLabel)
			allowed, err := redisx.AllowTokenBucket(r.Context(), rdb, key, capacity, refillPerSecond)
			if err != nil {
				// Same availability-over-strictness call as Auth's revocation
				// check: a Redis hiccup shouldn't 500 every login attempt.
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), apperr.TooManyRequests("too many requests, try again shortly"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
