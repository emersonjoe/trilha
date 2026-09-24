package cookbook

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cookbook/before"
	"github.com/emersonjoe/trilha/examples/cookbook/crm"
	"github.com/emersonjoe/trilha/examples/cookbook/host"
	"github.com/emersonjoe/trilha/h"
)

// Page is the same blog page after the move: the address is the folder it
// lives in (app/blog/slug_/page.go), the layout is applied for it, the 404
// is an error it returns, and the HTML is a value instead of a string.
func Page(c *trilha.Ctx) (h.Node, error) {
	a, err := ArticleBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return nil, err
	}
	c.SetTitle(a.Title)
	return h.Article(
		h.H1(h.Text(a.Title)),
		h.P(h.Time(h.Attr("datetime", a.Published.Format("2006-01-02")), h.Text(a.Published.Format("2 Jan 2006")))),
	), nil
}

// GET is the same API route: no writer, no encoder, no Content-Type by
// hand. The error carries its own status, and an unexpected one becomes a
// problem+json body with the request id in it.
func GET(c *trilha.Ctx) error {
	a, err := ArticleBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return err
	}
	return c.JSON(200, a)
}

// Front is how the two systems share a process while the move happens: the
// old mux answers what has not been moved yet, and everything it does not
// know falls through to the framework. The old middleware still wraps both,
// so nothing loses its headers halfway.
func Front(mux *http.ServeMux, a *trilha.App) http.Handler {
	mux.Handle("/", a.Handler())
	return before.Secure(mux)
}

// Host is the same move when the app does not live in package main: crm is a
// folder of the binary that already exists, `trilha gen` wrote NewApp into
// the package that folder declares, and mounting it is one line. There is no
// registration file to keep by hand. The nonce goes in on the way past,
// because the app renders its scripts under the host's policy.
func Host(mux *http.ServeMux, nonce func(*http.Request) string) http.Handler {
	mux.Handle("/", crm.NewApp().Handler())
	return before.Secure(withNonce(mux, nonce))
}

// withNonce hands the app the nonce the host already published. Without it
// the app invents one per request, and the policy the browser is enforcing —
// the host's — has never heard of that one.
func withNonce(next http.Handler, nonce func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, host.WithNonce(r, nonce(r)))
	})
}
