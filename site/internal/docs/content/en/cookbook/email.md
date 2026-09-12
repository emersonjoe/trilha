---
title: E-mail
description: trilha/mail — the body is an h.Node, the plain text writes itself, dev writes .eml files, and a test asserts on what was sent without a server.
---

Sending mail is three problems wearing one coat: talking to a server, assembling a message
that is valid, and not sending anything from a test. `trilha/mail` answers all three, and the
part worth reading is which decisions it takes away from you.

## Or: `trilha add mail`

```bash
trilha add mail --dry-run
```

```text
  + internal/correio/correio.go
  + internal/correio/correio_test.go

--dry-run: nothing was written
```

Two files and no screen — `mail` has no listing and no route, so it does not touch
`app/setup.go`. What lands is `internal/correio/correio.go`: the `Mailer` variable below,
already built from `mail.FromEnv()`, and a test that proves it against `mail.Outbox` with no
network. What this page adds is everything the recipe cannot decide for you: the message
itself, `mail.Layout` and `mail.Button`, the invitation flow, and how to swap in another
provider through `Transport`. Run the recipe for the file that owns the `Mailer`; write the
messages here, because a recipe cannot guess what your app has to say.

## The mailer

```go
// Mailer is the one an app holds: built once, at startup, from the
// environment. New opens no connection and reads no file, so it belongs in a
// var and not behind a sync.Once.
var Mailer = mail.New(mail.FromEnv())
```

`FromEnv` reads one variable:

```bash
TRILHA_MAIL_URL='smtp://user:senha@smtp.org.br:587?from=Acervo <no-reply@org.br>'
```

Port 587 is STARTTLS, 465 (or `smtps://`) is implicit TLS. With the variable unset the
behaviour depends on where the app is running, and the difference is the point: in dev it
writes `.eml` files into `./mail` and says where; anywhere else `Send` answers
`mail.ErrNotConfigured`.

That asymmetry is deliberate. A production app that quietly files invitations into a
directory is an app whose users are never invited, and nobody finds out for a week.

## Sending

```go
// SendWelcome is what a handler calls. The message is an h.Node — the same
// nodes the pages are written with — and mail.Layout is what keeps the tables
// and the inline CSS out of here.
func SendWelcome(c *trilha.Ctx, nome, email, link string) error {
	return Mailer.Send(c.Context(), mail.Message{
		To:      []string{email},
		Subject: "Sua conta está pronta",
		Body: mail.Layout("Acervo",
			h.P(h.Textf("Olá, %s.", nome)),
			mail.Button("Definir minha senha", link),
			mail.Muted(h.Text("O link vale por uma hora.")),
		),
	})
}
```

`mail.Layout` is a centred table with widths in pixels and every rule inline, because Outlook
renders with Word — no flexbox, no grid, no float — and Gmail strips `<style>` out of the
head. Writing that once here is what keeps it out of your application. `Button` is an `<a>`
styled like a button: a form control in an email does nothing, and a link degrades to a link.

Every message goes out as `multipart/alternative`, and **the plain text is generated from the
same node**:

```
Acervo

Olá, Ana.

Definir minha senha <https://acervo.org.br/convite/abc123>

O link vale por uma hora.
```

A link becomes `text <https://…>` rather than disappearing, which is what makes the message
useful in a client that shows no HTML — and one fewer point of spam score. Write `Message.Text`
yourself only when you want different words there.

Headers travel Q-encoded and bodies quoted-printable, because a line of generated HTML goes
past the 998 octets SMTP accepts, and a server that wraps it for you wraps it in the middle of
a URL. `Bcc` is a recipient of the envelope and of no header. The headers this package writes
cannot be replaced through `Message.Headers`: a message with two `From` lines is one some
servers reject and others deliver to the wrong person.

## The whole flow: an invitation

`examples/local-login` invites somebody who has no account. The link is a capability with a
deadline — `c.Link`, one use, 48 hours — and the e-mail is what delivers it:

```go
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
```

Two things in that example are worth copying. The accept page lives in its own folder, outside
the one that invites: a middleware guards its folder **and everything under it**, so an accept
page under the invite screen would demand the session the invited person does not have yet.
And the address in the message is absolute, built from `TRILHA_BASE_URL` — an e-mail has no
current page to be relative to.

## Testing it

`mail.Outbox` keeps messages in memory, already taken apart. A test asserts on the link inside
the body, not on quoted-printable:

```go
// caixa põe um Outbox no lugar do remetente do app, que é como um aplicativo
// testa e-mail: sem rede, sem contêiner, sem servidor SMTP falso.
func caixa(t *testing.T) *mail.Outbox {
	t.Helper()
	box := &mail.Outbox{}
	antes := correio.Mailer
	correio.Mailer = mail.New(mail.Options{From: "Acervo <no-reply@exemplo.com>", Transport: box})
	t.Cleanup(func() { correio.Mailer = antes })
	return box
}
```

```go
	// O link vive no texto tanto quanto no HTML — é o que faz a mensagem
	// funcionar num cliente que não mostra HTML.
	link := extraiURL(t, msg.Text, "/convite/")
	if !strings.Contains(msg.HTML, link) {
		t.Fatal("o link do texto não é o mesmo do HTML")
	}
```

:::note
It is `Outbox` and not `mail.Sent(t)` because a package that is not a test package cannot
import `testing`: everything that imports it registers the test flags in every binary of the
project, and `-test.v` on a web server is a surprising thing to ship.
:::

## Another provider

`Transport` is one method, and that is the extension point. An app that sends through an HTTP
API writes this instead of the framework carrying a driver for every provider:

```go
// Deliver implements mail.Transport.
func (r Resend) Deliver(ctx context.Context, from string, to []string, raw []byte) error {
	body, err := json.Marshal(map[string]any{"from": from, "to": to, "raw": string(raw)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.Key)
	req.Header.Set("Content-Type", "application/json")
	cli := r.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		detalhe, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("resend: %s: %s", res.Status, detalhe)
	}
	return nil
}
```

```go
// SetupMailer picks the transport at startup: the provider when its key is
// there, the environment's answer otherwise.
func SetupMailer() *mail.Mailer {
	o := mail.FromEnv()
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		o.Transport = Resend{Key: key}
	}
	return mail.New(o)
}
```

The message arrives already assembled — headers, multipart, encoding — so what is left is the
HTTP call.

## What is tested

The unit tests build messages and read them back with `net/mail` and `mime/multipart`, so what
is asserted is what a client would parse and not what this package meant to write.

The SMTP client is exercised against **a real server on a real socket with real TLS**, because
that is the only way to prove the interesting parts: that STARTTLS was actually negotiated,
that the password never left before it, and that `AUTH LOGIN` works with the servers that only
speak it. A fake transport would have proved that the package calls its own methods.

:::warning
`SMTP.AllowInsecureAuth` sends the password over a connection that was never encrypted. It is
off, and it stays off unless somebody types the field: a server that offers `PLAIN` on a clear
channel is a server misconfigured, not a reason to comply.
:::

## What the audit says

`trilha audit` warns when the code sends mail and `TRILHA_MAIL_URL` is empty. The dev mode is
good for developing and a terrible discovery in production.
