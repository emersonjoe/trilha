package trilha

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// Spec 178: Vary accumulates. Setting it wiped the "Origin" CORS added, and a
// shared cache could then hand one origin's Access-Control-Allow-Origin to
// another — on the page and on its 304.
func TestVaryKeepsWhatWasThere(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), CORS: CORS{Origins: []string{"https://app.exemplo.com"}}})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		c.ETag("v1")
		return h.P(h.Text("oi")), nil
	}})
	for _, inm := range []string{"", `"v1"`} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://app.exemplo.com")
		if inm != "" {
			req.Header.Set("If-None-Match", inm)
		}
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		vary := strings.Join(rec.Header().Values("Vary"), ",")
		if !strings.Contains(vary, "Origin") || strings.Count(vary, fragmentHeader) != 1 {
			t.Fatalf("status %d: Vary %q, want Origin and %s once", rec.Code, vary, fragmentHeader)
		}
	}
}
