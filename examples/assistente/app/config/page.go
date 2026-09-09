// Package configpage is the whole administration screen of a settings section:
// a page that draws the form from the struct, and a POST that is one line.
package configpage

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/assistente/internal/config"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page draws the form from the struct: um campo por campo, do tipo que as tags
// pediram, com o valor atual.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Configuração")
	return tela(c, nil), nil
}

// POST is the line the issue promised: bind, validate, save, auditar e voltar.
// O 422 é o único caso que esta função ainda escreve, porque a mensagem tem de
// voltar para dentro desta tela.
func POST(c *trilha.Ctx) error {
	err := config.Cfg.Update(c)
	errs, ok := err.(trilha.FieldErrors)
	if !ok {
		return err
	}
	return c.Render(422, tela(c, errs))
}

func tela(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	return h.Div(
		ui.H1(h.Text("Configuração do assistente")),
		ui.Muted(h.Text("Muda sem redeploy; vale para a próxima mensagem.")),
		ui.Card(ui.CardContent(ui.SettingsForm(c, config.Cfg, errs))),
	)
}
