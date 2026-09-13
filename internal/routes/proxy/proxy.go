// Package proxy is the API Gateway's reverse-proxy layer, built on the
// standard library's net/http/httputil.ReverseProxy. There is no protocol
// translation here — the gateway forwards the exact REST request it
// received to the target service's own REST API, per
// docs/architecture/microservices.md §"Internal Communication".
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// apiPrefix is stripped before forwarding — the gateway exposes
// /api/v1/... publicly (see docs/architecture/api-spec.md), but every
// internal service's own routes (as documented per-service in
// microservices.md) don't know about that prefix.
const apiPrefix = "/api/v1"

// ForwardTo returns a handler that proxies the current request to
// baseURL, preserving method, headers, query string, and body — anything
// httputil.ReverseProxy already does for you. baseURL is one of this
// project's own service URLs (from config), so a parse failure here is a
// startup misconfiguration, not something to recover from per-request.
func ForwardTo(baseURL string) http.Handler {
	target, err := url.Parse(baseURL)
	if err != nil {
		panic("proxy: invalid service base URL " + baseURL + ": " + err.Error())
	}

	return &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = target.Scheme
			r.URL.Host = target.Host
			r.URL.Path = strings.TrimPrefix(r.URL.Path, apiPrefix)
			r.Host = target.Host
		},
	}
}
