// Package handler is the API Gateway's own delivery layer — it has no
// domain/usecase/repository of its own (see
// docs/architecture/folder-structure.md), only routing, auth, and
// rate-limit middleware in front of a reverse proxy to every other
// service.
package handler

import (
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/api-gateway/proxy"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/middleware"
)

type ServiceURLs struct {
	Auth    string
	User    string
	Org     string
	Meeting string
}

// Register mounts every /api/v1/... route documented in
// docs/architecture/api-spec.md onto mux — a public set (Auth Service — a
// caller has no JWT yet by definition) and a JWT-protected set (everything
// else). Each pattern is registered without a method prefix so it matches
// every HTTP method on that path (the gateway doesn't care which method a
// route uses, only the target service does — the net/http.ServeMux
// equivalent of Fiber's .All()).
func Register(mux *http.ServeMux, urls ServiceURLs, jwtSecret []byte, rdb *redis.Client, log *logger.Logger) {
	// Token-bucket limits below are expressed as (burst capacity, steady
	// refill rate) — e.g. signup allows up to 10 back-to-back attempts,
	// then refills at 10-per-minute after that, rather than a hard reset
	// every 60s (see middleware.RateLimit and redisx.AllowTokenBucket for
	// why token bucket over the fixed-window counter this replaced).
	const (
		signupBurst, signupPerMinute = 10, 10.0
		loginBurst, loginPerMinute   = 20, 20.0
		resetBurst, resetPerMinute   = 10, 10.0
	)

	mux.Handle("/api/v1/auth/signup",
		middleware.RateLimit(rdb, "auth-signup", signupBurst, signupPerMinute/60)(proxy.ForwardTo(urls.Auth)))
	mux.Handle("/api/v1/auth/login",
		middleware.RateLimit(rdb, "auth-login", loginBurst, loginPerMinute/60)(proxy.ForwardTo(urls.Auth)))
	mux.Handle("/api/v1/auth/refresh", proxy.ForwardTo(urls.Auth))
	mux.Handle("/api/v1/auth/logout", proxy.ForwardTo(urls.Auth))
	mux.Handle("/api/v1/auth/password/reset-request",
		middleware.RateLimit(rdb, "auth-reset", resetBurst, resetPerMinute/60)(proxy.ForwardTo(urls.Auth)))
	mux.Handle("/api/v1/auth/password/reset-confirm", proxy.ForwardTo(urls.Auth))

	auth := middleware.Auth(jwtSecret, rdb, log)
	protect := func(pattern, target string) {
		mux.Handle(pattern, auth(proxy.ForwardTo(target)))
	}

	protect("/api/v1/users/me", urls.User)
	protect("/api/v1/orgs/{orgId}/users/{userId}", urls.User)

	protect("/api/v1/orgs", urls.Org)
	protect("/api/v1/orgs/{orgId}", urls.Org)
	protect("/api/v1/orgs/{orgId}/settings", urls.Org)
	protect("/api/v1/orgs/{orgId}/usage", urls.Org)

	protect("/api/v1/meetings", urls.Meeting)
	protect("/api/v1/meetings/{id}", urls.Meeting)
	protect("/api/v1/meetings/{id}/complete-upload", urls.Meeting)
	protect("/api/v1/meetings/{id}/status", urls.Meeting)
}
