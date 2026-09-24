package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var campoCSRF = regexp.MustCompile(`<input type="hidden" name="_csrf" value="([^"]+)"`)

// cliente walks the app like a browser: cookies kept, redirects not followed.
func cliente(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// corpo reads a response's body as text.
func corpo(t *testing.T, res *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return string(b)
}

// TestContasPeloNavegador walks the form the way a browser does: it GETs the
// page, expects the token field on it, and posts the form's own fields —
// POST → redirect → GET, the new conta on the list.
func TestContasPeloNavegador(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
	c := cliente(t, srv)

	res, err := c.Get(srv.URL + "/contas")
	if err != nil {
		t.Fatal(err)
	}
	form := corpo(t, res)
	m := campoCSRF.FindStringSubmatch(form)
	if m == nil {
		t.Fatal("o formulário de /contas sai sem o campo _csrf")
	}

	res, err = c.PostForm(srv.URL+"/contas", url.Values{"nome": {"Ana Souza"}, "email": {"ana@exemplo.com"}, "_csrf": {m[1]}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST = %d, want 303\n%s", res.StatusCode, corpo(t, res))
	}
	res, err = c.Get(srv.URL + "/contas")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(corpo(t, res), "Ana Souza") {
		t.Fatal("a listagem não mostra o que foi criado")
	}
}

// TestContasTokenErrado is the half that keeps the CSRF honest: a POST whose
// token does not match the cookie is refused.
func TestContasTokenErrado(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
	c := cliente(t, srv)

	res, err := c.Get(srv.URL + "/contas")
	if err != nil {
		t.Fatal(err)
	}
	corpo(t, res)
	res, err = c.PostForm(srv.URL+"/contas", url.Values{"nome": {"Ana Souza"}, "email": {"ana@exemplo.com"}, "_csrf": {"forjado"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("POST com token forjado = %d, want 403", res.StatusCode)
	}
}

// TestContasNomeVazio is the other half of the same walk: without the name the
// form comes back with 422.
func TestContasNomeVazio(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
	c := cliente(t, srv)

	res, err := c.Get(srv.URL + "/contas")
	if err != nil {
		t.Fatal(err)
	}
	form := corpo(t, res)
	m := campoCSRF.FindStringSubmatch(form)
	if m == nil {
		t.Fatal("o formulário de /contas sai sem o campo _csrf")
	}
	res, err = c.PostForm(srv.URL+"/contas", url.Values{"nome": {""}, "email": {"bia@exemplo.com"}, "_csrf": {m[1]}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST sem nome = %d, want 422", res.StatusCode)
	}
}
