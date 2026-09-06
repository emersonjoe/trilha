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
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
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

func (c *client) csrf() url.Values {
	c.do("GET", "/", nil)
	return url.Values{trilha.CSRFField: {c.jar[trilha.CSRFCookie]}}
}

func TestAggregation(t *testing.T) {
	plano.Seed()
	desp, _ := plano.Get("2")
	var soma, somaReal int64
	for _, f := range desp.Filhos {
		soma += plano.Orcado(f, "2026-09")
		somaReal += plano.Realizado(f, "2026-09")
	}
	if plano.Orcado(desp, "2026-09") != soma || plano.Realizado(desp, "2026-09") != somaReal || soma == 0 {
		t.Fatal("synthetic account must equal the sum of its children")
	}
	sal, _ := plano.Get("2.1.1")
	if !sal.Analitica() || sal.Nivel() != 3 || len(sal.Caminho()) != 3 || plano.Orcado(sal, "2026-09") != 5000000 {
		t.Fatal(sal)
	}
	if plano.Variacao(100, 125) != 25 || plano.Variacao(0, 5) != 0 || plano.Money(123456) != "R$ 1.234,56" || plano.Money(-5) != "-R$ 0,05" {
		t.Fatal("helpers")
	}
	for in, want := range map[string]int64{"1.234,56": 123456, "1234.56": 123456, "1234": 123400, "R$ 10,00": 1000} {
		if v, err := plano.ParseMoney(in); err != nil || v != want {
			t.Fatal(in, v, err)
		}
	}
	r := plano.Resumir("2026-09")
	if r.ResultadoReal != r.ReceitaReal-r.DespesaReal || r.PctReceita <= 0 {
		t.Fatalf("%+v", r)
	}
}

func TestOverviewAndDrillDown(t *testing.T) {
	c := newClient(t)
	rec := c.do("GET", "/?mes=2026-08", nil)
	body := rec.Body.String()
	for _, want := range []string{"<title>Orçamento ago/2026</title>", `href="/contas/2?mes=2026-08"`, `data-depth="1"`, `href="/contas/2.3?mes=2026-08"`, "Receita realizada", `<option value="2026-08" selected>`, `<dialog class="ui-dialog" id="novo"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(body, `href="/contas/2.3.1?`) {
		t.Fatal("overview shows only two levels")
	}
	rec = c.do("GET", "/contas/2.3?mes=2026-08", nil)
	body = rec.Body.String()
	for _, want := range []string{"<title>2.3 Marketing</title>", `aria-label="breadcrumb"`, `href="/contas/2?mes=2026-08">2 Despesas</a>`, `href="/contas/2.3.2?mes=2026-08"`, "Contas filhas", "ui-badge ui-badge-destructive"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	rec = c.do("GET", "/contas/2.3.2?mes=2026-08", nil)
	if !strings.Contains(rec.Body.String(), "Lançamentos") || !strings.Contains(rec.Body.String(), "Eventos (1ª quinzena)") || !strings.Contains(rec.Body.String(), "<tfoot>") {
		t.Fatal("analytic account must list entries")
	}
	if rec := c.do("GET", "/contas/9.9", nil); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	if rec := c.do("GET", "/?mes=lixo", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "set/2026") {
		t.Fatal("invalid month falls back to default")
	}
}

func TestEntryValidationAndPost(t *testing.T) {
	c := newClient(t)
	f := c.csrf()
	f.Set("conta", "2.1") // synthetic
	f.Set("valor", "-5")
	f.Set("data", "")
	f.Set("voltar", "/contas/2.1?mes=2026-09")
	rec := c.do("POST", "/lancamentos", f)
	if rec.Code != 422 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"conta analítica", "maior que zero", "obrigatório", `value="-5"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
	// Issue #27: as regras de forma vêm da tag, em português, e a regra de
	// negócio (conta analítica) continua em Go — as duas na mesma resposta.
	if body := rec.Body.String(); strings.Contains(body, "required") || strings.Contains(body, "must ") {
		t.Fatal("mensagem em inglês num app em português")
	}
	// Bind conversion error (bad date) surfaces as a field error too.
	f.Set("data", "ontem")
	if rec := c.do("POST", "/lancamentos", f); rec.Code != 422 || !strings.Contains(rec.Body.String(), trilha.BindInvalid) {
		t.Fatal(rec.Code)
	}
	before := plano.Realizado(mustGet("2.3.2"), "2026-09")
	f.Set("conta", "2.3.2")
	f.Set("data", "2026-09-25")
	f.Set("valor", "1.000,50")
	f.Set("descricao", "Meetup")
	rec = c.do("POST", "/lancamentos", f)
	if rec.Code != 303 || rec.Header().Get("Location") != "/contas/2.1?mes=2026-09&ok=1" {
		t.Fatal(rec.Code, rec.Header().Get("Location"))
	}
	if plano.Realizado(mustGet("2.3.2"), "2026-09")-before != 100050 {
		t.Fatal("entry not added")
	}
	rec = c.do("GET", "/contas/2.3.2?mes=2026-09&ok=1", nil)
	if !strings.Contains(rec.Body.String(), "Meetup") || !strings.Contains(rec.Body.String(), `data-ui-fade="4000"`) {
		t.Fatal("toast/entry missing")
	}
	if rec := c.do("GET", "/lancamentos?conta=2.3.2", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `<option value="2.3.2" selected>`) {
		t.Fatal("standalone form")
	}
}

func TestCSV(t *testing.T) {
	c := newClient(t)
	rec := c.do("GET", "/api/relatorio.csv?mes=2026-07", nil)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Header().Get("Content-Disposition"), "orcamento-2026-07.csv") {
		t.Fatal(rec.Code, rec.Header())
	}
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if lines[0] != "codigo,conta,tipo,nivel,orcado,realizado,variacao_pct" || len(lines) != 1+18 || !strings.HasPrefix(lines[1], "1,Receitas,receita,1,") {
		t.Fatal(len(lines), lines[0], lines[1])
	}
	if rec := c.do("GET", "/api/relatorio.csv?mes=x", nil); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func mustGet(cod string) *plano.Conta {
	c, _ := plano.Get(cod)
	return c
}
