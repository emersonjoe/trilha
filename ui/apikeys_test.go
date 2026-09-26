package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// O cartão do segredo tem de dizer a coisa que ninguém quer ler depois: é a
// única vez. Sem isso, a pessoa fecha a aba e a chave se perde.
func TestSecretOnceAvisaQueEhAUnicaVez(t *testing.T) {
	got := render(t, SecretOnce(nil, "ak_abc_segredo"))
	for _, want := range []string{"ak_abc_segredo", "only time", "readonly", `data-ui-copy="ak_abc_segredo"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}

// Atributo repetido não é cosmético: class="ui-input ui-input" e dois type no
// mesmo botão foi o que a tela do exemplo mostrou.
func TestSecretOnceNaoRepeteAtributo(t *testing.T) {
	got := render(t, SecretOnce(nil, "ak_abc_segredo"))
	if strings.Contains(got, "ui-input ui-input") {
		t.Fatalf("classe repetida: %s", got)
	}
	if strings.Count(got, `type="button"`) != 1 {
		t.Fatalf("type repetido: %s", got)
	}
}

func TestAPIKeysTableMostraOHandleENuncaAChave(t *testing.T) {
	rows := []APIKeyRow{
		{ID: "k1", Handle: "abc123", Name: "Integração", Scopes: []string{"docs:read"},
			Created: time.Now().Add(-48 * time.Hour), LastUsed: time.Now().Add(-time.Hour)},
		{ID: "k2", Handle: "def456", Name: "Antiga", Scopes: []string{"docs:read"},
			Created: time.Now().Add(-72 * time.Hour), Revoked: true},
	}
	got := render(t, APIKeysTable(nil, rows, APIKeysOpts{Revoke: "/chaves/revogar"}))
	for _, want := range []string{"Integração", "abc123", "docs:read", "never", "revoked", `action="/chaves/revogar"`, "data-ui-confirm"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
	// A chave revogada não oferece revogar de novo.
	if strings.Count(got, `value="k2"`) != 0 {
		t.Fatalf("a revogada ainda oferece o botão:\n%s", got)
	}
}

// #216: a receita despacha o único POST da tela pelo campo action, e sem
// action=revoke o clique de verdade cai no ramo que cria uma chave sem nome.
func TestAPIKeysTableRevogarLevaAAction(t *testing.T) {
	rows := []APIKeyRow{{ID: "k1", Handle: "abc123", Name: "Integração"}}
	got := render(t, APIKeysTable(nil, rows, APIKeysOpts{Revoke: "/chaves"}))
	if !strings.Contains(got, `<input type="hidden" name="action" value="revoke">`) {
		t.Fatalf("o formulário de revogar não manda action=revoke:\n%s", got)
	}
}

func TestAPIKeysTableSemChavesExplica(t *testing.T) {
	if got := render(t, APIKeysTable(nil, nil)); !strings.Contains(got, "No keys yet") {
		t.Fatalf("vazio:\n%s", got)
	}
}

// #288: a tela que escreve data no formato do produto passa o formatador uma
// vez e as duas colunas de data seguem; "nunca" continua sendo "nunca".
func TestAPIKeysTableDateFormatSobrescreveAsDatas(t *testing.T) {
	dia := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	rows := []APIKeyRow{
		{ID: "k1", Handle: "abc", Name: "Usada", Created: dia, LastUsed: dia},
		{ID: "k2", Handle: "def", Name: "Nova", Created: dia},
	}
	got := render(t, APIKeysTable(nil, rows, APIKeysOpts{
		DateFormat: func(_ *trilha.Ctx, t time.Time) h.Node { return h.Text("dia " + t.Format("2")) },
	}))
	if n := strings.Count(got, "dia 11"); n != 3 {
		t.Fatalf("o formatador desenhou %d datas, quero 3:\n%s", n, got)
	}
	if strings.Contains(got, "<time") || !strings.Contains(got, "never") {
		t.Fatalf("sobrou a data do kit ou sumiu o nunca:\n%s", got)
	}
}

// #289: criação e último uso respondem perguntas diferentes — o dia basta para
// uma, a outra pede a hora. LogFormat desenha a coluna de evento.
func TestAPIKeysTableLogFormatDesenhaOUltimoUso(t *testing.T) {
	dia := time.Date(2026, 9, 11, 14, 32, 0, 0, time.UTC)
	rows := []APIKeyRow{{ID: "k1", Handle: "abc", Name: "Usada", Created: dia, LastUsed: dia}}
	got := render(t, APIKeysTable(nil, rows, APIKeysOpts{
		DateFormat: func(_ *trilha.Ctx, t time.Time) h.Node { return h.Text("dia " + t.Format("2")) },
		LogFormat:  func(_ *trilha.Ctx, t time.Time) h.Node { return h.Text("às " + t.Format("15:04")) },
	}))
	if strings.Count(got, "dia 11") != 1 || strings.Count(got, "às 14:32") != 1 {
		t.Fatalf("criação com o dia e uso com a hora, uma vez cada:\n%s", got)
	}
}
