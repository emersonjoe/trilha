package trilha

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// helperApp is a small app with the two halves the helpers have to cross: a
// page with a form (CSRF, cookie, layout) and an API (JSON in, JSON out).
func helperApp() *App {
	a := New(Config{Env: Dev, Logger: quiet(), Secret: []byte("0123456789abcdef0123456789abcdef")})
	a.Register(Route{Pattern: "/entrar", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		return h.Form(h.Method("post"), CSRFInput(c), h.Input(h.Name("user"))), nil
	}, Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		if err := c.SetSigned("user", c.Form("user"), time.Hour); err != nil {
			return err
		}
		return c.Redirect("/painel")
	}}})
	a.Register(Route{Pattern: "/painel", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		user, ok := c.Signed("user")
		if !ok {
			return nil, Errorf(http.StatusUnauthorized, "sign in first")
		}
		return h.P(h.Text("hello " + user)), nil
	}})
	a.Register(Route{Pattern: "/api/itens", Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			var in struct {
				Name string `json:"name"`
			}
			if err := c.BindJSON(&in); err != nil {
				return err
			}
			return c.JSON(http.StatusCreated, map[string]string{"name": strings.ToUpper(in.Name)})
		},
	}})
	return a
}

// TestRequestAtravessaOApp: a mesma pilha que o ListenAndServe monta.
func TestRequestAtravessaOApp(t *testing.T) {
	a := helperApp()
	TestRequest(t, a, http.MethodGet, "/entrar").
		WantStatus(http.StatusOK).
		WantContains(`id="root"`, CSRFField)

	var out struct {
		Name string `json:"name"`
	}
	TestRequest(t, a, http.MethodPost, "/api/itens", WithJSON(map[string]string{"name": "cadeira"})).
		WantStatus(http.StatusCreated).
		WantHeader("Content-Type", "application/json; charset=utf-8").
		JSON(&out)
	if out.Name != "CADEIRA" {
		t.Errorf("name = %q", out.Name)
	}
}

// TestCSRFAutomatico: o POST de página passa sozinho porque o cliente prova o
// double-submit como o navegador; WithoutCSRF mostra a recusa.
func TestCSRFAutomatico(t *testing.T) {
	a := helperApp()
	TestRequest(t, a, http.MethodPost, "/entrar", WithForm(url.Values{"user": {"ana"}})).
		WantStatus(http.StatusSeeOther).
		WantHeader("Location", "/painel")

	TestRequest(t, a, http.MethodPost, "/entrar", WithForm(url.Values{"user": {"ana"}}), WithoutCSRF()).
		WantStatus(http.StatusForbidden)
}

// TestWithSigned abre a rota protegida sem repetir o login.
func TestWithSigned(t *testing.T) {
	a := helperApp()
	TestRequest(t, a, http.MethodGet, "/painel").WantStatus(http.StatusUnauthorized)
	TestRequest(t, a, http.MethodGet, "/painel", WithSigned("user", "ana")).
		WantStatus(http.StatusOK).
		WantContains("hello ana")
}

// TestClientGuardaOCookie: o login e a página seguinte são duas linhas.
func TestClientGuardaOCookie(t *testing.T) {
	c := NewTestClient(t, helperApp())
	res := c.PostForm("/entrar", url.Values{"user": {"ana"}}).WantStatus(http.StatusSeeOther)
	if res.Cookie("user") == nil {
		t.Fatal("o handler não pôs o cookie assinado")
	}
	c.Get("/painel").WantStatus(http.StatusOK).WantContains("hello ana")
}

// TestPageDevolveONode: o corpo sai com layout, o Node é o que a página
// escreveu — dá para olhar o nó em vez de casar string com HTML.
func TestPageDevolveONode(t *testing.T) {
	r := Route{
		Pattern: "/sobre",
		Layouts: []LayoutFunc{rootLayout},
		Page:    func(c *Ctx) (h.Node, error) { return h.H1(h.Text("Sobre")), nil },
	}
	res := TestPage(t, r, "/sobre").WantStatus(http.StatusOK).WantContains(`<div id="root">`, "Sobre")
	if res.Node == nil {
		t.Fatal("Node vazio")
	}
	got, err := h.Render(res.Node)
	if err != nil {
		t.Fatal(err)
	}
	if got != "<h1>Sobre</h1>" {
		t.Errorf("Node = %q", got)
	}
}

// TestRouteResolveParametro: uma rota sozinha, com os middlewares dela.
func TestRouteResolveParametro(t *testing.T) {
	visto := ""
	r := Route{
		Pattern: "/itens/{id}",
		Methods: map[string]HandlerFunc{
			"GET": func(c *Ctx) error { return c.Text(http.StatusOK, "item "+c.Param("id")) },
		},
		Middlewares: []MiddlewareFunc{func(c *Ctx, next Next) error {
			visto = c.Param("id")
			return next()
		}},
	}
	TestRoute(t, r, http.MethodGet, "/itens/7").WantStatus(http.StatusOK).WantContains("item 7")
	if visto != "7" {
		t.Errorf("o middleware não rodou: %q", visto)
	}
}

// fakeT guarda a falha em vez de derrubar o teste.
type fakeT struct{ msg string }

func (f *fakeT) Helper() {}
func (f *fakeT) Fatalf(format string, args ...any) {
	if f.msg == "" {
		f.msg = fmt.Sprintf(format, args...)
	}
}

// TestFalhaMostraOCorpo: quem lê a falha precisa ver a resposta, não só o
// número.
func TestFalhaMostraOCorpo(t *testing.T) {
	f := &fakeT{}
	TestRequest(f, helperApp(), http.MethodGet, "/painel").WantStatus(http.StatusOK)
	if !strings.Contains(f.msg, "status = 401") || !strings.Contains(f.msg, "sign in first") {
		t.Errorf("mensagem = %q", f.msg)
	}
}
