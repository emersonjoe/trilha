package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// fakeIDP is an OpenID Connect provider good enough to exercise the whole
// flow: discovery, JWKS, code exchange and a signed ID token. Every knob it
// exposes exists so a test can break one rule at a time.
type fakeIDP struct {
	t        *testing.T
	srv      *httptest.Server
	key      *rsa.PrivateKey
	kid      string
	clientID string
	secret   string

	mu    sync.Mutex
	codes map[string]url.Values

	// knobs
	claims     map[string]any // merged into every token
	issuerLie  string         // discovery announces another issuer
	tokenIss   string         // iss written in the token (default: real)
	tokenAud   string         // aud written in the token (default: clientID)
	expIn      time.Duration  // default 5m
	signWith   *rsa.PrivateKey
	signKid    string
	dropNonce  bool
	endSession bool
}

func newIDP(t *testing.T) *fakeIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIDP{t: t, key: key, kid: "k1", clientID: "app", secret: "s3cret",
		codes: map[string]url.Values{}, expIn: 5 * time.Minute, endSession: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("/jwks", idp.jwks)
	mux.HandleFunc("/token", idp.token)
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func (f *fakeIDP) discovery(w http.ResponseWriter, r *http.Request) {
	iss := f.srv.URL
	if f.issuerLie != "" {
		iss = f.issuerLie
	}
	doc := map[string]string{
		"issuer":                 iss,
		"authorization_endpoint": f.srv.URL + "/authorize",
		"token_endpoint":         f.srv.URL + "/token",
		"jwks_uri":               f.srv.URL + "/jwks",
	}
	if f.endSession {
		doc["end_session_endpoint"] = f.srv.URL + "/logout"
	}
	writeJSON(w, doc)
}

func (f *fakeIDP) jwks(w http.ResponseWriter, r *http.Request) {
	pub := f.key.Public().(*rsa.PublicKey)
	writeJSON(w, map[string]any{"keys": []any{map[string]string{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": f.kid,
		"n": enc64(pub.N.Bytes()), "e": enc64(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

// authorize is what the browser would do at the provider: it validates the
// request and hands back a code.
func (f *fakeIDP) authorize(q url.Values) string {
	f.t.Helper()
	if q.Get("client_id") != f.clientID {
		f.t.Fatalf("client_id errado: %q", q.Get("client_id"))
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		f.t.Fatalf("PKCE ausente: %v", q)
	}
	code := "code-" + q.Get("state")
	f.mu.Lock()
	f.codes[code] = q
	f.mu.Unlock()
	return code
}

func (f *fakeIDP) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", 400)
		return
	}
	f.mu.Lock()
	q, ok := f.codes[r.Form.Get("code")]
	delete(f.codes, r.Form.Get("code")) // um código, um uso
	f.mu.Unlock()
	if !ok {
		writeStatus(w, 400, map[string]string{"error": "invalid_grant"})
		return
	}
	if f.secret != "" && r.Form.Get("client_secret") != f.secret {
		writeStatus(w, 401, map[string]string{"error": "invalid_client"})
		return
	}
	sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
	if enc64(sum[:]) != q.Get("code_challenge") {
		writeStatus(w, 400, map[string]string{"error": "invalid_grant"})
		return
	}
	nonce := q.Get("nonce")
	if f.dropNonce {
		nonce = ""
	}
	writeJSON(w, map[string]string{"token_type": "Bearer", "id_token": f.idToken(nonce)})
}

// idToken signs a token with the current knobs.
func (f *fakeIDP) idToken(nonce string) string {
	now := time.Now()
	claims := map[string]any{
		"iss": pick(f.tokenIss, f.srv.URL),
		"sub": "user-1",
		"aud": pick(f.tokenAud, f.clientID),
		"exp": now.Add(f.expIn).Unix(),
		"iat": now.Unix(),
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	for k, v := range f.claims {
		claims[k] = v
	}
	return f.sign(claims)
}

func (f *fakeIDP) sign(claims map[string]any) string {
	key, kid := f.key, f.kid
	if f.signWith != nil {
		key = f.signWith
	}
	if f.signKid != "" {
		kid = f.signKid
	}
	hdr, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	pay, _ := json.Marshal(claims)
	signed := enc64(hdr) + "." + enc64(pay)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		f.t.Fatal(err)
	}
	return signed + "." + enc64(sig)
}

// provider returns the Provider pointing at this fake.
func (f *fakeIDP) provider() *Provider {
	p := OIDC(f.srv.URL, f.clientID, f.secret, "https://app.exemplo/entrar/retorno")
	p.HTTPClient = f.srv.Client()
	return p
}

// --- app harness ---------------------------------------------------------

// authApp wires the flow into a Trilha app the way an app/ tree would.
func authApp(t *testing.T, a *Auth) *trilha.App {
	return authAppLog(t, a, io.Discard)
}

// authAppLog is authApp with the log captured, for the tests that assert on
// what was written.
func authAppLog(t *testing.T, a *Auth, w io.Writer) *trilha.App {
	t.Helper()
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(w, nil))})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	route("/entrar", a.Start)
	route("/entrar/retorno", a.Callback)
	route("/sair", a.Logout)
	route("/admin", func(c *trilha.Ctx) error {
		u := a.User(c)
		return c.Text(200, "ola "+u.Email+" "+strings.Join(u.Roles, ","))
	}, a.Require())
	route("/admin/relatorio", func(c *trilha.Ctx) error { return c.Text(200, "relatorio") }, a.RequireRole("admin"))
	route("/api/dados", func(c *trilha.Ctx) error { return c.Text(200, "{}") }, a.Require())
	route("/", func(c *trilha.Ctx) error {
		if u := a.User(c); u != nil {
			return c.Text(200, "logado")
		}
		return c.Text(200, "anonimo")
	})
	return app
}

// browser keeps cookies between requests, like a real one.
type browser struct {
	t       *testing.T
	app     *trilha.App
	cookies map[string]string
}

func newBrowser(t *testing.T, app *trilha.App) *browser {
	return &browser{t: t, app: app, cookies: map[string]string{}}
}

func (b *browser) get(path string, header http.Header) *httptest.ResponseRecorder {
	b.t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Accept", "text/html")
	for k, vs := range header {
		req.Header[k] = vs
	}
	for k, v := range b.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	b.app.Handler().ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.cookies, c.Name)
			continue
		}
		b.cookies[c.Name] = c.Value
	}
	return rec
}

// login walks the whole round trip and returns the callback response.
func (b *browser) login(idp *fakeIDP, next string) *httptest.ResponseRecorder {
	b.t.Helper()
	path := "/entrar"
	if next != "" {
		path += "?next=" + url.QueryEscape(next)
	}
	rec := b.get(path, nil)
	if rec.Code != http.StatusFound && rec.Code != http.StatusSeeOther {
		b.t.Fatalf("/entrar → %d", rec.Code)
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		b.t.Fatal(err)
	}
	q := u.Query()
	code := idp.authorize(q)
	return b.get("/entrar/retorno?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(q.Get("state")), nil)
}

// --- small helpers -------------------------------------------------------

func enc64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func writeJSON(w http.ResponseWriter, v any) { writeStatus(w, 200, v) }

func writeStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
