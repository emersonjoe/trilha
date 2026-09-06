package trilha

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// echoUpstream is the API that already exists: it answers with what it was
// asked, so every test can look at the request from the other side.
func echoUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	return s
}

// upApp is an app with one page and one upstream on /api/.
func upApp(t *testing.T, target string, tune func(*Upstream)) *App {
	t.Helper()
	u := Upstream{Target: target, CSRF: Off}
	if tune != nil {
		tune(&u)
	}
	a := New(Config{Env: Prod, Logger: quiet(), Secret: []byte("segredo-de-teste-com-mais-de-32-bytes!!"),
		Upstreams: map[string]Upstream{"/api/": u}})
	return a
}

// SC-001 — the four methods cross with body and response intact.
func TestUpstreamCarriesEveryMethod(t *testing.T) {
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"method":"` + r.Method + `","path":"` + r.URL.Path + `","body":"` + string(b) + `"}`))
	})
	a := upApp(t, s.URL, nil)
	for _, m := range []string{"GET", "POST", "PUT", "DELETE"} {
		body := "corpo"
		if m == "GET" {
			body = ""
		}
		rec := get(t, a, m, "/api/documents?q=1", body, nil)
		want := `{"method":"` + m + `","path":"/api/documents","body":"` + body + `"}`
		if rec.Code != 200 || rec.Body.String() != want {
			t.Fatalf("%s: %d %s", m, rec.Code, rec.Body.String())
		}
	}
}

// SC-002 — a body larger than MaxBodyBytes crosses: the limit was written for
// handlers, not for a pipe.
func TestUpstreamStreamsBodyPastTheAppLimit(t *testing.T) {
	var got int
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = len(b)
		w.Write([]byte("ok"))
	})
	a := upApp(t, s.URL, nil)
	a.cfg.MaxBodyBytes = 1024
	big := strings.Repeat("x", 4<<20)
	rec := get(t, a, "POST", "/api/upload", big, nil)
	if rec.Code != 200 || got != len(big) {
		t.Fatalf("%d bytes recebidos: %d", rec.Code, got)
	}
}

// SC-003 — the upstream's headers win; the app's security headers stay where
// the upstream said nothing.
func TestUpstreamHeadersWinOverTheApp(t *testing.T) {
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="a.pdf"`)
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Write([]byte("%PDF"))
	})
	a := upApp(t, s.URL, nil)
	rec := get(t, a, "GET", "/api/files/a.pdf", "", nil)
	for k, want := range map[string]string{
		"Content-Type":        "application/pdf",
		"Content-Disposition": `attachment; filename="a.pdf"`,
		"Cache-Control":       "private, max-age=60",
		"ETag":                `"abc"`,
		"X-Frame-Options":     "SAMEORIGIN",
	} {
		if rec.Header().Get(k) != want {
			t.Errorf("%s=%q, queria %q", k, rec.Header().Get(k), want)
		}
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("o que o upstream não disse continua sendo do Trilha")
	}
}

// SC-004 — a route of the app wins over the prefix, which is what makes it
// possible to migrate the API one endpoint at a time.
func TestLocalRouteWinsOverUpstream(t *testing.T) {
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("do upstream")) })
	a := upApp(t, s.URL, nil)
	a.Register(Route{Pattern: "/api/local", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(200, "do app") },
	}})
	if rec := get(t, a, "GET", "/api/local", "", nil); rec.Body.String() != "do app" {
		t.Fatalf("%q", rec.Body.String())
	}
	if rec := get(t, a, "GET", "/api/outra", "", nil); rec.Body.String() != "do upstream" {
		t.Fatalf("%q", rec.Body.String())
	}
	// A method the local route does not serve is a 405 of the app, not a trip
	// to the upstream: the route answered first.
	if rec := get(t, a, "POST", "/api/local", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("405 esperado: %d %s", rec.Code, rec.Body.String())
	}
}

// SC-005 — the target is fixed in the configuration; the request cannot move it.
func TestUpstreamTargetIsNotReachableFromTheRequest(t *testing.T) {
	var seen []string
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Host+r.URL.Path)
		w.Write([]byte("ok"))
	})
	a := upApp(t, s.URL, nil)
	for _, p := range []string{"/api//exemplo.com/x", "/api/x%2f..%2f..%2fetc", `/api/\evil.com`, "/api/x@evil.com"} {
		rec := get(t, a, "GET", p, "", nil)
		if rec.Code >= 500 {
			t.Fatalf("%s: %d", p, rec.Code)
		}
	}
	for _, h := range seen {
		if !strings.HasPrefix(h, strings.TrimPrefix(s.URL, "http://")) {
			t.Fatalf("saiu do alvo: %s", h)
		}
	}
}

// SC-006 — hop-by-hop headers and the browser's own Authorization stop here;
// the request id and the trace cross, so the two logs meet.
func TestUpstreamDropsHopByHopAndTheClientCredential(t *testing.T) {
	var hdr http.Header
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Clone()
		w.Write([]byte("ok"))
	})
	a := upApp(t, s.URL, nil)
	get(t, a, "GET", "/api/x", "", map[string]string{
		"Authorization":       "Bearer do-browser",
		"Proxy-Authorization": "Basic zz",
		"X-Request-ID":        "req-1",
		"Traceparent":         "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	for _, k := range []string{"Authorization", "Proxy-Authorization", "Connection", "Upgrade"} {
		if hdr.Get(k) != "" {
			t.Errorf("%s atravessou: %q", k, hdr.Get(k))
		}
	}
	if hdr.Get("X-Request-ID") != "req-1" || hdr.Get("Traceparent") == "" {
		t.Errorf("request id/trace não atravessaram: %v", hdr)
	}
	if hdr.Get("X-Forwarded-For") == "" || hdr.Get("X-Forwarded-Host") == "" {
		t.Errorf("forwarded ausente: %v", hdr)
	}
}

// SC-007 — Headers is where the session credential is injected, and it runs
// after the browser's own is dropped.
func TestUpstreamInjectsTheSessionCredential(t *testing.T) {
	var auth string
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Write([]byte("ok"))
	})
	a := upApp(t, s.URL, func(u *Upstream) {
		u.Headers = func(c *Ctx, hdr http.Header) {
			if tok, ok := c.Signed("api_token"); ok {
				hdr.Set("Authorization", "Bearer "+tok)
			}
		}
	})
	// The cookie is minted by the app itself, as a login would.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/x", nil)
	c := newCtx(a, &responseWriter{ResponseWriter: rec}, req, kindAPI)
	if err := c.SetSigned("api_token", "jwt-da-sessao", time.Hour); err != nil {
		t.Fatal(err)
	}
	cookie := rec.Header().Get("Set-Cookie")
	get(t, a, "GET", "/api/x", "", map[string]string{"Cookie": strings.SplitN(cookie, ";", 2)[0], "Authorization": "Bearer do-browser"})
	if auth != "Bearer jwt-da-sessao" {
		t.Fatalf("credencial injetada: %q", auth)
	}
}

// SC-008 — a dead upstream is 502 and a slow one is 504, both problem+json:
// the client of /api/ is a script, never a browser in the address bar.
func TestUpstreamFailureIsProblemJSON(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	addr := dead.URL
	dead.Close()
	a := upApp(t, addr, nil)
	rec := get(t, a, "GET", "/api/x", "", map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Header().Get("Content-Type"), "problem+json") {
		t.Fatalf("502: %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	slow := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	b := upApp(t, slow.URL, func(u *Upstream) { u.Timeout = 20 * time.Millisecond })
	rec = get(t, b, "GET", "/api/x", "", nil)
	if rec.Code != http.StatusGatewayTimeout || !strings.Contains(rec.Body.String(), `"status":504`) {
		t.Fatalf("504: %d %s", rec.Code, rec.Body.String())
	}
}

// SC-009 — a write through the upstream needs the CSRF token, like any other
// write of the app; Off is for an API authenticated by key.
func TestUpstreamRequiresCSRFByDefault(t *testing.T) {
	s := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	a := upApp(t, s.URL, func(u *Upstream) { u.CSRF = "" })
	if rec := get(t, a, "POST", "/api/x", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("sem token: %d %s", rec.Code, rec.Body.String())
	}
	// With the double-submit pair the write goes through.
	rec := httptest.NewRecorder()
	c := newCtx(a, &responseWriter{ResponseWriter: rec}, httptest.NewRequest("GET", "/", nil), kindPage)
	tok := c.CSRFToken()
	hdr := map[string]string{"Cookie": CSRFCookie + "=" + tok, CSRFHeader: tok}
	if rec := get(t, a, "POST", "/api/x", "", hdr); rec.Code != 200 {
		t.Fatalf("com token: %d %s", rec.Code, rec.Body.String())
	}
}

// An upstream without a usable target is a complaint at boot, not a 502 an
// hour later — and the app keeps answering everything else.
func TestUpstreamWithBadTargetIsIgnored(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), Upstreams: map[string]Upstream{
		"/api/": {Target: "nao-e-url"},
		"/b/":   {Target: ""},
	}})
	a.applyConfig()
	if len(a.upstreams) != 0 {
		t.Fatalf("%d upstreams", len(a.upstreams))
	}
	if rec := get(t, a, "GET", "/api/x", "", nil); rec.Code != 404 {
		t.Fatalf("%d", rec.Code)
	}
}

// The longest prefix wins, so a subtree can be migrated to another API.
func TestLongestUpstreamPrefixWins(t *testing.T) {
	v1 := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("v1")) })
	v2 := echoUpstream(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("v2")) })
	a := New(Config{Env: Prod, Logger: quiet(), Upstreams: map[string]Upstream{
		"/api/":    {Target: v1.URL, CSRF: Off},
		"/api/v2/": {Target: v2.URL, CSRF: Off},
	}})
	if rec := get(t, a, "GET", "/api/v2/x", "", nil); rec.Body.String() != "v2" {
		t.Fatalf("%q", rec.Body.String())
	}
	if rec := get(t, a, "GET", "/api/x", "", nil); rec.Body.String() != "v1" {
		t.Fatalf("%q", rec.Body.String())
	}
}
