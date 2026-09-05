package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
)

type client struct {
	t   *testing.T
	h   http.Handler
	jar map[string]string
}

func newClient(t *testing.T) *client {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &client{t: t, h: newApp().Handler(), jar: map[string]string{}}
}

func (c *client) do(method, path string, form url.Values) *httptest.ResponseRecorder {
	c.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range c.jar {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		c.jar[ck.Name] = ck.Value
	}
	return rec
}

// csrf loads the form once and returns a form pre-filled with the token.
func (c *client) csrf() url.Values {
	c.do("GET", "/", nil)
	return url.Values{trilha.CSRFField: {c.jar[trilha.CSRFCookie]}}
}

func TestFormRendersConditionalGroups(t *testing.T) {
	c := newClient(t)
	rec := c.do("GET", "/", nil)
	body := rec.Body.String()
	for _, want := range []string{`data-ui-show-when="tipo=pf"`, `data-ui-show-when="tipo=pj"`, `data-ui-show-when="cobranca_diferente"`, `data-ui-show-when="novidades"`, "Ada Lovelace", `<select class="ui-select" id="cidade" name="cidade" disabled`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestValidationRoundTrip(t *testing.T) {
	c := newClient(t)
	f := c.csrf()
	f.Set("tipo", "pj")
	f.Set("nome", "Empresa X")
	f.Set("email", "nao-e-email")
	f.Set("cnpj", "11111111111111")
	f.Set("cep", "123")
	f.Set("uf", "SP")
	f.Set("cidade", "Campinas")
	f.Set("novidades", "on")
	rec := c.do("POST", "/", f)
	if rec.Code != 422 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"CNPJ inválido", "E-mail inválido", "Informe a razão social", "CEP com 8 dígitos", "Informe a rua", "Escolha a frequência",
		`value="Empresa X"`, `value="nao-e-email"`, `value="pj" checked`, `<option value="SP" selected>`, `<option value="Campinas" selected>`, `name="novidades" checked`, `<title>Cadastro de cliente</title>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in\n%s", want, body)
		}
	}
	if strings.Contains(body, "CPF inválido") {
		t.Fatal("PF rules must not apply to PJ")
	}
	if len(clientes.Todos()) != 1 {
		t.Fatal("must not save")
	}
}

func TestValidSubmissionAndHiddenFieldsIgnored(t *testing.T) {
	c := newClient(t)
	f := c.csrf()
	f.Set("tipo", "pf")
	f.Set("nome", "Grace Hopper")
	f.Set("email", "Grace@Example.com")
	f.Set("cpf", "529.982.247-25")
	f.Set("nascimento", "1906-12-09")
	f.Set("cnpj", "04252011000110") // hidden for PF: must be ignored, not validated
	f.Set("cep", "20040-020")
	f.Set("rua", "Av. Rio Branco")
	f.Set("numero", "1")
	f.Set("uf", "RJ")
	f.Set("cidade", "Rio de Janeiro")
	f.Set("novidades", "on")
	f.Set("frequencia", "mensal")
	rec := c.do("POST", "/", f)
	if rec.Code != 303 || rec.Header().Get("Location") != "/?ok=1" {
		t.Fatal(rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	todos := clientes.Todos()
	if len(todos) != 2 || todos[0].Nome != "Grace Hopper" || todos[0].Email != "grace@example.com" || todos[0].CPF != "52998224725" || todos[0].CNPJ != "" || todos[0].Frequencia != "mensal" {
		t.Fatalf("%+v", todos[0])
	}
	rec = c.do("GET", "/?ok=1", nil)
	if !strings.Contains(rec.Body.String(), `class="ui-toast ui-toast-success"`) || !strings.Contains(rec.Body.String(), `data-ui-fade="4000"`) || !strings.Contains(rec.Body.String(), "Grace Hopper") {
		t.Fatal("toast/list missing")
	}
	// Invalid CPF and under-age.
	f.Set("cpf", "12345678900")
	f.Set("nascimento", "2020-01-01")
	rec = c.do("POST", "/", f)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "CPF inválido") || !strings.Contains(rec.Body.String(), "18 anos") {
		t.Fatal(rec.Code)
	}
}

func TestCidadesAPI(t *testing.T) {
	c := newClient(t)
	rec := c.do("GET", "/api/cidades?uf=SP", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"Campinas"`) || rec.Header().Get("Cache-Control") == "" {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := c.do("GET", "/api/cidades?uf=XX", nil); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}

func TestDocumentos(t *testing.T) {
	if !clientes.CPFValido("529.982.247-25") || clientes.CPFValido("111.111.111-11") || clientes.CPFValido("52998224726") {
		t.Fatal("cpf")
	}
	if !clientes.CNPJValido("04.252.011/0001-10") || clientes.CNPJValido("04252011000111") || clientes.CNPJValido("00000000000000") {
		t.Fatal("cnpj")
	}
}
