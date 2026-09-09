---
title: mail
description: Mailer, Options, Message, Layout, Transport, SMTP, Dir and Outbox — the API of the mail package, with the defaults and what each field changes.
---

`import "github.com/emersonjoe/trilha/mail"` — the handful of messages an internal application
actually sends: the invitation, the link, the notice that a flow finished. The body is an
`h.Node`, the same language the pages are written in, and every message goes out as
`multipart/alternative` with the plain text generated from the same node.

## The mailer

```go
func New(o Options) *Mailer                                  // no network, no files
func FromEnv() Options                                       // from TRILHA_MAIL_URL
func (m *Mailer) Send(ctx context.Context, msg Message) error
```

`New` belongs in a package-level var next to the rest of the app's setup. `Send` blocks until
the server accepted the message or the deadline passed; it is safe for concurrent use.

| `Options` | Default | What it does |
|---|---|---|
| `From string` | — | the sender, `"no-reply@org.br"` or `"Acervo <no-reply@org.br>"` |
| `Transport Transport` | from the environment | where messages go; `nil` in production means `ErrNotConfigured` |
| `Timeout time.Duration` | 10 s | bounds the whole delivery, as a deadline on the context |
| `Logger *slog.Logger` | `slog.Default()` | one line per message, with the subject, the recipients and how long it took |

## The environment

```bash
TRILHA_MAIL_URL='smtp://user:pass@smtp.org.br:587?from=Acervo <no-reply@org.br>'
TRILHA_MAIL_URL='smtps://user:pass@smtp.org.br'      # implicit TLS, port 465
TRILHA_MAIL_FROM='Acervo <no-reply@org.br>'          # or the from= above
TRILHA_MAIL_DIR=mail                                 # dev only, default ./mail
```

Unset, the behaviour depends on `TRILHA_ENV`: **dev** writes `.eml` files into `./mail` and
says where on stderr; **anywhere else** there is no transport and `Send` answers
`ErrNotConfigured`. A production app that quietly writes invitations into a directory is an app
whose users are never invited.

A URL it cannot read is a panic at boot rather than a mailer that does something else. Add
`?insecure_auth=1` — spelled out, because a flag that turns off an encryption check should be
readable in the file that sets it — to authenticate over a clear channel.

## Message

```go
type Message struct {
	To, Cc, Bcc []string
	From        string            // overrides Options.From
	ReplyTo     string
	Subject     string
	Body        h.Node            // the HTML
	Text        string            // generated from Body when empty
	Headers     map[string]string // the ones this struct has no field for
}
```

Every address is parsed before anything is sent, so a bad one fails here with the address in
the message rather than as a 501 three hops away. `Bcc` is a recipient of the envelope and of
no header. `Headers` cannot replace the headers this package writes — `From`, `To`, `Cc`,
`Bcc`, `Subject`, `Date`, `Message-ID`, `MIME-Version`, `Content-Type`,
`Content-Transfer-Encoding` — because a message with two `From` lines is one some servers
reject and others deliver to the wrong person.

Subjects are Q-encoded and bodies quoted-printable: a line of generated HTML goes past the 998
octets SMTP accepts, and a server that wraps it for you wraps it inside a URL.

## The layout

```go
func Layout(brand string, body ...h.Node) h.Node
func Button(label, url string) h.Node
func Muted(children ...h.Node) h.Node
func PlainText(n h.Node) string
```

`Layout` is a centred table, widths in pixels, every rule inline — Outlook renders with Word,
which knows no flexbox, no grid and no float, and Gmail strips `<style>` out of the head. Doing
that once here is what keeps it out of the application. The footer says why the message
arrived, which is the line that stops a legitimate message being reported as spam; its language
comes from `TRILHA_LANG` (then `LC_ALL`, `LC_MESSAGES`, `LANG`), and an app that wants its own
words writes its own layout — it is twenty lines.

`Button` is an `<a>` styled like a button: a form control in an email does nothing, and a link
degrades to a link in the clients that refuse the styling.

`PlainText` is what fills `Message.Text`. Blocks become line breaks, a list becomes one line
per item, `<style>` and `<script>` are dropped, and a link becomes `text <https://…>` — a URL
that vanishes wastes the whole message. Lines are not wrapped, because a wrapped line is a
wrapped URL; quoted-printable already handles the length limit.

## Transports

```go
type Transport interface {
	Deliver(ctx context.Context, from string, to []string, raw []byte) error
}
```

One method, so an application that sends through an HTTP provider writes it instead of the
framework carrying a driver for each one. `raw` is the complete message; `from` and `to` are
the envelope, which is not always what the headers say.

### SMTP

```go
type SMTP struct {
	Addr              string      // host:port
	User, Pass        string      // empty means no authentication
	ImplicitTLS       bool        // implied by port 465
	TLS               *tls.Config // for a private CA — not a place to turn verification off
	AllowInsecureAuth bool
}
```

Port 465 (or `ImplicitTLS`) starts encrypted; anything else negotiates STARTTLS when the server
offers it. **Authentication never happens over a clear channel** unless `AllowInsecureAuth` is
set: a server that offers `PLAIN` unencrypted is misconfigured, not an invitation. `PLAIN` is
preferred and `LOGIN` is used when it is all the server announces — that is Office 365 and a
few appliances, and it is not in the standard library.

The deadline comes from the context and is set on the connection, which is what makes a handler
that gave up hang up. `smtp.SendMail` has none, and that is the invisible bug this replaces: a
slow server pins the handler until TCP notices, minutes later.

### Dir and Outbox

```go
type Dir string        // writes .eml files; the default in development
type Outbox struct{}   // keeps messages in memory, for tests

func (o *Outbox) Messages() []Sent
func (o *Outbox) Last() Sent
func (o *Outbox) Reset()

type Sent struct {
	From    string
	To      []string
	Subject string
	Text    string
	HTML    string
	Raw     []byte
	At      time.Time
}
```

`Dir` writes `.eml` — the format every client opens by double-clicking, so what you check is
the real message and not a rendering made for the check.

`Sent` is already taken apart, so a test asserts on the link inside the body and not on
quoted-printable. `Last` answers a zero `Sent` when nothing was sent, so a test that forgets to
check the count fails on the empty subject rather than panicking.

`Outbox` lives here, in a package that is not a test package, on purpose: it is what
`mail.Sent(t)` would have been, and a non-test package cannot import `testing` — everything
that imports it registers the test flags in every binary of the project.

## Errors

`ErrNotConfigured` is production without a server. It is an error and not a file written
somewhere, because an application that reports success for a message it never sent is one
nobody debugs until a customer asks.

`trilha audit` warns when the code sends mail and `TRILHA_MAIL_URL` is empty.

## Not here

Asynchronous delivery, a persistent queue, retries, DKIM, attachments and IMAP. The first
arrives with the task module; the rest belong to the mail server and the provider, not to a
module of seven hundred lines.
