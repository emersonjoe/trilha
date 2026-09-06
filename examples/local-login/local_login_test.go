package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// api is the service that already exists: it answers with what it saw, so the
// test can look at the request from the other side of the proxy.
func api(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"path":"` + r.URL.Path + `","auth":"` + r.Header.Get("Authorization") + `"}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func cliente(t *testing.T, apiURL string) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	t.Setenv("API_URL", apiURL)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
}

// navegador is the Accept a browser sends: it is what turns the 401 of a
// protected page into the redirect to the login.
func navegador() trilha.TestOption { return trilha.WithHeader("Accept", "text/html") }

func entrar(t *testing.T, c *trilha.TestClient, email, senha string) *trilha.TestResponse {
	t.Helper()
	return c.Request("POST", "/entrar", trilha.WithBody("application/x-www-form-urlencoded",
		"email="+email+"&senha="+senha))
}

// #62 — the password is checked against the app's own table and the session
// comes out of it; the wrong password says the same thing for both fields.
func TestLoginLocal(t *testing.T) {
	c := cliente(t, "")
	c.Get("/painel", navegador()).WantStatus(http.StatusFound).WantContains("/entrar?next=")

	entrar(t, c, "ana@exemplo.com", "errada").WantStatus(422).WantContains("E-mail ou senha inválidos")
	entrar(t, c, "ninguem@exemplo.com", "segredo-da-ana").WantStatus(422)

	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)
	c.Get("/painel").WantStatus(200).WantContains("Olá, Ana").WantContains("analista")

	c.Request("POST", "/sair").WantStatus(http.StatusSeeOther)
	c.Get("/painel", navegador()).WantStatus(http.StatusFound).WantContains("/entrar?next=")
}

// #62 — a write of this app needs the CSRF token even when it is a route.go:
// app/kind.go says the whole tree is pages.
func TestSairExigeToken(t *testing.T) {
	c := cliente(t, "")
	entrar(t, c, "bia@exemplo.com", "segredo-da-bia").WantStatus(http.StatusSeeOther)
	c.Request("POST", "/sair", trilha.WithoutCSRF()).WantStatus(403)
	c.Get("/painel").WantStatus(200)
}

// #60 + #62 — the proxy carries the call to the API with the credential the
// login stored, and nobody without a session gets one.
func TestUpstreamLevaOTokenDaSessao(t *testing.T) {
	s := api(t)
	c := cliente(t, s.URL)

	c.Get("/api/documentos").WantStatus(200).WantContains(`"auth":""`)

	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)
	rec := c.Get("/api/documentos").WantStatus(200)
	if !strings.Contains(rec.Body.String(), `"auth":"Bearer jwt-da-ana"`) {
		t.Fatalf("credencial da sessão: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"path":"/api/documentos"`) {
		t.Fatalf("caminho: %s", rec.Body.String())
	}

	// The write through the proxy needs the token, like any other write here.
	c.Request("POST", "/api/documentos", trilha.WithoutCSRF()).WantStatus(403)
	c.Request("POST", "/api/documentos").WantStatus(200)
}

// Without API_URL there is no proxy: the example runs alone and /api/ is a 404.
func TestSemAPIURLNaoHaProxy(t *testing.T) {
	cliente(t, "").Get("/api/documentos").WantStatus(404)
}
