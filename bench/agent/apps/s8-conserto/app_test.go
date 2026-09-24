package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// client builds the app the way the server does and drives it in memory: no
// port, no browser, and the cookies of the session kept between requests.
func client(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
}

// campoCSRF reads the hidden field out of a rendered form, which is how a
// browser gets the token it posts back. Empty means the form has none.
var campoCSRF = regexp.MustCompile(`<input type="hidden" name="_csrf" value="([^"]+)"`)

// TestContasPeloNavegador walks the form the way a browser does: it GETs the
// page, expects the token field on it, and posts the form's own fields —
// POST → redirect → GET, the new conta on the list.
func TestContasPeloNavegador(t *testing.T) {
	c := client(t)
	form := c.Get("/contas").WantStatus(http.StatusOK).Body.String()
	m := campoCSRF.FindStringSubmatch(form)
	if m == nil {
		t.Fatal("o formulário de /contas sai sem o campo _csrf: o POST de um navegador é recusado")
	}
	c.PostForm("/contas", url.Values{"nome": {"Ana Souza"}, "email": {"ana@exemplo.com"}}).
		WantStatus(http.StatusSeeOther)
	c.Get("/contas").WantStatus(http.StatusOK).WantContains("Ana Souza")
}

// TestContasNomeVazio is the other half of the same walk: without the name the
// form comes back with 422 and the mistake beside the field.
func TestContasNomeVazio(t *testing.T) {
	c := client(t)
	form := c.Get("/contas").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(form, "Nova conta") {
		t.Fatalf("a tela de contas mudou de forma:\n%s", form)
	}
	res := c.PostForm("/contas", url.Values{"nome": {""}, "email": {"bia@exemplo.com"}})
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST sem nome = %d, want 422\n%s", res.Code, res.Body.String())
	}
}
