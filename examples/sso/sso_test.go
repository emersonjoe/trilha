package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

type cliente struct {
	t   *testing.T
	h   http.Handler
	jar map[string]string
}

func novo(t *testing.T) *cliente {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "0123456789abcdef0123456789abcdef")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &cliente{t: t, h: newApp().Handler(), jar: map[string]string{}}
}

func (c *cliente) do(method, path string, header http.Header) *httptest.ResponseRecorder {
	c.t.Helper()
	var body io.Reader
	if method == "POST" {
		form := url.Values{}
		if tok, ok := c.jar[trilha.CSRFCookie]; ok {
			form.Set("_csrf", tok)
		}
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "text/html")
	for k, vs := range header {
		req.Header[k] = vs
	}
	for k, v := range c.jar {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 || ck.Value == "" {
			delete(c.jar, ck.Name)
			continue
		}
		c.jar[ck.Name] = ck.Value
	}
	return rec
}

func (c *cliente) get(path string) *httptest.ResponseRecorder { return c.do("GET", path, nil) }

// idpFalso responde só à descoberta: é tudo o que /entrar precisa.
func idpFalso(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
			"end_session_endpoint":   srv.URL + "/logout",
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func configurar(t *testing.T) *httptest.Server {
	t.Helper()
	srv := idpFalso(t)
	t.Setenv("SSO_PROVIDER", "oidc")
	t.Setenv("SSO_ISSUER", srv.URL)
	t.Setenv("SSO_CLIENT_ID", "exemplo")
	t.Setenv("SSO_CLIENT_SECRET", "s3cret")
	t.Setenv("SSO_REDIRECT_URL", "http://localhost:3000/entrar/retorno")
	return srv
}

// Sem provedor o app sobe e explica o que falta, em vez de quebrar.
func TestSemConfiguracaoOAppExplica(t *testing.T) {
	c := novo(t)
	rec := c.get("/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Login indisponível") {
		t.Fatalf("home → %d", rec.Code)
	}
	if rec := c.get("/painel"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("/painel sem provedor → %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = c.do("GET", "/api/eu", http.Header{"Accept": {"application/json"}})
	// O motivo fica no log: uma resposta 5xx não conta detalhes ao cliente.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/api/eu sem provedor → %d %s", rec.Code, rec.Body.String())
	}
}

// /entrar leva ao provedor com PKCE, state e nonce, e guarda os três cookies.
func TestInicioDoLogin(t *testing.T) {
	srv := configurar(t)
	c := novo(t)
	rec := c.get("/entrar")
	if rec.Code != http.StatusFound && rec.Code != http.StatusSeeOther {
		t.Fatalf("/entrar → %d %s", rec.Code, rec.Body.String())
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != mustHost(t, srv.URL) {
		t.Fatalf("destino %q", u)
	}
	q := u.Query()
	for _, k := range []string{"state", "nonce", "code_challenge"} {
		if q.Get(k) == "" {
			t.Errorf("falta %s", k)
		}
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("PKCE fraco: %q", q.Get("code_challenge_method"))
	}
	if q.Get("client_id") != "exemplo" || q.Get("response_type") != "code" {
		t.Errorf("query %v", q)
	}
	if strings.Contains(u.String(), "s3cret") {
		t.Error("segredo do cliente na URL")
	}
	for _, name := range []string{"trilha_oidc_state", "trilha_oidc_nonce", "trilha_oidc_verifier"} {
		if _, ok := c.jar[name]; !ok {
			t.Errorf("cookie %s não foi guardado", name)
		}
	}
}

// Navegador anônimo vai para o login levando o destino; API recebe 401.
func TestAreaProtegida(t *testing.T) {
	configurar(t)
	c := novo(t)
	rec := c.get("/painel")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/entrar?next=%2Fpainel" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = c.do("GET", "/api/eu", http.Header{"Accept": {"application/json"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/api/eu → %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("API redirecionada para HTML: %q", loc)
	}
	rec = c.get("/painel/relatorio")
	if rec.Code != http.StatusFound {
		t.Fatalf("/painel/relatorio anônimo → %d", rec.Code)
	}
}

// Sair é POST com CSRF: ninguém desloga você por um link de outro site.
func TestSairExigeCSRF(t *testing.T) {
	configurar(t)
	c := novo(t)
	if rec := c.do("POST", "/sair", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("/sair sem CSRF → %d", rec.Code)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}
