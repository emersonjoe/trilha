// Package convites is the screen that invites somebody who does not have an
// account yet: it makes a one-use link with c.Link and sends it by e-mail.
//
// The two halves are the point. The link is a capability with a deadline —
// nothing is written to the users table until the person uses it — and the
// e-mail is what carries it, which is why this example has a mailer at all: a
// link nobody can deliver is a link somebody pastes into a chat, and that is
// where invitations leak.
package convites

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/correio"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page draws the form at GET /convites.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Convidar")
	return tela(c, "", ""), nil
}

// POST makes the link and sends it.
func POST(c *trilha.Ctx) error {
	email := strings.TrimSpace(c.Form("email"))
	nome := strings.TrimSpace(c.Form("nome"))
	if email == "" || !strings.Contains(email, "@") {
		return c.HTML(http.StatusUnprocessableEntity, tela(c, email, "Informe um e-mail válido."))
	}

	// 48 horas e um uso só: um convite é uma senha temporária. O que vai
	// dentro dele é lido por quem tiver o link, daí só o e-mail e o nome.
	caminho, err := c.Link("convite", trilha.LinkOpts{
		Data: map[string]string{"email": email, "nome": nome},
		TTL:  48 * time.Hour,
		Uses: 1,
		Path: "/convite",
	})
	if err != nil {
		return err
	}
	quem := "Alguém"
	if u := sessao.Flow.User(c); u != nil && u.Name != "" {
		quem = u.Name
	}
	if err := correio.Convite(c.Context(), email, nome, quem, endereco(c, caminho)); err != nil {
		// A falha do e-mail é do app, não de quem clicou — e a tela diz que
		// não saiu, em vez de dizer "convite enviado" para um envio que falhou.
		c.Log().Error("convite", "email", email, "err", err)
		return c.HTML(http.StatusBadGateway, tela(c, email,
			"O convite não pôde ser enviado. Confira a configuração de e-mail e tente de novo."))
	}
	c.Audit("convite.enviou", email, trilha.Fields{"nome": nome})
	c.Flash("success", "Convite enviado para "+email+".")
	return c.Redirect("/convites")
}

// endereco turns the path c.Link returned into the address that goes in the
// message: an e-mail has no current page to be relative to.
//
// TRILHA_BASE_URL comes first, because it is the only value that is right when
// the app sits behind a proxy. The fallback reads the Host header, which is
// whatever the client typed — safe here only because Trilha refuses a Host
// outside TRILHA_ALLOWED_HOSTS, and unsafe in any app that never set it.
func endereco(c *trilha.Ctx, caminho string) string {
	if base := strings.TrimSuffix(os.Getenv("TRILHA_BASE_URL"), "/"); base != "" {
		return base + caminho
	}
	esquema := "https"
	if c.Request().TLS == nil {
		esquema = "http"
	}
	return esquema + "://" + c.Request().Host + caminho
}

func tela(c *trilha.Ctx, email, erro string) h.Node {
	return ui.Card(
		ui.CardHeader(
			ui.CardTitle("Convidar alguém"),
			ui.CardDescription("Um link de uso único, válido por 48 horas, enviado por e-mail."),
		),
		ui.CardContent(
			h.Form(h.Method("post"), h.Action("/convites"),
				trilha.CSRFInput(c),
				ui.Field("nome", "Nome", ui.Input(h.ID("nome"), h.Name("nome"), h.Required())),
				ui.Field("email", "E-mail",
					ui.Input(h.ID("email"), h.Type("email"), h.Name("email"), h.Value(email), h.Required()),
					ui.Error(erro)),
				ui.Button(h.Type("submit"), h.Text("Enviar convite")),
			),
		),
	)
}
