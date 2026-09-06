---
title: E-mail
description: One interface the handlers call, SMTP behind it in production, the log in dev, a body from a template, and headers that refuse to be injected.
---

Sending mail is three problems wearing one coat: talking to a server, assembling a message
that is valid, and not sending anything from a test. Only the first is about SMTP.

## The seam

```go
// Mailer is what the handlers call. They never learn which one they got,
// which is the whole point: the test and the dev server do not send mail.
type Mailer interface {
	Send(ctx context.Context, to []string, subject, body string) error
}
```

An interface with one method, defined where it is used. The handlers never learn which
implementation they got, which is the entire point: a test that signs a user up must not send
mail to a real address.

```go
// SetupMailer makes the choice once, at startup. Production without an
// address configured fails to start, which is better than a sign-up that
// silently sends nothing.
func SetupMailer(a *trilha.App) error {
	if a.Env() == trilha.Dev {
		a.Values()["mailer"] = LogMailer{Log: a.Logger()}
		return nil
	}
	addr, from := os.Getenv("SMTP_ADDR"), os.Getenv("SMTP_FROM")
	if addr == "" || from == "" {
		return errors.New("SMTP_ADDR and SMTP_FROM are required outside dev")
	}
	host, _, _ := strings.Cut(addr, ":")
	a.Values()["mailer"] = SMTPMailer{
		Addr: addr,
		From: from,
		Auth: smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASSWORD"), host),
	}
	return nil
}
```

Production without an address configured refuses to start. That is deliberate: a sign-up that
silently sends nothing is discovered by a customer, and a process that will not boot is
discovered by the deploy.

## Sending

```go
// SMTPMailer sends through a real server.
type SMTPMailer struct {
	Addr string // "smtp.example.com:587"
	From string
	Auth smtp.Auth
}
```

```go
// Send hands the message to the server. smtp.SendMail takes no context, so
// the deadline is honoured here: when the request gives up, the handler
// returns and the goroutine finishes on its own.
func (m SMTPMailer) Send(ctx context.Context, to []string, subject, body string) error {
	msg, err := Message(m.From, to, subject, body)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(m.Addr, m.Auth, m.From, to, msg) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

`smtp.SendMail` takes no context, and a mail server that stops answering would otherwise hold
the request until the write timeout. The `select` gives the deadline back to the handler; the
goroutine finishes on its own.

:::note
Port 587 with `PlainAuth` means STARTTLS, and `net/smtp` refuses plain authentication on a
connection that is not encrypted — that refusal is a feature. Port 465 is implicit TLS, which
`net/smtp` does not do on its own: dial with `tls.Dial` and use `smtp.NewClient`.
:::

## The message

```go
// Message assembles the bytes of RFC 5322. A newline inside a header is how
// a form field becomes a second Bcc:, so anything that came from outside is
// refused rather than escaped.
func Message(from string, to []string, subject, body string) ([]byte, error) {
	for _, v := range append([]string{from, subject}, to...) {
		if strings.ContainsAny(v, "\r\n") {
			return nil, errors.New("cookbook: header injection")
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String()), nil
}
```

The loop at the top is the only security check in this file and the one that is usually
missing. A newline inside a header is how a "name" field from a form becomes a second `Bcc:` —
your server, someone else's mailing list. Refusing is right; escaping is a guess.

The body comes from `text/template`, not `html/template`:

```go
// welcome is text/template, not html/template: what is being escaped here
// is nothing, and HTML escaping in a plain-text mail turns an apostrophe
// into &#39;.
var welcome = template.Must(template.New("welcome").Parse(
	`Hello, {{.Name}}.

Your account is ready. Set your password here:
{{.URL}}

This link is good for one hour.
`))
```

```go
// Welcome renders the body and sends it.
func Welcome(ctx context.Context, m Mailer, name, email, link string) error {
	var b strings.Builder
	if err := welcome.Execute(&b, struct{ Name, URL string }{name, link}); err != nil {
		return err
	}
	return m.Send(ctx, []string{email}, "Welcome", b.String())
}
```

HTML escaping in a plain-text mail turns an apostrophe into `&#39;` in somebody's inbox. If
you send a multipart HTML mail, then `html/template` is right for that part — and the plain
one still goes along, because a lot of clients show it.

## In dev

```go
// LogMailer is the implementation for dev and tests: it writes the message
// to the log. Nobody's inbox learns about your fixtures.
type LogMailer struct{ Log *slog.Logger }

```

The whole message in the log, including the link, which is what you actually need when you are
testing a password reset for the fifth time.

:::tip
Two other implementations pay for themselves: one that collects messages in a slice, for
tests to assert on, and one that writes `.eml` files to a directory so you can open them in a
mail client.
:::
