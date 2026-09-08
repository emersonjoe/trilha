package main

import (
	"bytes"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ui"
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

// O fragmento vivo do painel: uma conexão por página (ui.Live), o pedaço que
// escuta pelo nome (ui.On) e a rota do stream atrás do mesmo middleware.
func TestPainelTemFragmentoVivo(t *testing.T) {
	c := cliente(t, "")
	entrar(t, c, "ana@exemplo.com", "segredo-da-ana")

	c.Get("/painel").WantStatus(200).WantContains(
		`data-trilha-live="/painel/eventos"`,
		`id="agora" data-trilha-on="painel:agora"`,
		`src="/ui.live.js?v=`)

	// O mesmo handler responde só o pedaço quando quem pergunta é o script.
	frag := c.Get("/painel", trilha.WithHeader("Trilha-Fragment", "agora")).WantStatus(200)
	if b := frag.Body.String(); strings.Contains(b, "<html") || !strings.Contains(b, `id="agora"`) {
		t.Fatalf("fragmento = %s", b)
	}

	// O stream diz o nome do que mudou e nada mais: o HTML vem do refetch, e a
	// conexão nunca vira canal de dados.
	ev := c.Get("/painel/eventos").WantStatus(200)
	if ct := ev.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	if b := ev.Body.String(); !strings.Contains(b, "event: painel:agora\ndata: \n\n") {
		t.Fatalf("stream = %q", b)
	}
}

// A cópia do kit em public/ é o que o browser baixa; se ela ficar para trás do
// que o pacote embute, o atributo existe e o comportamento não.
func TestUiLiveJSEstaAtualizado(t *testing.T) {
	b, err := os.ReadFile("public/ui.live.js")
	if err != nil {
		t.Fatal(err)
	}
	// A cópia sai com um cabeçalho de versão do `trilha ui` na frente; o que
	// tem de bater é o corpo.
	if !bytes.Contains(b, ui.Asset("ui.live.js")) {
		t.Fatal("public/ui.live.js está desatualizado: rode `trilha ui --force`")
	}
}

// #100 — the matrix is one declaration and it answers in three places: the
// middleware that guards, the 403 that explains, and the grid that edits.
func TestPoliticaGuardaExplicaEEdita(t *testing.T) {
	c := cliente(t, api(t).URL)

	// Anonymous does not reach the screen that edits permissions.
	if rec := c.Get("/permissoes", navegador()); rec.Code == 200 {
		t.Fatal("anônimo abriu a tela de permissões")
	}

	// Ana is an analyst: she may edit reports and may not administer users.
	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(303)
	rec := c.Get("/permissoes", navegador())
	if rec.Code != 403 {
		t.Fatalf("analista em /permissoes → %d, queria 403", rec.Code)
	}
	// The 403 says what was missing, which is what she repeats to whoever
	// administers the app.
	if body := rec.Body.String(); !strings.Contains(body, "usuarios") {
		t.Fatalf("o 403 não diz o que faltou: %q", body)
	}

	// Bia is an admin: she gets the grid, with one field per cell.
	c2 := cliente(t, api(t).URL)
	entrar(t, c2, "bia@exemplo.com", "segredo-da-bia").WantStatus(303)
	c2.Get("/permissoes", navegador()).WantStatus(200).
		WantContains(`name="grant.analista.relatorios"`, `name="grant.leitor.usuarios"`, "Relatórios")
}

// #104 — mudar quem pode o quê é exatamente a ação que alguém pergunta depois.
// A app escreve uma linha; o ator, o IP e a rota vêm da sessão.
func TestAuditoriaRegistraQuemMudouAPermissao(t *testing.T) {
	c := cliente(t, api(t).URL)
	entrar(t, c, "bia@exemplo.com", "segredo-da-bia").WantStatus(303)

	c.Request("POST", "/permissoes", trilha.WithBody("application/x-www-form-urlencoded",
		"grant.admin.usuarios=administrar&grant.admin.relatorios=administrar")).WantStatus(303)

	recs := sessao.Trilha()
	if len(recs) == 0 {
		t.Fatal("nada foi auditado")
	}
	r := recs[len(recs)-1]
	if r.Action != "permissao.alterou" {
		t.Fatalf("ação = %q", r.Action)
	}
	if r.Actor.Email != "bia@exemplo.com" || r.Actor.Via != "session" {
		t.Fatalf("o ator não veio da sessão: %+v", r.Actor)
	}
	if r.Route != "/permissoes" {
		t.Fatalf("rota = %q", r.Route)
	}
}
