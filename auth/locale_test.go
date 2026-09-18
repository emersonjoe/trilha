package auth

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/emersonjoe/trilha"
)

// #270 — o idioma que a pessoa escolheu fica na sessão, e é a primeira coisa
// que o Ctx.Locale pergunta: ele ganha do Accept-Language do navegador, que
// num posto de atendimento é o do computador do posto e não o dela.
func TestLocaleOfLeADaSessao(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	app := trilha.New(trilha.Config{Env: trilha.Prod,
		Secret:  []byte("0123456789abcdef0123456789abcdef"),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Locales: []string{"pt-BR", "ht", "fr"}})
	app.Config().LocaleOf = a.LocaleOf
	route := func(pattern string, h trilha.HandlerFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}})
	}
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Email: "jean@exemplo.com"})
	})
	route("/idioma", func(c *trilha.Ctx) error {
		if err := a.Update(c, func(u *User) { u.Locale = c.Query("l") }); err != nil {
			return err
		}
		return c.Text(200, c.Locale())
	})
	route("/", func(c *trilha.Ctx) error { return c.Text(200, c.Locale()) })

	b := newBrowser(t, app)
	pt := http.Header{"Accept-Language": {"pt-BR"}}
	// Sem sessão nenhuma: vale o cabeçalho.
	if got := b.get("/", pt).Body.String(); got != "pt-BR" {
		t.Fatalf("sem sessão: %q", got)
	}
	b.get("/entrar", nil)
	// Recém-entrada, sem preferência gravada: continua no cabeçalho.
	if got := b.get("/", pt).Body.String(); got != "pt-BR" {
		t.Fatalf("sessão sem preferência: %q", got)
	}
	b.get("/idioma?l=ht", pt)
	if got := b.get("/", pt).Body.String(); got != "ht" {
		t.Fatalf("a preferência da sessão não ganhou do cabeçalho: %q", got)
	}
	// Uma preferência que a app não oferece mais não trava ninguém.
	b.get("/idioma?l=de", pt)
	if got := b.get("/", pt).Body.String(); got != "pt-BR" {
		t.Fatalf("preferência não oferecida: %q", got)
	}
}
