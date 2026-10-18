package jogo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// PropsDoJogo feeds every game island: one struct, chosen by the mode (#303).
type PropsDoJogo struct {
	Rodada   int      `json:"rodada"`
	Palavras []string `json:"palavras"`
}

// Contagem is what a helper builds.
type Contagem struct {
	N int `json:"n"`
}

func novaContagem(n int) Contagem { return Contagem{N: n} }

func Page(c *trilha.Ctx) (h.Node, error) {
	props := PropsDoJogo{Rodada: 1}
	var outra PropsDoJogo
	switch c.Param("modo") {
	case "pares":
		return c.Island("/jogo/pares.js", props, nil), nil
	case "lacuna":
		return c.Island("/jogo/lacuna.js", &props, nil), nil
	}
	return h.Div(
		c.Island("/jogo/blocos.js", outra, nil),
		c.Island("/jogo/contagem.js", novaContagem(3), nil),
		c.Island("/jogo/convertida.js", PropsDoJogo(props), nil),
		parametro(c, props),
	), nil
}

func parametro(c *trilha.Ctx, p PropsDoJogo) h.Node {
	return c.Island("/jogo/parametro.js", p, nil)
}
