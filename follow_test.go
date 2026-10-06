package trilha

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// #291: a fragment answered with a redirect is followed in place when the
// client says it can (Trilha-Follow). The message then rides in the header,
// for the toast, and no cookie is left behind for a page that never loads.
func TestFlashEmFragmentoSeguidoVaiNoCabecalho(t *testing.T) {
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	a.Register(Route{Pattern: "/salvar", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		c.Flash("success", "Salvo")
		return c.Redirect("/pronto")
	}}})
	rec := get(t, a, "POST", "/salvar", "", map[string]string{fragmentHeader: "lista", followHeader: "1"})
	if rec.Code != http.StatusNoContent || rec.Header().Get(locationHeader) != "/pronto" {
		t.Fatalf("%d, Trilha-Location %q", rec.Code, rec.Header().Get(locationHeader))
	}
	if rec.Header().Get(flashHeader) == "" {
		t.Fatal("quem segue mostra o aviso num toast: ele vai no cabeçalho")
	}
	if flashCookieOf(t, rec) != nil {
		t.Fatal("um cookie de flash apareceria de novo na próxima página inteira")
	}
}

// RedirectReload is Redirect for when the frame changes (login, logout, the
// organization): without a fragment the same 303; with one, the client is
// told to load the page for real, and the message waits in the cookie for it.
func TestRedirectReload(t *testing.T) {
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	a.Register(Route{Pattern: "/sair", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		c.Flash("info", "Até logo")
		return c.RedirectReload("/entrar")
	}}})
	rec := get(t, a, "POST", "/sair", "", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/entrar" || rec.Header().Get(reloadHeader) != "" {
		t.Fatalf("sem fragmento: %d Location %q Reload %q", rec.Code, rec.Header().Get("Location"), rec.Header().Get(reloadHeader))
	}
	rec = get(t, a, "POST", "/sair", "", map[string]string{fragmentHeader: "lista", followHeader: "1"})
	if rec.Code != http.StatusNoContent || rec.Header().Get(locationHeader) != "/entrar" || rec.Header().Get(reloadHeader) != "1" {
		t.Fatalf("com fragmento: %d Location %q Reload %q", rec.Code, rec.Header().Get(locationHeader), rec.Header().Get(reloadHeader))
	}
	if rec.Header().Get(flashHeader) != "" || flashCookieOf(t, rec) == nil {
		t.Fatal("a página que recarrega é quem mostra o aviso: cookie, não cabeçalho")
	}
	if err := RedirectReload("https://evil.example"); err == nil {
		t.Fatal("RedirectReload saiu do site: é o redirect aberto que Redirect recusa")
	}
}

// #293: PushURL and ReplaceURL tell the client which address rebuilds the
// state a fragment answer just drew. Only on a fragment, only a path of this
// site — like Redirect — and ReplaceURL("") says "leave the address alone".
func TestPushURLEReplaceURL(t *testing.T) {
	var logs bytes.Buffer
	a := New(Config{Env: Dev, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	var call func(c *Ctx)
	a.Register(Route{Pattern: "/painel", Page: func(c *Ctx) (h.Node, error) {
		call(c)
		return h.Div(h.ID("painel")), nil
	}})
	frag := map[string]string{fragmentHeader: "painel"}
	for _, tc := range []struct {
		name       string
		call       func(c *Ctx)
		hdr        map[string]string
		push, repl string
	}{
		{"push", func(c *Ctx) { c.PushURL("/docs/7") }, frag, "/docs/7", ""},
		{"replace", func(c *Ctx) { c.ReplaceURL("/docs?aba=marcos") }, frag, "", "/docs?aba=marcos"},
		{"leave", func(c *Ctx) { c.ReplaceURL("") }, frag, "", "false"},
		{"page", func(c *Ctx) { c.PushURL("/docs/7") }, nil, "", ""},
		{"external", func(c *Ctx) { c.PushURL("https://outro.example/x") }, frag, "", ""},
		{"protocol-relative", func(c *Ctx) { c.ReplaceURL("//outro.example") }, frag, "", ""},
	} {
		call = tc.call
		rec := get(t, a, "GET", "/painel", "", tc.hdr)
		if got := rec.Header().Get(pushURLHeader); got != tc.push {
			t.Errorf("%s: Trilha-Push-Url %q, want %q", tc.name, got, tc.push)
		}
		if got := rec.Header().Get(replaceURLHeader); got != tc.repl {
			t.Errorf("%s: Trilha-Replace-Url %q, want %q", tc.name, got, tc.repl)
		}
	}
	if !strings.Contains(logs.String(), "outro.example") {
		t.Errorf("an address that leaves the site is refused in silence; dev log:\n%s", logs.String())
	}
}
