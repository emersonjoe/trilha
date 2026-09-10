package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Spec 093 (#142): the assistant in the corner is the other shape of a chat —
// a button that opens a panel over the screen, mounted once in the layout of an
// authenticated area. What it must not be is a screen that stops working when
// the script does not load.
func TestAssistantAbreSemVirarSPA(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Assistant(c, AssistantOpts{
			Action: "/api/assistente",
			Page:   "/painel/assistente",
			Label:  "Assistente",
			Hint:   "Perguntando sobre a folha 12",
			Chat:   ChatOpts{Context: map[string]string{"folha": "12", "rota": "/painel/folhas/12"}},
		})
	})
	for _, want := range []string{
		// The launcher is a link first: without script it goes to the page.
		`href="/painel/assistente"`,
		`aria-expanded="false"`,
		`aria-controls="assistant-dialog"`,
		`aria-haspopup="dialog"`,
		`data-ui-dialog-open="assistant-dialog"`,
		// And the panel is a native dialog, which is focus and Escape from the
		// browser instead of two hundred lines of ours.
		`<dialog class="ui-dialog ui-assistant-dialog" id="assistant-dialog"`,
		`Perguntando sobre a folha 12`,
		// The chat inside is the chat that already exists.
		`data-trilha-chat="/api/assistente"`,
		// The context travels as fields of the form, in name order, escaped.
		`<input type="hidden" name="ctx.folha" value="12">`,
		`<input type="hidden" name="ctx.rota" value="/painel/folhas/12">`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("falta %q em:\n%s", want, got)
		}
	}
	if i, j := strings.Index(got, "ctx.folha"), strings.Index(got, "ctx.rota"); i > j {
		t.Error("os campos de contexto saíram fora de ordem: o mesmo app tem de render o mesmo HTML")
	}
}

// Two assistants on one page share nothing: every id is derived from the ID,
// and a second one that reuses the first's ids is two panels answering one
// button.
func TestDoisAssistentesNaoCompartilhamID(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return h.Div(
			Assistant(c, AssistantOpts{ID: "ajuda", Action: "/api/ajuda", Page: "/ajuda"}),
			Assistant(c, AssistantOpts{ID: "suporte", Action: "/api/suporte", Page: "/suporte"}),
		)
	})
	for _, want := range []string{
		`id="ajuda-dialog"`, `id="ajuda-chat"`, `id="ajuda-chat-form"`,
		`id="suporte-dialog"`, `id="suporte-chat"`, `id="suporte-chat-form"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("falta %q em:\n%s", want, got)
		}
	}
}

// The context is a value, never markup: a page that puts a document title in it
// must not be able to close the attribute.
func TestContextoNaoEHTML(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Chat(c, ChatOpts{
			Action:  "/api/chat",
			Context: map[string]string{"titulo": `" onload="alert(1)`},
		})
	})
	if strings.Contains(got, `onload="alert(1)"`) {
		t.Fatalf("o contexto virou atributo:\n%s", got)
	}
	if !strings.Contains(got, `name="ctx.titulo"`) {
		t.Fatalf("o campo de contexto não foi escrito:\n%s", got)
	}
}
