package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// fragApp registers one page inside a layout, plus a POST that redirects.
func fragApp(t *testing.T, env Env) *App {
	t.Helper()
	a := New(Config{Env: env, Logger: quiet(), Secret: []byte("0123456789abcdef0123456789abcdef")})
	layout := func(c *Ctx, children h.Node) (h.Node, error) {
		return h.Html(h.Body(h.Header(h.Text("cabeçalho")), children)), nil
	}
	a.Register(Route{Pattern: "/", Kind: KindPage, Layouts: []LayoutFunc{layout},
		Page: func(c *Ctx) (h.Node, error) {
			lista := h.Div(h.ID("lista"), h.Text("itens de "+c.Fragment()))
			if c.Fragment() == "lista" {
				return lista, nil
			}
			return h.Div(h.Text("página"), lista), nil
		},
		Methods: map[string]HandlerFunc{
			"POST": func(c *Ctx) error {
				if c.Form("erro") == "1" {
					return c.Render(422, h.Div(h.ID("lista"), h.Text("erro no campo")))
				}
				return c.Redirect("/pronto")
			},
		}})
	return a
}

func fragGet(t *testing.T, a *App, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	if target != "" {
		req.Header.Set("Trilha-Fragment", target)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

// fragPost envia um formulário com CSRF válido e o cabeçalho de fragmento.
func fragPost(t *testing.T, a *App, body string) *httptest.ResponseRecorder {
	t.Helper()
	const tok = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEF"
	if body != "" {
		body += "&"
	}
	return get(t, a, "POST", "/", body+CSRFField+"="+tok, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Cookie":       CSRFCookie + "=" + tok,
		fragmentHeader: "lista",
	})
}

// FR-001/FR-002: o pedaço vem sem layout e sem envelope de documento.
func TestFragmentSkipsLayoutAndEnvelope(t *testing.T) {
	a := fragApp(t, Prod)
	rec := fragGet(t, a, "lista")
	body := rec.Body.String()
	if body != `<div id="lista">itens de lista</div>` {
		t.Fatalf("fragmento: %q", body)
	}
	if strings.Contains(body, "<!doctype") || strings.Contains(body, "cabeçalho") {
		t.Fatal("o fragmento não pode trazer documento nem layout")
	}
	// A mesma rota, sem o cabeçalho, continua devolvendo a página inteira.
	full := fragGet(t, a, "").Body.String()
	if !strings.Contains(full, "<!doctype") || !strings.Contains(full, "cabeçalho") || !strings.Contains(full, `id="lista"`) {
		t.Fatalf("página: %q", full)
	}
}

// FR-003: sem Vary, um cache serviria o pedaço no lugar da página.
func TestFragmentSetsVary(t *testing.T) {
	a := fragApp(t, Prod)
	for _, target := range []string{"lista", ""} {
		if got := fragGet(t, a, target).Header().Get("Vary"); !strings.Contains(got, "Trilha-Fragment") {
			t.Errorf("target %q → Vary %q", target, got)
		}
	}
}

// FR-002: o script de recarga do dev entraria de novo a cada troca.
func TestFragmentHasNoDevScript(t *testing.T) {
	a := fragApp(t, Dev)
	if body := fragGet(t, a, "lista").Body.String(); strings.Contains(body, "_trilha/events") {
		t.Fatalf("script de dev no fragmento: %q", body)
	}
	if body := fragGet(t, a, "").Body.String(); !strings.Contains(body, "_trilha/events") {
		t.Fatal("a página em dev perdeu o script de recarga")
	}
}

// FR-004: redirecionar dentro de um fragmento é navegar, não trocar uma div.
func TestFragmentRedirectBecomesLocationHeader(t *testing.T) {
	a := fragApp(t, Prod)
	rec := fragPost(t, a, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Trilha-Location"); got != "/pronto" {
		t.Fatalf("Trilha-Location %q", got)
	}
	if rec.Header().Get("Location") != "" {
		t.Fatal("um 204 com Location confundiria o fetch")
	}
}

// FR-002: Ctx.Render num 422 também pula os layouts.
func TestFragmentRenderKeepsStatus(t *testing.T) {
	a := fragApp(t, Prod)
	rec := fragPost(t, a, "erro=1")
	if rec.Code != 422 {
		t.Fatalf("%d", rec.Code)
	}
	if body := rec.Body.String(); body != `<div id="lista">erro no campo</div>` {
		t.Fatalf("%q", body)
	}
}

// O CSRF não ganha caminho novo: sem token, o POST continua barrado.
func TestFragmentStillNeedsCSRF(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), Secret: []byte("0123456789abcdef0123456789abcdef")})
	a.Register(Route{Pattern: "/", Kind: KindPage, Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error { return c.Text(200, "salvou") }}})
	req := httptest.NewRequest("POST", "/", strings.NewReader(""))
	req.Header.Set("Trilha-Fragment", "x")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fragmento sem CSRF → %d", rec.Code)
	}
}
