// Package correio is where this app sends e-mail from. It is one file on
// purpose: an application has a handful of messages, and each one deserves a
// function with a name — the handler says "send the invitation", not "build a
// multipart".
//
// The mailer is a package variable so a test can put an Outbox in its place
// and then assert on what was sent. That is the whole testing story of the
// module: no container, no network, no fake SMTP.
package correio

import (
	"context"

	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/mail"
)

// Mailer is what sends. In development TRILHA_MAIL_URL is empty, so the
// invitations end up as .eml files in ./mail and the terminal says where.
var Mailer = mail.New(mail.FromEnv())

// Marca is the name in the header and in the footer of every message.
const Marca = "Acervo"

// Convite is the message somebody receives before they have an account: the
// only thing in it is the link, and the only thing the link needs to say is
// who invited them and until when it works.
func Convite(ctx context.Context, para, nome, quemConvidou, link string) error {
	return Mailer.Send(ctx, mail.Message{
		To:      []string{para},
		Subject: "Você foi convidado para o " + Marca,
		Body: mail.Layout(Marca,
			h.P(h.Textf("%s convidou você para o %s.", quemConvidou, Marca)),
			mail.Button("Criar minha senha", link),
			mail.Muted(h.Text("O convite vale por 48 horas e só pode ser usado uma vez. "+
				"Se o botão não funcionar, cole este endereço no navegador: "+link)),
		),
	})
}

// Boasvindas fecha o ciclo: quem aceitou o convite recebe a confirmação de
// que a conta existe. É a mensagem que evita o chamado "criei a senha, e
// agora?".
func Boasvindas(ctx context.Context, para, nome string) error {
	return Mailer.Send(ctx, mail.Message{
		To:      []string{para},
		Subject: "Sua conta no " + Marca + " está pronta",
		Body: mail.Layout(Marca,
			h.P(h.Textf("Pronto, %s. Sua conta já funciona.", nome)),
			mail.Button("Entrar no "+Marca, "https://acervo.exemplo.com/entrar"),
		),
	})
}
