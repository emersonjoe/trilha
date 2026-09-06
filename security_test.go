package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

func TestDefaultSecurityHeaders(t *testing.T) {
	a := testApp(Dev, nil)
	rec := get(t, a, "GET", "/", "", nil)
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self' 'nonce-", "frame-ancestors 'none'", "form-action 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("csp lacks %q: %s", want, csp)
		}
	}
	// The nonce in the header matches the one on the injected dev script.
	nonce := strings.TrimSuffix(strings.SplitN(strings.SplitN(csp, "'nonce-", 2)[1], "'", 2)[0], "'")
	if nonce == "" || !strings.Contains(rec.Body.String(), `<script nonce="`+nonce+`">`) {
		t.Fatalf("nonce %q not on dev script:\n%s", nonce, rec.Body.String())
	}
	for k, v := range map[string]string{
		"Permissions-Policy":         defaultPermissions,
		"Cross-Origin-Opener-Policy": "same-origin",
		"X-Frame-Options":            "DENY",
		"X-Content-Type-Options":     "nosniff",
	} {
		if rec.Header().Get(k) != v {
			t.Errorf("%s=%q", k, rec.Header().Get(k))
		}
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS must not be sent over plain HTTP")
	}
	// 404 fallback also gets the headers and a fresh nonce.
	rec = get(t, a, "GET", "/nada", "", nil)
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "'nonce-") {
		t.Fatal("fallback without CSP")
	}
	rec2 := get(t, a, "GET", "/", "", nil)
	if rec2.Header().Get("Content-Security-Policy") == csp {
		t.Fatal("nonce must change per request")
	}
}

func TestSecurityOverrides(t *testing.T) {
	a := New(Config{Logger: quiet(), Security: Security{
		CSP: Off, FrameOptions: "SAMEORIGIN", PermissionsPolicy: Off, COOP: Off,
	}})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { return h.Text("x"), nil }})
	rec := get(t, a, "GET", "/", "", nil)
	if rec.Header().Get("Content-Security-Policy") != "" || rec.Header().Get("Permissions-Policy") != "" || rec.Header().Get("Cross-Origin-Opener-Policy") != "" {
		t.Fatalf("headers should be off: %v", rec.Header())
	}
	if rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatal(rec.Header().Get("X-Frame-Options"))
	}
	b := New(Config{Logger: quiet(), Security: Security{
		CSPExtra: map[string][]string{"style-src": {"https://fonts.googleapis.com"}, "font-src": {"https://fonts.gstatic.com"}, "worker-src": {"'self'"}},
	}})
	b.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { return h.Text("x"), nil }})
	csp := get(t, b, "GET", "/", "", nil).Header().Get("Content-Security-Policy")
	for _, want := range []string{"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com", "font-src 'self' https://fonts.gstatic.com", "worker-src 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("missing %q in %s", want, csp)
		}
	}
	c := New(Config{Logger: quiet(), Security: Security{CSP: "script-src 'nonce-{nonce}'"}})
	c.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { return h.Script(NonceAttr(c), h.Raw("1")), nil }})
	rec = get(t, c, "GET", "/", "", nil)
	n := strings.TrimSuffix(strings.TrimPrefix(rec.Header().Get("Content-Security-Policy"), "script-src 'nonce-"), "'")
	if !strings.Contains(rec.Body.String(), `<script nonce="`+n+`">1</script>`) {
		t.Fatal(rec.Body.String())
	}
}

func TestHSTSAndClientIPBehindTrustedProxy(t *testing.T) {
	a := New(Config{Logger: quiet(), TrustedProxies: []string{"10.0.0.0/8", "127.0.0.1"}})
	var seenIP string
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { seenIP = c.ClientIP(); return h.Text("x"), nil }})
	do := func(remote string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = remote
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		return rec
	}
	rec := do("10.1.2.3:5555", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-For": "203.0.113.9, 10.0.0.2"})
	if rec.Header().Get("Strict-Transport-Security") != defaultHSTS || seenIP != "203.0.113.9" {
		t.Fatalf("hsts=%q ip=%q", rec.Header().Get("Strict-Transport-Security"), seenIP)
	}
	rec = do("198.51.100.7:1", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-For": "203.0.113.9"})
	if rec.Header().Get("Strict-Transport-Security") != "" || seenIP != "198.51.100.7" {
		t.Fatalf("untrusted peer must be ignored: hsts=%q ip=%q", rec.Header().Get("Strict-Transport-Security"), seenIP)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tlsState
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("direct TLS must send HSTS")
	}
}

func TestRateLimit(t *testing.T) {
	a := New(Config{Logger: quiet(), RateLimit: RateLimit{RPS: 10, Burst: 3}})
	now := time.Unix(1000, 0)
	a.limiter.now = func() time.Time { return now }
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.Text(200, "ok") }}})
	do := func(ip string) int {
		req := httptest.NewRequest("GET", "/api/x", nil)
		req.RemoteAddr = ip + ":1"
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		if rec.Code == 429 && rec.Header().Get("Retry-After") == "" {
			t.Fatal("429 without Retry-After")
		}
		return rec.Code
	}
	for i := 0; i < 3; i++ {
		if do("1.1.1.1") != 200 {
			t.Fatalf("request %d blocked", i)
		}
	}
	if do("1.1.1.1") != 429 {
		t.Fatal("4th immediate request should be limited")
	}
	if do("2.2.2.2") != 200 {
		t.Fatal("other client has its own bucket")
	}
	now = now.Add(150 * time.Millisecond) // 1.5 tokens refilled
	if do("1.1.1.1") != 200 || do("1.1.1.1") != 429 {
		t.Fatal("refill")
	}
	var events []SecurityEvent
	b := New(Config{Logger: quiet(), OnSecurityEvent: func(e SecurityEvent) { events = append(events, e) }})
	b.Register(Route{Pattern: "/x", Middlewares: []MiddlewareFunc{Limit(1, 1)}, Page: func(c *Ctx) (h.Node, error) { return h.Text("x"), nil }})
	get(t, b, "GET", "/x", "", nil)
	if rec := get(t, b, "GET", "/x", "", nil); rec.Code != 429 || !strings.Contains(rec.Body.String(), "429") {
		t.Fatalf("per-route limit: %d", rec.Code)
	}
	if len(events) != 1 || events[0].Kind != "rate" || events[0].Status != 429 || events[0].Path != "/x" {
		t.Fatalf("%+v", events)
	}
}

func TestSignedCookies(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	a := New(Config{Logger: quiet(), Secret: key})
	a.Register(Route{Pattern: "/entrar", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		if err := c.SetSigned("sessao", "ana", time.Hour); err != nil {
			return err
		}
		return c.Text(200, "ok")
	}}})
	a.Register(Route{Pattern: "/quem", Page: func(c *Ctx) (h.Node, error) {
		v, ok := c.Signed("sessao")
		if !ok {
			return nil, Errorf(401, "sem sessão")
		}
		return h.Text("olá " + v), nil
	}})
	rec := get(t, a, "POST", "/entrar", "", nil)
	ck := rec.Result().Cookies()[0]
	if ck.Name != "sessao" || !ck.HttpOnly || ck.Secure || ck.SameSite != 2 || ck.MaxAge != 3600 {
		t.Fatalf("%+v", ck)
	}
	if rec := get(t, a, "GET", "/quem", "", map[string]string{"Cookie": "sessao=" + ck.Value}); rec.Code != 200 || !strings.Contains(rec.Body.String(), "olá ana") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	tampered := strings.Replace(ck.Value, "|", "x|", 1)
	if rec := get(t, a, "GET", "/quem", "", map[string]string{"Cookie": "sessao=" + tampered}); rec.Code != 401 {
		t.Fatal("tampered accepted")
	}
	if rec := get(t, a, "GET", "/quem", "", nil); rec.Code != 401 {
		t.Fatal("missing accepted")
	}
	// Expired.
	tok, _ := a.signer.Sign("ana", time.Now().Add(-time.Second))
	if _, ok := a.signer.Verify(tok, time.Now()); ok {
		t.Fatal("expired accepted")
	}
	// Rotation: the previous key still verifies, the new one signs.
	newKey := []byte(strings.Repeat("n", 32))
	b := New(Config{Logger: quiet(), Secret: newKey, PreviousSecret: key})
	if v, ok := b.signer.Verify(ck.Value, time.Now()); !ok || v != "ana" {
		t.Fatal("previous key must verify")
	}
	other := New(Config{Logger: quiet(), Secret: newKey})
	if _, ok := other.signer.Verify(ck.Value, time.Now()); ok {
		t.Fatal("foreign key accepted")
	}
	// Prod without secret: SetSigned fails, app still serves.
	p := New(Config{Logger: quiet(), Env: Prod})
	p.Register(Route{Pattern: "/s", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.SetSigned("x", "y", time.Hour) }}})
	if rec := get(t, p, "GET", "/s", "", nil); rec.Code != 500 {
		t.Fatal(rec.Code)
	}
	// Dev without secret: ephemeral key works.
	d := New(Config{Logger: quiet(), Env: Dev})
	if _, err := d.signer.Sign("x", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestSecretFromEnv(t *testing.T) {
	t.Setenv("TRILHA_SECRET", "curta")
	if cfg := ConfigFromEnv(); cfg.Secret != nil {
		t.Fatal("short secret must be rejected")
	}
	t.Setenv("TRILHA_SECRET", strings.Repeat("x", 40))
	if cfg := ConfigFromEnv(); len(cfg.Secret) != 40 {
		t.Fatal(len(cfg.Secret))
	}
	t.Setenv("TRILHA_SECRET", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // 32 zero bytes
	if cfg := ConfigFromEnv(); len(cfg.Secret) != 32 {
		t.Fatal(len(cfg.Secret))
	}
	t.Setenv("TRILHA_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.1.1")
	if cfg := ConfigFromEnv(); len(cfg.TrustedProxies) != 2 {
		t.Fatal(cfg.TrustedProxies)
	}
}

func TestSecurityEventsForCSRFAndAuth(t *testing.T) {
	var events []SecurityEvent
	a := New(Config{Logger: quiet(), OnSecurityEvent: func(e SecurityEvent) { events = append(events, e) }})
	a.Register(Route{Pattern: "/f", Page: func(c *Ctx) (h.Node, error) { return h.Text("x"), nil },
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return nil }}})
	a.Register(Route{Pattern: "/p", Page: func(c *Ctx) (h.Node, error) { return nil, Errorf(403, "no") }})
	a.Register(Route{Pattern: "/boom", Page: func(c *Ctx) (h.Node, error) { panic("x") }})
	get(t, a, "POST", "/f", "", nil)
	get(t, a, "GET", "/p", "", nil)
	get(t, a, "GET", "/boom", "", nil)
	get(t, a, "GET", "/f", "", nil)
	if len(events) != 3 || events[0].Kind != "csrf" || events[1].Kind != "auth" || events[2].Kind != "panic" {
		t.Fatalf("%+v", events)
	}
}

// Spec 046 (#52): Delegated says the headers belong to whoever is in front.
// It is not six Offs: it also drops nosniff, which had no Off, and it keeps
// what the host wrote before the app ran.
func TestSecurityDelegated(t *testing.T) {
	a := New(Config{Logger: quiet(), Security: Security{Delegated: true}})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { return h.Text("x"), nil }})

	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Security-Policy", "default-src 'self' cdn.farol")
	req := httptest.NewRequest("GET", "/", nil)
	a.Handler().ServeHTTP(rec, req)

	for _, k := range []string{
		"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy",
		"Permissions-Policy", "Cross-Origin-Opener-Policy", "Strict-Transport-Security",
	} {
		if v := rec.Header().Get(k); v != "" {
			t.Errorf("%s=%q: the app must not write it when delegated", k, v)
		}
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'self' cdn.farol" {
		t.Fatalf("the host policy was overwritten: %q", got)
	}
}

// Spec 046 (#52): with Security.Nonce the app stops drawing its own and
// answers with the one the host already put in its policy — which is what
// makes NonceAttr true inside a host.
func TestSecurityNonceFromHost(t *testing.T) {
	a := New(Config{Logger: quiet(), Security: Security{
		Delegated: true,
		Nonce:     func(r *http.Request) string { return r.Header.Get("X-Farol-Nonce") },
	}})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		return h.Script(NonceAttr(c), h.Raw("void 0")), nil
	}})

	rec := get(t, a, "GET", "/", "", map[string]string{"X-Farol-Nonce": "abc123"})
	if !strings.Contains(rec.Body.String(), `<script nonce="abc123">`) {
		t.Fatalf("the host nonce did not reach the attribute:\n%s", rec.Body.String())
	}

	// No nonce for this request: no attribute at all, instead of nonce="".
	rec = get(t, a, "GET", "/", "", nil)
	if strings.Contains(rec.Body.String(), "nonce=") {
		t.Fatalf("empty nonce must render no attribute:\n%s", rec.Body.String())
	}
}
