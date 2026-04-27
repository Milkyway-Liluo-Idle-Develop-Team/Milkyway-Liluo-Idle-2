package server

import "context"

type ridKey struct{}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ridKey{}, id)
}

// RequestIDFromContext returns the per-request id assigned by middleware.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ridKey{}).(string)
	return id, ok
}
