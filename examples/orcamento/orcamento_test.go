package main

import (
	"io"
	"log/slog"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
)

func newClient(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
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
	res := c.Get("/?mes=2026-08").WantStatus(200).WantContains(
		"<title>Orçamento ago/2026</title>", `href="/contas/2?mes=2026-08"`, `data-depth="1"`,
		`href="/contas/2.3?mes=2026-08"`, "Receita realizada", `<option value="2026-08" selected>`,
		`<dialog class="ui-dialog" id="novo"`)
	if strings.Contains(res.Body.String(), `href="/contas/2.3.1?`) {
		t.Fatal("overview shows only two levels")
	}
	c.Get("/contas/2.3?mes=2026-08").WantStatus(200).WantContains(
		"<title>2.3 Marketing</title>", `aria-label="breadcrumb"`,
		`href="/contas/2?mes=2026-08">2 Despesas</a>`, `href="/contas/2.3.2?mes=2026-08"`,
		"Contas filhas", "ui-badge ui-badge-destructive")
	c.Get("/contas/2.3.2?mes=2026-08").WantStatus(200).
		WantContains("Lançamentos", "Eventos (1ª quinzena)", "<tfoot>")
	c.Get("/contas/9.9").WantStatus(404)
	c.Get("/?mes=lixo").WantStatus(200).WantContains("set/2026")
}

func TestEntryValidationAndPost(t *testing.T) {
	c := newClient(t)
	f := url.Values{}
	f.Set("conta", "2.1") // synthetic
	f.Set("valor", "-5")
	f.Set("data", "")
	f.Set("voltar", "/contas/2.1?mes=2026-09")
	res := c.PostForm("/lancamentos", f).WantStatus(422).
		WantContains("conta analítica", "maior que zero", "obrigatório", `value="-5"`)
	// Issue #27: as regras de forma vêm da tag, em português, e a regra de
	// negócio (conta analítica) continua em Go — as duas na mesma resposta.
	if body := res.Body.String(); strings.Contains(body, "required") || strings.Contains(body, "must ") {
		t.Fatal("mensagem em inglês num app em português")
	}
	// Bind conversion error (bad date) surfaces as a field error too.
	f.Set("data", "ontem")
	c.PostForm("/lancamentos", f).WantStatus(422).WantContains(trilha.BindInvalid)

	before := plano.Realizado(mustGet("2.3.2"), "2026-09")
	f.Set("conta", "2.3.2")
	f.Set("data", "2026-09-25")
	f.Set("valor", "1.000,50")
	f.Set("descricao", "Meetup")
	c.PostForm("/lancamentos", f).WantStatus(303).
		WantHeader("Location", "/contas/2.1?mes=2026-09&ok=1")
	if plano.Realizado(mustGet("2.3.2"), "2026-09")-before != 100050 {
		t.Fatal("entry not added")
	}
	c.Get("/contas/2.3.2?mes=2026-09&ok=1").WantContains("Meetup", `data-ui-fade="4000"`)
	c.Get("/lancamentos?conta=2.3.2").WantStatus(200).
		WantContains(`<option value="2.3.2" selected>`)
}

func TestCSV(t *testing.T) {
	c := newClient(t)
	res := c.Get("/api/relatorio.csv?mes=2026-07").WantStatus(200)
	if !strings.HasPrefix(res.Header().Get("Content-Type"), "text/csv") || !strings.Contains(res.Header().Get("Content-Disposition"), "orcamento-2026-07.csv") {
		t.Fatal(res.Header())
	}
	lines := strings.Split(strings.TrimSpace(res.Body.String()), "\n")
	if lines[0] != "codigo,conta,tipo,nivel,orcado,realizado,variacao_pct" || len(lines) != 1+18 || !strings.HasPrefix(lines[1], "1,Receitas,receita,1,") {
		t.Fatal(len(lines), lines[0], lines[1])
	}
	c.Get("/api/relatorio.csv?mes=x").WantStatus(400)
}

func mustGet(cod string) *plano.Conta {
	c, _ := plano.Get(cod)
	return c
}
