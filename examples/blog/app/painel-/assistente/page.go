// Package assistente é o assistente da área do app: a mesma rota responde a
// conversa como página, para quem está sem JavaScript, e como stream, para o
// ui.chat.js.
//
// O modelo aqui é uma linha de código e não uma chamada de rede: o exemplo
// existe para mostrar o contrato — o contexto que a página mandou chega ao
// servidor — e não para gastar uma chave de API em cada `go test`. Num app de
// verdade estas quinze linhas são um `ai.Serve`.
package assistente

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page responde GET /assistente: a conversa como página inteira. É para onde o
// launcher do ui.Assistant aponta, e é o que abre quando o script não está lá.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Assistente")
	return tela(c, nil), nil
}

// POST responde a mensagem. Com Accept: text/event-stream vai em eventos, que é
// o que o ui.chat.js lê; sem ele, a página volta com a conversa dentro — a
// mesma rota, os dois caminhos.
func POST(c *trilha.Ctx) error {
	pergunta := strings.TrimSpace(c.Form("message"))
	if pergunta == "" {
		return trilha.Errorf(422, "diga alguma coisa")
	}
	resposta := responder(c, pergunta)
	if !strings.Contains(c.Request().Header.Get("Accept"), "text/event-stream") {
		return c.Render(200, tela(c, []ui.ChatMessage{
			{Role: "user", Text: pergunta},
			{Role: "assistant", Text: resposta},
		}))
	}
	s := c.Stream()
	if err := s.Send("text", resposta); err != nil {
		return err
	}
	return s.JSON("done", map[string]any{"output": resposta, "html": ui.ChatHTML(resposta)})
}

// responder é o "modelo": ele repete de onde a pergunta veio, que é o único
// jeito de um teste provar que o contexto da página atravessou.
func responder(c *trilha.Ctx, pergunta string) string {
	rota := c.Form("ctx.rota")
	if rota == "" {
		rota = "lugar nenhum"
	}
	return "Você perguntou **" + pergunta + "** a partir de `" + rota + "`."
}

func tela(c *trilha.Ctx, historia []ui.ChatMessage) h.Node {
	return ui.Stack(
		h.H1(h.Class("ui-h1"), h.Text("Assistente")),
		ui.Muted(h.Text("Esta é a conversa como página: é o que o botão do canto abre quando não há JavaScript.")),
		ui.Card(ui.CardContent(ui.Chat(c, ui.ChatOpts{
			Action:      "/assistente",
			History:     historia,
			Greeting:    "Pergunte alguma coisa sobre o painel.",
			Placeholder: "Escreva uma mensagem…",
			Submit:      "Enviar",
			Context:     map[string]string{"rota": c.Request().URL.Path},
		}))),
	)
}
