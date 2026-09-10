// Package painel is a route group for the app area (/painel, /relatorio).
package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// dica é a linha sob o título do assistente: o que ele sabe agora. Vem da rota
// porque é a rota que muda — o mesmo painel numa tela e noutra não é a mesma
// pergunta.
func dica(rota string) string {
	switch rota {
	case "/relatorio":
		return "Perguntando sobre o relatório"
	case "/assistente":
		return "Perguntando sobre a área do app"
	default:
		return "Perguntando sobre o painel"
	}
}

// Layout wraps the app area with a sidebar.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	area, _ := c.Get("area").(string)
	cur := c.Request().URL.Path
	// Navegação no cliente na área do app: um clique na barra lateral busca a
	// próxima página e troca só o #conteudo, mantendo cabeçalho e rolagem. O
	// endereço na barra é o mesmo de sempre; sem JavaScript, o link recarrega.
	return h.Section(h.Class("app"), h.Data("area", area), ui.Navigate("conteudo"),
		ui.NavigateScript(c),
		ui.Sidebar(ui.Nav(
			ui.NavLink("/painel", "Painel", cur == "/painel"),
			ui.NavLink("/relatorio", "Relatório", cur == "/relatorio"),
		)),
		h.Div(h.Class("app-content"), children),
		// O assistente é montado uma vez, aqui: quem entra na área do app tem o
		// botão no canto em todas as telas dela. A dica e o contexto saem da
		// rota — é o que a página sabe e o modelo não — e o launcher é um link
		// para /assistente, que é a mesma conversa como página.
		ui.Assistant(c, ui.AssistantOpts{
			Action: "/assistente",
			Page:   "/assistente",
			Label:  "Assistente",
			Hint:   dica(cur),
			Chat: ui.ChatOpts{
				Greeting:    "Pergunte alguma coisa sobre esta tela.",
				Placeholder: "Escreva uma mensagem…",
				Submit:      "Enviar",
				Context:     map[string]string{"rota": cur},
			},
		}),
		ui.ChatScript(c),
	), nil
}
