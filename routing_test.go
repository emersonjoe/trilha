package trilha

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

// langApp é o app da issue #203: uma rota de idioma na raiz, que no mux do Go
// casa com qualquer caminho de um segmento — inclusive com o nome de um
// arquivo de Public.
func langApp(t *testing.T, public fstest.MapFS, logger *slog.Logger) *App {
	t.Helper()
	if logger == nil {
		logger = quiet()
	}
	a := New(Config{Env: Prod, Logger: logger, Public: public})
	a.Register(Route{Pattern: "/{lang}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("lang:" + c.Param("lang")), nil
	}})
	a.Register(Route{Pattern: "/manifest.webmanifest", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("route wins"), nil
	}})
	return a
}

// #203: a folha de estilo tem de sair, mesmo com /{lang} registrado antes dela.
func TestStaticAnswersBeforeWildcardRoute(t *testing.T) {
	pub := fstest.MapFS{
		"ui.css":               {Data: []byte("body{}")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	a := langApp(t, pub, nil)
	rec := get(t, a, "GET", "/ui.css", "", nil)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("o arquivo não saiu: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if body := rec.Body.String(); body != "body{}" {
		t.Fatalf("corpo %q", body)
	}
	// O arquivo sai com os mesmos cabeçalhos que teria saindo do fallback.
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("cabeçalhos: %v", rec.Header())
	}
	if rec := get(t, a, "HEAD", "/ui.css", "", nil); rec.Code != 200 {
		t.Fatalf("HEAD: %d", rec.Code)
	}
	// A rota continua respondendo o que é dela.
	if rec := get(t, a, "GET", "/en", "", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "lang:en") {
		t.Fatalf("a rota parou de responder: %d %s", rec.Code, rec.Body.String())
	}
	// Rota literal ganha do arquivo de mesmo endereço: quem escreveu o
	// endereço à mão escreveu o endereço.
	if rec := get(t, a, "GET", "/manifest.webmanifest", "", nil); !strings.Contains(rec.Body.String(), "route wins") {
		t.Fatalf("a rota literal tem de ganhar: %s", rec.Body.String())
	}
}

// Um arquivo de Mounts passa na frente do curinga pela mesma razão.
func TestMountAnswersBeforeWildcardRoute(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(),
		Mounts: map[string]fs.FS{"/icons": fstest.MapFS{"logo.svg": {Data: []byte("<svg/>")}}}})
	a.Register(Route{Pattern: "/icons/{name}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("page:" + c.Param("name")), nil
	}})
	if rec := get(t, a, "GET", "/icons/logo.svg", "", nil); rec.Code != 200 || rec.Body.String() != "<svg/>" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, a, "GET", "/icons/outro", "", nil); !strings.Contains(rec.Body.String(), "page:outro") {
		t.Fatalf("a rota responde o que não é arquivo: %s", rec.Body.String())
	}
}

// #203: Asset só pode devolver URL que chega no arquivo; o caso que sobra (uma
// rota literal com o mesmo endereço) sai no log.
func TestAssetWarnsWhenALiteralRouteOwnsThePath(t *testing.T) {
	var buf strings.Builder
	pub := fstest.MapFS{
		"ui.css":               {Data: []byte("body{}")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	a := langApp(t, pub, slog.New(slog.NewTextHandler(&buf, nil)))
	if got := a.Asset("/ui.css"); !strings.HasPrefix(got, "/ui.css?v=") {
		t.Fatalf("%q", got)
	}
	if strings.Contains(buf.String(), "ui.css") {
		t.Fatalf("aviso indevido: %s", buf.String())
	}
	a.Asset("/manifest.webmanifest")
	if !strings.Contains(buf.String(), "manifest.webmanifest") {
		t.Fatalf("o aviso precisa dizer qual endereço está tomado: %s", buf.String())
	}
}

// nextApp são os dois padrões da issue #204, que o http.ServeMux recusa
// registrar juntos. reversed inverte a ordem de registro para provar que a
// resposta não depende dela.
func nextApp(t *testing.T, reversed bool) *App {
	t.Helper()
	a := New(Config{Env: Prod, Logger: quiet(), Secret: []byte("0123456789abcdef0123456789abcdef")})
	org := Route{Pattern: "/o/{slug}/login", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("org:" + c.Param("slug")), nil
	}, Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return c.Text(200, "entrou") }}}
	deck := Route{Pattern: "/{lang}/cards/{deckId}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("deck:" + c.Param("lang") + ":" + c.Param("deckId") + ":" + c.Pattern()), nil
	}}
	if reversed {
		a.Register(deck)
		a.Register(org)
	} else {
		a.Register(org)
		a.Register(deck)
	}
	return a
}

// #204: registrar os dois não pode explodir, e o segmento estático ganha do
// curinga posição por posição.
func TestSegmentSpecificityReplacesThePanic(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		a := nextApp(t, reversed)
		if rec := get(t, a, "GET", "/o/cards/login", "", nil); !strings.Contains(rec.Body.String(), "org:cards") {
			t.Fatalf("reversed=%v: /o/cards/login devia ser da primeira rota: %d %s", reversed, rec.Code, rec.Body.String())
		}
		rec := get(t, a, "GET", "/en/cards/mazo-1", "", nil)
		if !strings.Contains(rec.Body.String(), "deck:en:mazo-1:/{lang}/cards/{deckId}") {
			t.Fatalf("reversed=%v: %d %s", reversed, rec.Code, rec.Body.String())
		}
		if rec := get(t, a, "GET", "/o/qualquer/login", "", nil); !strings.Contains(rec.Body.String(), "org:qualquer") {
			t.Fatalf("reversed=%v: %s", reversed, rec.Body.String())
		}
		if rec := get(t, a, "GET", "/pt/x", "", nil); rec.Code != 404 {
			t.Fatalf("reversed=%v: caminho de ninguém: %d", reversed, rec.Code)
		}
	}
}

// A rota despachada pelo kit responde como qualquer outra: HEAD pelo GET, 405
// com Allow, barra final redirecionando.
func TestConflictingRouteKeepsTheMuxContract(t *testing.T) {
	a := nextApp(t, true)
	if rec := get(t, a, "HEAD", "/o/acme/login", "", nil); rec.Code != 200 {
		t.Fatalf("HEAD: %d", rec.Code)
	}
	rec := get(t, a, "DELETE", "/o/acme/login", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("405: %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, HEAD, POST" {
		t.Fatalf("Allow: %q", allow)
	}
	rec = get(t, a, "GET", "/o/acme/login/", "", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/o/acme/login" {
		t.Fatalf("barra final: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if _, ok := a.Route("/o/{slug}/login"); !ok {
		t.Fatal("a rota tem de aparecer em Route()")
	}
	if ms := a.Routes()["/{lang}/cards/{deckId}"]; len(ms) != 1 || ms[0] != "GET" {
		t.Fatalf("Routes(): %v", ms)
	}
}

// Padrão repetido é bug do arquivo gerado e continua explodindo: o conflito de
// especificidade não pode virar desculpa para engolir uma rota.
func TestDuplicatePatternStillPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("registrar o mesmo padrão duas vezes tem de explodir")
		}
	}()
	a := New(Config{Logger: quiet()})
	r := Route{Pattern: "/blog/{slug}", Page: func(c *Ctx) (h.Node, error) { return h.Text("x"), nil }}
	a.Register(r)
	a.Register(r)
}

// A comparação é posição por posição, da esquerda para a direita: o "docs"
// literal decide antes de qualquer coisa que venha depois dele, mesmo quando o
// que vem depois é um {path...}. Empate em todas as posições comparáveis vai
// para o padrão que soletra mais da URL.
func TestSpecificityIsReadLeftToRight(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet()})
	a.Register(Route{Pattern: "/docs/{path...}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("catch:" + c.Param("path")), nil
	}})
	a.Register(Route{Pattern: "/{secao}/guia/{id}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("guia:" + c.Param("secao") + ":" + c.Param("id")), nil
	}})
	if rec := get(t, a, "GET", "/docs/guia/7", "", nil); !strings.Contains(rec.Body.String(), "catch:guia/7") {
		t.Fatalf("o literal da primeira posição decide: %s", rec.Body.String())
	}
	if rec := get(t, a, "GET", "/x/guia/7", "", nil); !strings.Contains(rec.Body.String(), "guia:x:7") {
		t.Fatalf("%s", rec.Body.String())
	}
	if rec := get(t, a, "GET", "/docs/a/b/c", "", nil); !strings.Contains(rec.Body.String(), "catch:a/b/c") {
		t.Fatalf("o {path...} leva as barras: %s", rec.Body.String())
	}

	b := New(Config{Env: Prod, Logger: quiet()})
	b.Register(Route{Pattern: "/arquivos/{path...}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("catch:" + c.Param("path")), nil
	}})
	b.Register(Route{Pattern: "/arquivos/{ano}/{mes}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("mes:" + c.Param("ano") + "/" + c.Param("mes")), nil
	}})
	if rec := get(t, b, "GET", "/arquivos/2026/09", "", nil); !strings.Contains(rec.Body.String(), "mes:2026/09") {
		t.Fatalf("o padrão que soletra mais ganha do {path...}: %s", rec.Body.String())
	}
	if rec := get(t, b, "GET", "/arquivos/2026", "", nil); !strings.Contains(rec.Body.String(), "catch:2026") {
		t.Fatalf("%s", rec.Body.String())
	}
}
