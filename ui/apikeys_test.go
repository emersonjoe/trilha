package ui

import (
	"strings"
	"testing"
	"time"
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

func TestAPIKeysTableSemChavesExplica(t *testing.T) {
	if got := render(t, APIKeysTable(nil, nil)); !strings.Contains(got, "No keys yet") {
		t.Fatalf("vazio:\n%s", got)
	}
}
