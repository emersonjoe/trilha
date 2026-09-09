package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

func registros() []trilha.AuditRecord {
	at := time.Now().Add(-2 * time.Hour)
	return []trilha.AuditRecord{
		{
			At: at, Action: "documento.excluiu", Target: "42", IP: "10.0.0.7",
			Route:  "/documentos/{id}",
			Actor:  trilha.Actor{Subject: "u_17", Name: "Ana", Via: "session"},
			Fields: trilha.Fields{"modulo": "docs", "de": "ver"},
		},
		{
			At: at, Action: "chave.usou", Target: "k_9",
			Actor: trilha.Actor{Subject: "key_1", Via: "api_key"},
		},
		{At: at, Action: "login.tentou", Actor: trilha.Actor{Via: "anonymous"}},
	}
}

func TestAuditTableMostraQuemFezOQue(t *testing.T) {
	got := render(t, AuditTable(nil, registros(), AuditOpts{Total: 3}))
	for _, want := range []string{
		"Ana", "documento.excluiu", "10.0.0.7", ">42<",
		"Who", "Action", "Target", "Detail",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}

// Ninguém reconhecido é registrado como tal: trilha que some com a ação anônima
// tem um buraco exatamente onde alguém vai procurar.
func TestAuditTableEscreveOAnonimo(t *testing.T) {
	got := render(t, AuditTable(nil, registros(), AuditOpts{Total: 3}))
	if !strings.Contains(got, "anonymous") {
		t.Fatalf("o anônimo sumiu:\n%s", got)
	}
	// E o que não é uma pessoa no teclado leva o selo do como.
	if !strings.Contains(got, "api_key") {
		t.Fatalf("faltou o selo do api_key:\n%s", got)
	}
}

// Fields é detalhe e não coluna: cada ação tem as suas chaves, e uma coluna por
// chave é uma tabela que ganha coluna toda vez que alguém audita algo novo.
func TestAuditTableAbreODetalhe(t *testing.T) {
	got := render(t, AuditTable(nil, registros(), AuditOpts{Total: 3}))
	if !strings.Contains(got, "<details") || !strings.Contains(got, "ui-audit-fields") {
		t.Fatalf("detalhe:\n%s", got)
	}
	// Em ordem, sempre a mesma: "de" antes de "modulo".
	if i, j := strings.Index(got, "de: "), strings.Index(got, "modulo: "); i < 0 || j < 0 || i > j {
		t.Fatalf("as chaves saíram fora de ordem:\n%s", got)
	}
	// O gabarito da rota entra no detalhe, não numa coluna sua.
	if !strings.Contains(got, "/documentos/{id}") {
		t.Fatalf("faltou a rota:\n%s", got)
	}
}

// A exportação leva o recorte que está na tela. Exportar ignorando o filtro na
// frente da pessoa é exportar a coisa errada, e ela só descobre na planilha.
func TestAuditTableExportaOMesmoRecorte(t *testing.T) {
	got := render(t, AuditTable(nil, registros(), AuditOpts{
		Total: 3, Export: "/auditoria.csv", Actions: []string{"documento.excluiu"}, Action: "documento.excluiu",
	}))
	if !strings.Contains(got, `href="/auditoria.csv?`) && !strings.Contains(got, `href="/auditoria.csv"`) {
		t.Fatalf("botão de exportar:\n%s", got)
	}
	if !strings.Contains(got, "Export CSV") {
		t.Fatalf("faltou o rótulo:\n%s", got)
	}
	// O select volta na opção que está filtrando.
	if !strings.Contains(got, `value="documento.excluiu" selected`) {
		t.Fatalf("o filtro não voltou marcado:\n%s", got)
	}
}

func TestAuditTableSemNadaExplica(t *testing.T) {
	got := render(t, AuditTable(nil, nil, AuditOpts{Total: 0}))
	if !strings.Contains(got, "Nothing recorded") {
		t.Fatalf("estado vazio:\n%s", got)
	}
}
