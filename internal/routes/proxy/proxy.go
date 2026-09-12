// Package proxy is the API Gateway's reverse-proxy layer. There is no
// protocol translation here — the gateway forwards the exact REST request
// it received to the target service's own REST API, per
// docs/architecture/microservices.md §"Internal Communication".
package proxy

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

// apiPrefix is stripped before forwarding — the gateway exposes
// /api/v1/... publicly (see docs/architecture/api-spec.md), but every
// internal service's own routes (as documented per-service in
// microservices.md) don't know about that prefix.
const apiPrefix = "/api/v1"

// ForwardTo returns a handler that proxies the current request to
// baseURL, preserving path, query string, method, headers, and body.
func ForwardTo(baseURL string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := strings.TrimPrefix(c.OriginalURL(), apiPrefix)
		return proxy.Do(c, baseURL+path)
	}
}
