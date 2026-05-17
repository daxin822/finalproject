package auth

import "context"

type ctxKey int

const claimsKey ctxKey = 1

func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func FromContext(ctx context.Context) (Claims, bool) {
	v := ctx.Value(claimsKey)
	if v == nil {
		return Claims{}, false
	}
	cl, ok := v.(Claims)
	return cl, ok
}
