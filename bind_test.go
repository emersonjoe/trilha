package trilha

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

type endereco struct {
	CEP string `form:"cep"`
	UF  string `form:"uf"`
}

type cadastro struct {
	Endereco endereco
	Cobranca endereco  `form:"cob_"`
	Nome     string    `form:"nome"`
	Idade    int       `form:"idade"`
	Peso     float64   `form:"peso"`
	Ativo    bool      `form:"ativo"`
	Tags     []string  `form:"tag"`
	Nasc     time.Time `form:"nasc"`
	Limite   *int      `form:"limite"`
	Ignora   string    `form:"-"`
	Query    string
}

func bindApp(t *testing.T, fn HandlerFunc) *App {
	t.Helper()
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{"POST": fn}})
	return a
}

func postForm(a *App, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func TestBindForm(t *testing.T) {
	var got cadastro
	a := bindApp(t, func(c *Ctx) error {
		if err := c.Bind(&got); err != nil {
			return err
		}
		return c.Text(200, "ok")
	})
	rec := postForm(a, "/api/x?Query=q", "nome=+Ada+&idade=36&peso=61,5&ativo=on&tag=a&tag=b&nasc=1990-12-10&limite=3&Ignora=x&cep=13000000&uf=SP&cob_cep=20000000&cob_uf=RJ")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if got.Nome != "Ada" || got.Idade != 36 || got.Peso != 61.5 || !got.Ativo || len(got.Tags) != 2 || got.Nasc.Year() != 1990 || got.Limite == nil || *got.Limite != 3 || got.Ignora != "" || got.Query != "q" || got.Endereco.CEP != "13000000" || got.Endereco.UF != "SP" || got.Cobranca.CEP != "20000000" || got.Cobranca.UF != "RJ" {
		t.Fatalf("%+v", got)
	}
	// Empty optional values: zero, pointer stays nil, no error.
	got = cadastro{}
	if rec := postForm(a, "/api/x", "nome=B&idade=&limite=&nasc="); rec.Code != 200 || got.Limite != nil || got.Idade != 0 {
		t.Fatal(rec.Code, got)
	}
}

func TestBindErrorsAndJSON(t *testing.T) {
	var got cadastro
	a := bindApp(t, func(c *Ctx) error {
		if err := c.Bind(&got); err != nil {
			return err
		}
		return c.Text(200, "ok")
	})
	rec := postForm(a, "/api/x", "idade=abc&peso=x&ativo=talvez&nasc=ontem")
	if rec.Code != 422 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var body struct {
		Status int               `json:"status"`
		Fields map[string]string `json:"fields"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != 422 || len(body.Fields) != 4 || body.Fields["idade"] != BindInvalid {
		t.Fatal(rec.Body.String())
	}
	req := httptest.NewRequest("POST", "/api/x", strings.NewReader(`{"Nome":"J","Idade":5}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || got.Nome != "J" || got.Idade != 5 {
		t.Fatal(rec.Code, got)
	}
}

func TestFieldErrors(t *testing.T) {
	e := FieldErrors{}
	if e.Any() || e.OrNil() != nil {
		t.Fatal("empty")
	}
	e.Add("b", "x")
	e.Add("a", "y")
	e.Add("a", "ignored")
	if !e.Has("a") || e.Get("a") != "y" || e.Error() != "trilha: validation failed: a: y; b: x" || e.OrNil() == nil {
		t.Fatal(e.Error())
	}
}

func TestRenderWithLayouts(t *testing.T) {
	a := New(Config{Logger: quiet()})
	layout := func(c *Ctx, children h.Node) (h.Node, error) {
		return h.Html(h.Body(h.Div(h.Class("layout"), children))), nil
	}
	a.Register(Route{Pattern: "/form", Layouts: []LayoutFunc{layout},
		Page: func(c *Ctx) (h.Node, error) { return h.Form(CSRFInput(c)), nil },
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
			errs := FieldErrors{"nome": "obrigatório"}
			return c.Render(422, h.Form(h.P(h.Text(errs.Get("nome")))))
		}},
	})
	// GET to obtain the CSRF cookie, then POST with it.
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/form", nil))
	tok := rec.Result().Cookies()[0]
	req := httptest.NewRequest("POST", "/form", strings.NewReader("_csrf="+tok.Value))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(tok)
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `<div class="layout"><form><p>obrigatório</p></form></div>`) {
		t.Fatal(rec.Code, rec.Body.String())
	}
}
