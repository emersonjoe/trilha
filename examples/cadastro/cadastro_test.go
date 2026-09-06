package main

import (
	"io"
	"log/slog"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
)

func newClient(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
}

// tela pede só o fragmento da tela, sem documento nem layout (spec 018).
var tela = trilha.WithHeader("Trilha-Fragment", "tela")

func TestFormRendersConditionalGroups(t *testing.T) {
	c := newClient(t)
	c.Get("/").WantStatus(200).WantContains(
		`data-ui-show-when="tipo=pf"`, `data-ui-show-when="tipo=pj"`,
		`data-ui-show-when="cobranca_diferente"`, `data-ui-show-when="novidades"`,
		"Ada Lovelace", `<select class="ui-select" id="cidade" name="cidade" disabled`)
}

func TestValidationRoundTrip(t *testing.T) {
	c := newClient(t)
	f := url.Values{}
	f.Set("tipo", "pj")
	f.Set("nome", "Empresa X")
	f.Set("email", "nao-e-email")
	f.Set("cnpj", "11111111111111")
	f.Set("cep", "123")
	f.Set("uf", "SP")
	f.Set("cidade", "Campinas")
	f.Set("novidades", "on")
	res := c.PostForm("/", f).WantStatus(422).WantContains(
		"CNPJ inválido", "E-mail inválido", "Informe a razão social", "CEP com 8 dígitos",
		"Informe a rua", "Escolha a frequência", `value="Empresa X"`, `value="nao-e-email"`,
		`value="pj" checked`, `<option value="SP" selected>`, `<option value="Campinas" selected>`,
		`name="novidades" checked`, `<title>Cadastro de cliente</title>`)
	if strings.Contains(res.Body.String(), "CPF inválido") {
		t.Fatal("PF rules must not apply to PJ")
	}
	if len(clientes.Todos()) != 1 {
		t.Fatal("must not save")
	}
}

func TestValidSubmissionAndHiddenFieldsIgnored(t *testing.T) {
	c := newClient(t)
	f := url.Values{}
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
	c.PostForm("/", f).WantStatus(303).WantHeader("Location", "/?ok=1")
	todos := clientes.Todos()
	if len(todos) != 2 || todos[0].Nome != "Grace Hopper" || todos[0].Email != "grace@example.com" || todos[0].CPF != "52998224725" || todos[0].CNPJ != "" || todos[0].Frequencia != "mensal" {
		t.Fatalf("%+v", todos[0])
	}
	c.Get("/?ok=1").WantContains(`class="ui-toast ui-toast-success"`, `data-ui-fade="4000"`, "Grace Hopper")
	// Invalid CPF and under-age.
	f.Set("cpf", "12345678900")
	f.Set("nascimento", "2020-01-01")
	c.PostForm("/", f).WantStatus(422).WantContains("CPF inválido", "18 anos")
}

func TestCidadesAPI(t *testing.T) {
	c := newClient(t)
	res := c.Get("/api/cidades?uf=SP").WantStatus(200).WantContains(`"Campinas"`)
	if res.Header().Get("Cache-Control") == "" {
		t.Fatal("a lista é estável: ela merece cache")
	}
	c.Get("/api/cidades?uf=XX").WantStatus(404)
}

func TestDocumentos(t *testing.T) {
	if !clientes.CPFValido("529.982.247-25") || clientes.CPFValido("111.111.111-11") || clientes.CPFValido("52998224726") {
		t.Fatal("cpf")
	}
	if !clientes.CNPJValido("04.252.011/0001-10") || clientes.CNPJValido("04252011000111") || clientes.CNPJValido("00000000000000") {
		t.Fatal("cnpj")
	}
}

// Spec 018: a busca funciona nos dois caminhos, pelo mesmo endereço.
func TestBuscaComESemFragmento(t *testing.T) {
	c := newClient(t)
	// Sem o cabeçalho: página inteira, filtrada.
	c.Get("/?q=ada").WantContains("Ada Lovelace", "<!doctype")
	c.Get("/?q=zzz").WantContains("Nada encontrado para zzz")
	// Com o cabeçalho: só a tela, sem documento nem layout.
	rec := c.Get("/?q=ada", tela)
	body := rec.Body.String()
	if strings.Contains(body, "<!doctype") || strings.Contains(body, "<html") {
		t.Fatalf("fragmento com envelope: %s", body[:200])
	}
	if !strings.HasPrefix(body, `<div id="tela"`) || !strings.Contains(body, "Ada Lovelace") {
		t.Fatalf("fragmento inesperado: %s", body[:200])
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Trilha-Fragment") {
		t.Fatal("sem Vary, um cache serviria o pedaço no lugar da página")
	}
}

// Spec 018: o envio sem recarga devolve a tela nova; sem o cabeçalho, PRG.
func TestEnvioSemRecarga(t *testing.T) {
	c := newClient(t)
	f := url.Values{}
	f.Set("tipo", "pf")
	f.Set("nome", "Alan Turing")
	f.Set("email", "alan@example.com")
	f.Set("cpf", "529.982.247-25")
	f.Set("nascimento", "1912-06-23")
	f.Set("cep", "13010-000")
	f.Set("rua", "Rua Treze de Maio")
	f.Set("numero", "2")
	f.Set("uf", "SP")
	f.Set("cidade", "Campinas")
	rec := c.PostForm("/", f, tela).WantStatus(200)
	body := rec.Body.String()
	if strings.Contains(body, "<!doctype") || !strings.HasPrefix(body, `<div id="tela"`) {
		t.Fatalf("resposta não é um fragmento: %s", body[:200])
	}
	if !strings.Contains(body, "Cadastro salvo!") || !strings.Contains(body, "Alan Turing") {
		t.Fatal("a tela devolvida precisa trazer o aviso e a lista atualizada")
	}
	if rec.Header().Get("Location") != "" || rec.Header().Get("Trilha-Location") != "" {
		t.Fatal("o caminho sem recarga não redireciona")
	}
	// O erro de validação volta como fragmento, com 422 e o campo marcado.
	f.Set("email", "nao-e-email")
	// O cliente foca o primeiro aria-invalid: ele precisa estar lá.
	c.PostForm("/", f, tela).WantStatus(422).
		WantContains("E-mail inválido", `aria-invalid="true"`)
}
