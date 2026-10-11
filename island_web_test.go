package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

const islandToken = "0123456789abcdef0123456789abcdef"

func redirectingIslandApp(t *testing.T, files fstest.MapFS) *App {
	t.Helper()
	a := New(Config{Env: Prod, Logger: quiet(), Secret: []byte(islandToken), Public: files})
	a.Register(Route{Pattern: "/api/concluir", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			c.Flash("success", "Done")
			return c.Redirect("/resultado")
		},
	}})
	return a
}

func islandPost(a *App, hdr map[string]string, cookie bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/concluir", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if cookie {
		req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: islandToken})
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

// #302: the island's fetch would follow a 303 in silence and hand island.post
// the HTML of the destination. An island request (Trilha-Island, sent by the
// runtime) gets what a fragment gets: 204 + Trilha-Location, which island.post
// turns into a navigation — and the flash waits in the cookie for that page.
func TestIslandRedirectBecomesLocation(t *testing.T) {
	a := redirectingIslandApp(t, nil)
	rec := islandPost(a, map[string]string{islandHeader: "1", CSRFHeader: islandToken}, true)
	if rec.Code != http.StatusNoContent || rec.Header().Get(locationHeader) != "/resultado" || rec.Header().Get("Location") != "" {
		t.Fatalf("%d, Trilha-Location %q, Location %q", rec.Code, rec.Header().Get(locationHeader), rec.Header().Get("Location"))
	}
	if flashCookieOf(t, rec) == nil {
		t.Fatal("the page that loads shows the message: it has to be in the cookie")
	}
	// Any other client of the same route still gets the 303 it always got.
	rec = islandPost(a, map[string]string{"Authorization": "Bearer x"}, false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/resultado" {
		t.Fatalf("a client that is not an island: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

// #300: a route.go is exempt from CSRF because an API client carries a bearer,
// not the page's cookies. A request that comes with the browser's cookies and
// no Authorization is the page — an island, or another site's form riding on
// the session — and needs the token like a page does. Bearer clients and
// cookie-less callers (webhooks) stay exempt.
func TestRouteWithCookiesNeedsCSRF(t *testing.T) {
	a := redirectingIslandApp(t, nil)
	for _, tc := range []struct {
		name   string
		hdr    map[string]string
		cookie bool
		want   int
	}{
		{"cookie, no token: another site riding on the session", nil, true, http.StatusForbidden},
		{"cookie and the token: the island", map[string]string{CSRFHeader: islandToken}, true, http.StatusSeeOther},
		{"cookie and a wrong token", map[string]string{CSRFHeader: "nope"}, true, http.StatusForbidden},
		{"bearer: an API client", map[string]string{"Authorization": "Bearer abc"}, true, http.StatusSeeOther},
		{"no cookie at all: a webhook", nil, false, http.StatusSeeOther},
	} {
		if got := islandPost(a, tc.hdr, tc.cookie).Code; got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
	// Any cookie of the browser counts, not only the CSRF one: a session cookie
	// alone, without the token's cookie, is refused too.
	req := httptest.NewRequest("POST", "/api/concluir", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: "sessao", Value: "x"})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a session cookie without the token: %d", rec.Code)
	}
}

// #298: a relative import inside an island module resolved to /motor.js with
// no version, and the hour of cache served the old one after a deploy. A page
// that mounts islands gets an import map with the versioned address of every
// module the islands reach, before any of them load.
func TestIslandImportsAreVersioned(t *testing.T) {
	files := fstest.MapFS{
		"ilha.js":        {Data: []byte(`import { f } from "./motor/index.js";\nimport * as u from '../util.js';\nexport default (el) => f(el);`)},
		"motor/index.js": {Data: []byte(`export { f } from "./f.js"; const lazy = () => import("./lazy.js");`)},
		"motor/f.js":     {Data: []byte(`export const f = (x) => x; // import "./not-a-file.js" is ignored`)},
		"motor/lazy.js":  {Data: []byte(`export default 1;`)},
		"util.js":        {Data: []byte(`import "https://cdn.example/x.js"; import "lit";`)},
		"outra.js":       {Data: []byte(`export default 2;`)},
	}
	a := redirectingIslandApp(t, files)
	a.Register(Route{Pattern: "/jogo", Page: func(c *Ctx) (h.Node, error) {
		return h.Html(h.Head(h.Title(h.Text("Jogo"))), h.Body(c.Island("/ilha.js", nil, h.P(h.Text("…"))))), nil
	}})
	a.Register(Route{Pattern: "/sem", Page: func(c *Ctx) (h.Node, error) {
		return h.Html(h.Head(h.Title(h.Text("Sem"))), h.Body(h.P(h.Text("no island")))), nil
	}})
	rec := get(t, a, "GET", "/jogo", "", nil)
	body := rec.Body.String()
	i := strings.Index(body, `<script type="importmap"`)
	if i < 0 || i > strings.Index(body, "<title>") {
		t.Fatalf("no import map at the top of <head>:\n%s", body)
	}
	m := body[i : i+strings.Index(body[i:], "</script>")]
	if !strings.Contains(m, `nonce="`) {
		t.Errorf("the import map is inline: without the nonce the CSP refuses it:\n%s", m)
	}
	for _, p := range []string{"/motor/index.js", "/motor/f.js", "/motor/lazy.js", "/util.js"} {
		if !strings.Contains(m, `"`+p+`":"`+a.Asset(p)+`"`) || !strings.Contains(a.Asset(p), "?v=") {
			t.Errorf("%s is not mapped to %s:\n%s", p, a.Asset(p), m)
		}
	}
	for _, p := range []string{"/outra.js", "not-a-file", "cdn.example", `"lit"`} {
		if strings.Contains(m, p) {
			t.Errorf("%s is in the map, and no island reaches it:\n%s", p, m)
		}
	}
	if strings.Contains(get(t, a, "GET", "/sem", "", nil).Body.String(), "importmap") {
		t.Error("a page without islands got an import map")
	}
	frag := get(t, a, "GET", "/jogo", "", map[string]string{fragmentHeader: "x"}).Body.String()
	if strings.Contains(frag, "importmap") {
		t.Error("a fragment got an import map: it can only go in a document's head")
	}
}
