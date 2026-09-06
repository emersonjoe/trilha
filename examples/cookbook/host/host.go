// Package host belongs to the binary that already exists, not to the app it
// mounts: it is where the host publishes what it already decided, so the
// embedded app can ask instead of deciding a second time.
package host

import (
	"context"
	"net/http"
)

type nonceKey struct{}

// WithNonce is what the host's middleware calls once per request, with the
// nonce it already wrote into its own Content-Security-Policy.
func WithNonce(r *http.Request, nonce string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), nonceKey{}, nonce))
}

// Nonce reads it back. It comes from the context and never from a header: a
// nonce a client could set is not a nonce. An empty answer renders no
// attribute at all, which beats one no policy has heard of.
func Nonce(r *http.Request) string {
	n, _ := r.Context().Value(nonceKey{}).(string)
	return n
}
