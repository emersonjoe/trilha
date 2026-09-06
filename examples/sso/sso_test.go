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

// navegador é o Accept que muda a resposta de erro: HTML na página, JSON na
// API. O resto (pote de cookies, CSRF) vem do cliente do trilha.
var navegador = trilha.WithHeader("Accept", "text/html")

type cliente struct{ *trilha.TestClient }

func novo(t *testing.T) *cliente {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "0123456789abcdef0123456789abcdef")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &cliente{trilha.NewTestClient(t, newApp())}
}

func (c *cliente) get(path string) *trilha.TestResponse { return c.Get(path, navegador) }

func (c *cliente) api(path string) *trilha.TestResponse {
	return c.Get(path, trilha.WithHeader("Accept", "application/json"))
}

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
	c.get("/").WantStatus(200).WantContains("Login indisponível")
	c.get("/painel").WantStatus(http.StatusSeeOther).WantHeader("Location", "/")
	// O motivo fica no log: uma resposta 5xx não conta detalhes ao cliente.
	c.api("/api/eu").WantStatus(http.StatusServiceUnavailable)
}

// /entrar leva ao provedor com PKCE, state e nonce, e guarda os três cookies.
func TestInicioDoLogin(t *testing.T) {
	srv := configurar(t)
	c := novo(t)
	res := c.get("/entrar")
	if res.Code != http.StatusFound && res.Code != http.StatusSeeOther {
		t.Fatalf("/entrar → %d %s", res.Code, res.Body.String())
	}
	u, err := url.Parse(res.Header().Get("Location"))
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
		if res.Cookie(name) == nil {
			t.Errorf("cookie %s não foi posto", name)
		}
	}
}

// Navegador anônimo vai para o login levando o destino; API recebe 401.
func TestAreaProtegida(t *testing.T) {
	configurar(t)
	c := novo(t)
	c.get("/painel").WantStatus(http.StatusFound).
		WantHeader("Location", "/entrar?next=%2Fpainel")
	// A API recebe 401, não o HTML do login.
	c.api("/api/eu").WantStatus(http.StatusUnauthorized).WantHeader("Location", "")
	c.get("/painel/relatorio").WantStatus(http.StatusFound)
}

// Sair é POST com CSRF: ninguém desloga você por um link de outro site.
func TestSairExigeCSRF(t *testing.T) {
	configurar(t)
	c := novo(t)
	c.Request("POST", "/sair", navegador, trilha.WithoutCSRF()).WantStatus(http.StatusForbidden)
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}
