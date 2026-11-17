package http

import "context"

type identityKey struct{}

// WithIdentity returns a context whose requests carry these headers instead
// of the manager's credentials: a call made for a user (e.g. a command sent
// with the user's ID token) rather than with the host's account.
func WithIdentity(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, identityKey{}, headers)
}

// IdentityFrom returns the headers set by WithIdentity, or nil.
func IdentityFrom(ctx context.Context) map[string]string {
	h, _ := ctx.Value(identityKey{}).(map[string]string)
	return h
}
