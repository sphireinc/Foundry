package auth

import "context"

type identityContextKey struct{}

func withIdentity(ctx context.Context, identity *Identity) context.Context {
	if ctx == nil || identity == nil {
		return ctx
	}
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	if ctx == nil {
		return nil, false
	}
	identity, ok := ctx.Value(identityContextKey{}).(*Identity)
	return identity, ok && identity != nil
}
