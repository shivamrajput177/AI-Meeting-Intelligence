// Package reqctx holds the small set of request-scoped values every
// service needs to read: who is calling (user_id, org_id, role) and how
// to correlate this request across service/log boundaries (request id).
//
// These are populated once — by the API Gateway's auth middleware for a
// gateway-fronted request, or by a service's own internal-token middleware
// for a direct service-to-service call — and read everywhere downstream
// via the accessors below, never by re-parsing a header ad hoc.
package reqctx

import "context"

type ctxKey int

const (
	keyUserID ctxKey = iota
	keyOrgID
	keyRole
	keyRequestID
	keyTraceparent
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

func UserID(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}

func WithOrgID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, keyOrgID, orgID)
}

func OrgID(ctx context.Context) string {
	v, _ := ctx.Value(keyOrgID).(string)
	return v
}

func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, keyRole, role)
}

func Role(ctx context.Context) string {
	v, _ := ctx.Value(keyRole).(string)
	return v
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// WithTraceparent/Traceparent carry a W3C traceparent string
// ("00-{trace-id}-{span-id}-{flags}") across the Kafka async boundary —
// see shared/kafkax's doc comment for how it's generated, propagated
// through Kafka message headers, and why Phase 2.7 hand-rolls this format
// instead of pulling in the full OTel SDK. A Kafka consumer sets this on
// ctx (via kafkax.ChildTraceparent off the fetched message's headers)
// before calling its usecase, so that usecase's own eventual
// publisher.Publish* call — which already receives that same ctx — picks
// it back up transparently through kafkax.Publish, with no per-call-site
// plumbing needed anywhere in between.
func WithTraceparent(ctx context.Context, traceparent string) context.Context {
	return context.WithValue(ctx, keyTraceparent, traceparent)
}

func Traceparent(ctx context.Context) string {
	v, _ := ctx.Value(keyTraceparent).(string)
	return v
}

// HTTP header names these values travel under between the gateway and a
// service, or between two services, since there's no gRPC metadata
// channel in this design (see docs/architecture/microservices.md
// §"Internal Communication").
const (
	HeaderUserID    = "X-User-Id"
	HeaderOrgID     = "X-Org-Id"
	HeaderRole      = "X-Role"
	HeaderRequestID = "X-Request-Id"
	// HeaderInternalToken authenticates service-to-service calls that
	// aren't on behalf of any particular user (e.g. Auth Service creating
	// the user+org rows during signup). See shared/httpserver
	// for the middleware that checks it.
	HeaderInternalToken = "X-Internal-Token"
)
