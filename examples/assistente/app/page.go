package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the chat.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Assistente · Trilha")
	tools := h.Fragment()
	for _, t := range ferramentas.Tools {
		tools = h.Fragment(tools, h.Li(ui.Code(t.Name), h.Text(" — "+t.Description)))
	}
	return h.Div(h.Class("layout"),
		ui.Card(h.Class("chat"),
			h.Div(h.ID("mensagens"), h.Aria("live", "polite"),
				h.Div(h.Class("msg assistente"), h.Text("Olá! Pergunte as horas, peça uma conta, guarde uma nota ou peça uma tradução.")),
			),
			h.Form(h.ID("form"), h.Attr("autocomplete", "off"),
				ui.Input(h.ID("entrada"), h.Name("mensagem"), h.Placeholder("Escreva uma mensagem…"), h.Required(), h.Attr("maxlength", "2000")),
				ui.Submit(ui.Icon("arrow-right"), h.Text("Enviar")),
			),
			ui.Muted(h.ID("estado"), h.Class("estado")),
		),
		ui.Card(h.Class("info"),
			ui.CardHeader(ui.CardTitle("Ferramentas")),
			ui.CardContent(h.Ul(tools)),
			ui.CardHeader(ui.CardTitle("Agentes")),
			ui.CardContent(h.P(h.Text("Assistente (ferramentas) → Tradutor (handoff quando você pede uma tradução)."))),
			ui.CardHeader(ui.CardTitle("MCP")),
			ui.CardContent(
				h.P(h.Text("As mesmas ferramentas ficam em "), ui.Code("POST /mcp"), h.Text(" (Streamable HTTP) para hosts como Claude, Cursor ou VS Code.")),
				h.P(ui.Badge(ui.Secondary(), h.Text(ferramentas.Client.Model)), h.Text(" em "), ui.Code(ferramentas.Client.BaseURL)),
			),
		),
	), nil
}
