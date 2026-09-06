package app

import (
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Error renders every error status but 404, with the app's own layout. The
// status comes from the error, so a 403 reads like the app and not like the
// framework.
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	if trilha.StatusOf(err) == http.StatusForbidden {
		c.SetTitle("Sem acesso")
		// A mensagem do HTTPError foi escrita para o cliente ver; qualquer
		// outro erro de 403 vira a frase genérica.
		motivo := "Você não tem acesso a esta página."
		var he *trilha.HTTPError
		if errors.As(err, &he) && he.Message != "" {
			motivo = he.Message
		}
		return ui.Stack(
			h.H1(h.Text("Sem acesso")),
			ui.Muted(h.Text(motivo)),
			h.P(h.A(h.Href("/login?next="+c.Request().URL.Path), h.Text("Entrar"))),
		), nil
	}
	c.SetTitle("Erro")
	return ui.Stack(
		h.H1(h.Text("Algo deu errado")),
		h.If(c.Env() == trilha.Dev, ui.Alert("Detalhe (só em dev)", ui.Destructive(), ui.Icon("triangle-alert"), ui.AlertDescription(h.Pre(h.Text(err.Error()))))),
		ui.Muted(h.Textf("Cite o id %s ao reportar.", c.RequestID())),
	), nil
}
