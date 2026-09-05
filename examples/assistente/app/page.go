package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
	"github.com/emersonjoe/trilha/h"
)

// Page renders the chat.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Assistente · Trilha")
	tools := h.Fragment()
	for _, t := range ferramentas.Tools {
		tools = h.Fragment(tools, h.Li(h.Code(h.Text(t.Name)), h.Text(" — "+t.Description)))
	}
	return h.Fragment(
		h.Section(h.Class("chat"),
			h.Div(h.ID("mensagens"), h.Aria("live", "polite"),
				h.Div(h.Class("msg assistente"), h.Text("Olá! Pergunte as horas, peça uma conta, guarde uma nota ou peça uma tradução.")),
			),
			h.Form(h.ID("form"), h.Attr("autocomplete", "off"),
				h.Input(h.ID("entrada"), h.Name("mensagem"), h.Placeholder("Escreva uma mensagem…"), h.Required(), h.Attr("maxlength", "2000")),
				h.Button(h.Type("submit"), h.Text("Enviar")),
			),
			h.P(h.ID("estado"), h.Class("estado")),
		),
		h.Aside(h.Class("info"),
			h.H2(h.Text("Ferramentas")),
			h.Ul(tools),
			h.H2(h.Text("Agentes")),
			h.P(h.Text("Assistente (ferramentas) → Tradutor (handoff quando você pede uma tradução).")),
			h.H2(h.Text("MCP")),
			h.P(h.Text("As mesmas ferramentas ficam em "), h.Code(h.Text("POST /mcp")), h.Text(" (Streamable HTTP) para hosts como Claude, Cursor ou VS Code.")),
			h.P(h.Text("Modelo: "), h.Code(h.Text(ferramentas.Client.Model)), h.Text(" em "), h.Code(h.Text(ferramentas.Client.BaseURL))),
		),
	), nil
}
