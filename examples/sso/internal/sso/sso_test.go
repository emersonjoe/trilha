package sso

import (
	"testing"

	"github.com/emersonjoe/trilha/auth"
)

// O OnLoginToken também responde "preciso de uma claim que o kit não mapeia":
// o Claims.All é o payload inteiro do ID token, e é de lá que estas saem.
func TestExtrasDasClaims(t *testing.T) {
	tok := &auth.IDToken{Raw: "ey.ey.ey", Claims: &auth.Claims{All: map[string]any{
		"hd":      "exemplo.com",
		"nivel":   float64(3),
		"interno": true,
		"groups":  []any{"a", "b"}, // objeto e lista ficam de fora
	}}}

	got := extrasDasClaims(tok, []string{"hd", "nivel", "interno", "groups", "inexistente"})
	want := map[string]string{"hd": "exemplo.com", "nivel": "3", "interno": "true"}
	if len(got) != len(want) {
		t.Fatalf("extras = %v, queria %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("extras[%q] = %q, queria %q", k, got[k], v)
		}
	}
}

// Sem nada configurado não há o que copiar, e o callback não precisa se
// defender de um mapa nil.
func TestExtrasDasClaimsSemNomes(t *testing.T) {
	if got := extrasDasClaims(&auth.IDToken{Claims: &auth.Claims{}}, nil); got != nil {
		t.Fatalf("extras = %v", got)
	}
	if got := extrasDasClaims(nil, []string{"hd"}); got != nil {
		t.Fatalf("extras de um token nil = %v", got)
	}
}
