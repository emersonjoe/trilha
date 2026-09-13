// Package midiamarca serve o logotipo que uma organização subiu. Ele costuma
// ser um SVG, e um SVG é desenho para o olho e documento com script para o
// navegador: o c.Inline recusa, e a recusa está certa.
package midiamarca

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/midia"
)

// GET /midia/marca?org= devolve o logotipo inline, com o script desligado.
//
// O NeutralizeScript é a frase "eu sei o que é um SVG": a resposta sai do
// mesmo envelope de sempre — nome saneado, nosniff, enquadramento relaxado só
// nesta resposta — e com a política que o kit impõe, que não deixa nada rodar
// nem buscar recurso nenhum.
//
// E quando o que a organização subiu não é desenho nenhum, a recusa é erro de
// conteúdo e não de programação: a tela de entrada fica com a marca padrão em
// vez de estourar uma página de erro.
//
// openapi:query org string  a organização; vazio é a do próprio blog
// openapi:tag midia
func GET(c *trilha.Ctx) error {
	l, ok := midia.LogoDe(c.Query("org"))
	if !ok {
		return trilha.Errorf(http.StatusNotFound, "organização sem marca")
	}
	err := c.Send(l.Nome, bytes.NewReader(l.Bytes), l.Tipo, trilha.SendOpts{
		Inline:           true,
		NeutralizeScript: true,
	})
	if errors.Is(err, trilha.ErrCannotInline) {
		p := midia.LogoPadrao()
		return c.Send(p.Nome, bytes.NewReader(p.Bytes), p.Tipo, trilha.SendOpts{
			Inline:           true,
			NeutralizeScript: true,
		})
	}
	return err
}
