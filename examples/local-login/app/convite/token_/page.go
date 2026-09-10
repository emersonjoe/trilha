// Package token_ is the other side of the invitation: the page somebody opens
// from the e-mail, with no session, to choose a password.
//
// It is a folder of its own, outside /convites, and that is not tidiness: a
// middleware guards its folder and everything under it, so an accept page
// under the screen that invites would demand the session the invited person
// does not have yet. The link is the authorization here. c.Claim checks the signature, the purpose and the deadline,
// and answers the same thing for all the ways it can fail — telling a stranger
// whether a token expired or never existed tells them how close they are.
package token_

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/correio"
	"github.com/emersonjoe/trilha/examples/local-login/internal/usuarios"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page draws the password form at GET /convite/{token}.
func Page(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim("convite")
	if err != nil {
		return nil, err // 404: inválido, vencido ou já usado — nunca qual
	}
	c.SetTitle("Criar minha senha")
	return tela(c, link.Data["nome"], link.Data["email"], ""), nil
}

// POST creates the account and spends the link.
func POST(c *trilha.Ctx) error {
	link, err := c.Claim("convite")
	if err != nil {
		return err
	}
	senha := c.Form("senha")
	if len(senha) < 10 {
		return c.HTML(http.StatusUnprocessableEntity,
			tela(c, link.Data["nome"], link.Data["email"], "A senha precisa de pelo menos 10 caracteres."))
	}
	// Gastar antes de criar. Se a criação falhar, o convite foi embora e a
	// pessoa pede outro; se fosse ao contrário, dois envios simultâneos
	// criariam duas contas.
	if err := link.Consume(); err != nil {
		return err
	}
	tabela := trilha.Use[*usuarios.Store](c)
	email := link.Data["email"]
	nome := link.Data["nome"]
	if nome == "" {
		nome = strings.SplitN(email, "@", 2)[0]
	}
	tabela.Add("u-"+link.ID, email, nome, "leitor", senha, "jwt-"+link.ID)
	// Quem age aqui não tem sessão — é justamente quem ainda não tem conta —
	// mas tem nome: o convite diz de quem ele é. Sem esta linha a trilha
	// registraria "anônimo" no único evento que precisa dizer quem aceitou, e
	// é isso que o `trilha audit` aponta numa rota aberta que audita.
	c.SetActor(trilha.Actor{Subject: "u-" + link.ID, Email: email, Name: nome, Via: "convite"})
	c.Audit("convite.aceitou", email, nil)

	// A confirmação não pode derrubar o cadastro: a conta já existe, e um
	// servidor de e-mail fora do ar não é motivo para dizer que não deu certo.
	if err := correio.Boasvindas(c.Context(), email, nome); err != nil {
		c.Log().Warn("boas-vindas", "email", email, "err", err)
	}
	c.Flash("success", "Sua conta está pronta. Entre com o e-mail e a senha que você acabou de escolher.")
	return c.Redirect("/entrar")
}

func tela(c *trilha.Ctx, nome, email, erro string) h.Node {
	return ui.Card(
		ui.CardHeader(
			ui.CardTitle("Bem-vindo, "+nome),
			ui.CardDescription("Escolha a senha da conta de "+email+"."),
		),
		ui.CardContent(
			h.Form(h.Method("post"),
				trilha.CSRFInput(c),
				ui.Field("senha", "Senha",
					ui.Input(h.ID("senha"), h.Type("password"), h.Name("senha"),
						h.Attr("autocomplete", "new-password"), h.Required()),
					ui.Error(erro)),
				ui.Button(h.Type("submit"), h.Text("Criar minha senha")),
			),
		),
	)
}
