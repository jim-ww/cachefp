package cachefp

import "context"

type ctxKey struct{}

// Identifier is the result of a successful (or exhausted) identification pass.
type Identifier struct {
	// ID is the raw decoded value. Only meaningful when Identified is true.
	ID uint64
	// Hash is a short, human-readable rendering of ID.
	Hash string
	// Identified reports whether the visitor's browser could be fingerprinted
	// via the cache probe. False means identification was attempted but the
	// browser didn't yield a usable signal (e.g. cache isolation, private
	// browsing, or repeated failures past MaxAttempts).
	Identified bool
}

func withIdentifier(ctx context.Context, id Identifier) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the Identifier attached to the request context by the
// middleware, if any. It is absent for non-GET requests and for requests
// served before the middleware has finished a probe pass.
func FromContext(ctx context.Context) (Identifier, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identifier)
	return id, ok
}
