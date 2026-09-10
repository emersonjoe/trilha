package recipes

// mailRecipe is the file an application's messages live in.
//
// The mail module has existed since 0.63.0 — layout, button, the plain-text
// half, SMTP, and the dev mode that writes .eml files into a folder. What is
// missing is the ordinary thing: one function per message, with a name, so a
// handler says "send the invitation" instead of building a multipart in the
// middle of a request.
func mailRecipe() Recipe {
	return Recipe{
		Name: "mail",
		Summary: map[string]string{
			"en": "the file this app sends e-mail from: one function per message, and a test with no network",
			"pt": "o arquivo de onde este app manda e-mail: uma função por mensagem, e teste sem rede",
		},
		Doc: "/reference/mail",
		Files: []File{
			{Rel: "internal/correio/correio.go", Go: true, Body: mailDecl},
			{Rel: "internal/correio/correio_test.go", Go: true, Body: mailDeclTest},
		},
		Next: map[string]string{
			"en": "In dev nothing leaves: with TRILHA_MAIL_URL empty the messages land as .eml files in " +
				"./mail. Set TRILHA_MAIL_URL and TRILHA_MAIL_FROM for production — `trilha audit` says so " +
				"too. If a message has to survive a crash, send it from a task: it is work outside the " +
				"request, and `trilha add tasks` writes that.",
			"pt": "Em dev não sai nada: com TRILHA_MAIL_URL vazio as mensagens viram .eml em ./mail. " +
				"Defina TRILHA_MAIL_URL e TRILHA_MAIL_FROM para produção — o `trilha audit` avisa isso " +
				"também. Se uma mensagem precisa sobreviver a uma queda, mande de dentro de uma tarefa: é " +
				"trabalho fora da requisição, e o `trilha add tasks` escreve isso.",
		},
	}
}

const mailDecl = `// Package correio is where this application sends e-mail from.
//
// It is one file on purpose: an application has a handful of messages, and
// each one deserves a function with a name — the handler says "send the
// invitation", not "build a multipart". It is also the file somebody who does
// not write Go can be asked to read, which is where the wording gets fixed.
package correio

import (
	"context"

	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/mail"
)

// Mailer is what sends.
//
// It is a package variable, and that is deliberate: it is what lets a test put
// a mail.Outbox in its place and then assert on what was sent. That is the
// whole testing story of this module — no container, no network, no fake SMTP.
//
// In development TRILHA_MAIL_URL is empty, so messages end up as .eml files in
// ./mail and the terminal says where. In production with the variable unset,
// Send answers mail.ErrNotConfigured instead of pretending: silence is the one
// failure nobody notices until somebody asks why the e-mail never arrived.
var Mailer = mail.New(mail.FromEnv())

// Marca is the name in the header and in the footer of every message.
const Marca = "{{.T.mail_brand}}"

// Boasvindas is one message with a name. Copy it for the next one: what makes
// this file worth having is that every message an application sends is here,
// spelled out, instead of assembled inline where nobody reviews it.
func Boasvindas(ctx context.Context, para, nome, link string) error {
	return Mailer.Send(ctx, mail.Message{
		To:      []string{para},
		Subject: "{{.T.mail_welcome_subject}} " + Marca,
		Body: mail.Layout(Marca,
			h.P(h.Textf("{{.T.mail_welcome_hi}}", nome)),
			mail.Button("{{.T.mail_welcome_button}}", link),
			// The address in plain text too: a button that a client does not
			// render is a message that cannot be acted on.
			mail.Muted(h.Text("{{.T.mail_welcome_hint}} "+link)),
		),
	})
}
`

const mailDeclTest = `package correio

import (
	"context"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/mail"
)

// O Outbox no lugar do transporte é a história de teste inteira do módulo: o
// que foi mandado fica em memória, já desmontado, e o teste afirma sobre o
// assunto e sobre o link — não sobre quoted-printable.
func TestBoasvindas(t *testing.T) {
	caixa := &mail.Outbox{}
	antes := Mailer
	Mailer = mail.New(mail.Options{From: "Teste <no-reply@exemplo.com>", Transport: caixa})
	t.Cleanup(func() { Mailer = antes })

	if err := Boasvindas(context.Background(), "ana@exemplo.com", "Ana", "https://exemplo.com/entrar"); err != nil {
		t.Fatal(err)
	}
	msgs := caixa.Messages()
	if len(msgs) != 1 {
		t.Fatalf("mandou %d mensagens", len(msgs))
	}
	m := msgs[0]
	if len(m.To) != 1 || m.To[0] != "ana@exemplo.com" {
		t.Fatalf("para = %v", m.To)
	}
	if !strings.Contains(m.Subject, Marca) {
		t.Fatalf("assunto = %q", m.Subject)
	}
	// O link no corpo é o que a pessoa vai clicar, e é o que quebra em
	// silêncio quando alguém troca a rota: ele entra no teste.
	if !strings.Contains(m.HTML, "https://exemplo.com/entrar") {
		t.Fatalf("o link não está no corpo:\n%s", m.HTML)
	}
	// E o texto alternativo existe: um cliente que não renderiza HTML recebe
	// uma mensagem, e não um espaço em branco.
	if strings.TrimSpace(m.Text) == "" {
		t.Fatal("a mensagem foi sem a versão em texto")
	}
}

// Sem transporte nenhum, Send não finge que enviou.
func TestSemConfiguracaoNaoFinge(t *testing.T) {
	antes := Mailer
	Mailer = mail.New(mail.Options{From: "Teste <no-reply@exemplo.com>"})
	t.Cleanup(func() { Mailer = antes })

	if err := Boasvindas(context.Background(), "ana@exemplo.com", "Ana", "https://exemplo.com/entrar"); err != mail.ErrNotConfigured {
		t.Fatalf("err = %v", err)
	}
}
`
