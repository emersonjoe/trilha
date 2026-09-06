package cookbook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/smtp"
	"os"
	"strings"
	"text/template"

	"github.com/emersonjoe/trilha"
)

// Mailer is what the handlers call. They never learn which one they got,
// which is the whole point: the test and the dev server do not send mail.
type Mailer interface {
	Send(ctx context.Context, to []string, subject, body string) error
}

// SMTPMailer sends through a real server.
type SMTPMailer struct {
	Addr string // "smtp.example.com:587"
	From string
	Auth smtp.Auth
}

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

// LogMailer is the implementation for dev and tests: it writes the message
// to the log. Nobody's inbox learns about your fixtures.
type LogMailer struct{ Log *slog.Logger }

// Send logs what would have been sent.
func (m LogMailer) Send(ctx context.Context, to []string, subject, body string) error {
	m.Log.InfoContext(ctx, "e-mail not sent (dev)", "to", to, "subject", subject, "body", body)
	return nil
}

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

// welcome is text/template, not html/template: what is being escaped here
// is nothing, and HTML escaping in a plain-text mail turns an apostrophe
// into &#39;.
var welcome = template.Must(template.New("welcome").Parse(
	`Hello, {{.Name}}.

Your account is ready. Set your password here:
{{.URL}}

This link is good for one hour.
`))

// Welcome renders the body and sends it.
func Welcome(ctx context.Context, m Mailer, name, email, link string) error {
	var b strings.Builder
	if err := welcome.Execute(&b, struct{ Name, URL string }{name, link}); err != nil {
		return err
	}
	return m.Send(ctx, []string{email}, "Welcome", b.String())
}

// SendWelcome is the other end of the seam: a handler asks for the interface,
// never for the implementation behind it. The type argument is what Provide
// filed the value under, which is why it is written out here — LogMailer and
// SMTPMailer are two answers to the same question.
func SendWelcome(c *trilha.Ctx, name, email, link string) error {
	return Welcome(c.Context(), trilha.Use[Mailer](c), name, email, link)
}

// SetupMailer makes the choice once, at startup. Production without an
// address configured fails to start, which is better than a sign-up that
// silently sends nothing.
func SetupMailer(a *trilha.App) error {
	if a.Env() == trilha.Dev {
		trilha.Provide[Mailer](a, LogMailer{Log: a.Logger()})
		return nil
	}
	addr, from := os.Getenv("SMTP_ADDR"), os.Getenv("SMTP_FROM")
	if addr == "" || from == "" {
		return errors.New("SMTP_ADDR and SMTP_FROM are required outside dev")
	}
	host, _, _ := strings.Cut(addr, ":")
	trilha.Provide[Mailer](a, SMTPMailer{
		Addr: addr,
		From: from,
		Auth: smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASSWORD"), host),
	})
	return nil
}
