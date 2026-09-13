// Package midiapagina é a tela que junta os dois binários que não são do app:
// o episódio, que vem de outro serviço e precisa responder Range para o
// <audio> do iPhone posicionar, e a marca em SVG, que o navegador trata como
// documento com script. As duas rotas ao lado servem os dois de dentro do
// envelope do kit.
package midiapagina

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Mídia")
	return h.Div(
		ui.H1(h.Text("Mídia")),
		h.P(h.Img(h.Src("/midia/marca"), h.Alt("Marca da organização"), h.Width("64"), h.Height("64"))),
		ui.Muted(h.Text("O logotipo é um SVG: sai inline, com a política que desliga o script.")),
		h.Audio(h.Controls(), h.Src("/midia/audio"), h.Attr("preload", "metadata")),
		ui.Muted(h.Text("O episódio chega de outro serviço, sem buscar posição: o Range é cortado do stream.")),
		h.P(h.A(h.Href("/midia/audio"), h.Text("Baixar o episódio"))),
	), nil
}
