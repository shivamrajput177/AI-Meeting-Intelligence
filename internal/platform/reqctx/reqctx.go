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
	// the user+org rows during signup). See internal/platform/httpserver
	// for the middleware that checks it.
	HeaderInternalToken = "X-Internal-Token"
)
