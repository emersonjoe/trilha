package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// #147 — a caixa é uma tabela e dois formulários: sem JavaScript, com o prazo
// escrito no idioma de quem lê e o atraso destacado.
func TestInbox(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Inbox(c, []InboxRow{
			{ID: "apr_1", Kind: "eliminacao", Subject: "Listagem 2024/07",
				Target: "/admin/retencao/123", State: "pending",
				Due: time.Now().Add(-2 * time.Hour), Late: true},
			{ID: "apr_2", Kind: "reembolso", Subject: "Nota 88", State: "approved",
				By: "ana", Reason: "dentro da política"},
		}, InboxOpts{Decide: "/admin/tarefas", CSRF: trilha.CSRFInput(c)})
	})
	for _, quero := range []string{
		`href="/admin/retencao/123"`,
		`class="ui-late"`,
		`name="id" value="apr_1"`,
		`name="decision" value="approved"`,
		`name="decision" value="rejected"`,
		`name="reason"`,
		"ui-badge",
		"dentro da política",
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	// A linha já decidida não traz os botões: decidir de novo o que já foi
	// decidido é um clique que não faz nada e uma pergunta que fica.
	if strings.Count(got, `name="decision"`) != 2 {
		t.Fatalf("a linha decidida tem botões:\n%s", got)
	}
}

// Uma caixa vazia diz que está vazia, e não desenha uma tabela de nada.
func TestInboxVazia(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Inbox(c, nil, InboxOpts{Decide: "/x"})
	})
	if !strings.Contains(got, "Nothing waiting") {
		t.Fatalf("caixa vazia = %s", got)
	}
}

// O contador de menu some no zero: um badge que mostra zero ensina a ignorar
// badges.
func TestInboxBadge(t *testing.T) {
	if got := render(t, InboxBadge(0)); got != "" {
		t.Fatalf("badge de zero = %q", got)
	}
	if got := render(t, InboxBadge(3)); !strings.Contains(got, ">3<") {
		t.Fatalf("badge = %q", got)
	}
}
