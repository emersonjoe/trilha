package painel

import (
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page shows the session and what it carries for the API. The clock is a live
// fragment: the page holds one stream (ui.Live) and this piece asks for itself
// again whenever the stream says the name it is waiting for (ui.On) — the
// event carries the name, never the HTML, so the authorization is the one this
// route already has.
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "agora" {
		return agora(), nil
	}
	c.SetTitle("Painel")
	u := sessao.Flow.User(c)
	return h.Div(h.Class("cartao"), ui.Live("/painel/eventos"),
		h.H1(h.Text("Olá, "+u.Name)),
		h.P(h.Text("Papéis: "+strings.Join(u.Roles, ", "))),
		// The token itself never reaches the page: what the browser needs to
		// know is that the call goes out with it.
		h.P(h.Text("As chamadas a /api/ saem com o token desta sessão.")),
		agora(),
		ui.LiveScript(c),
	), nil
}

// agora é o fragmento vivo. Sem JavaScript ele é o relógio de quando a página
// carregou: velho, nunca quebrado.
func agora() h.Node {
	return h.P(h.ID("agora"), ui.On("painel:agora", ""),
		h.Text("Servidor: "+time.Now().Format("15:04:05")))
}
