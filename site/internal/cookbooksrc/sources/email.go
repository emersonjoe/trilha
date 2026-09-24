package cookbook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/mail"
)

// Mailer is the one an app holds: built once, at startup, from the
// environment. New opens no connection and reads no file, so it belongs in a
// var and not behind a sync.Once.
var Mailer = mail.New(mail.FromEnv())

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

// Resend is a Transport for a provider that speaks HTTP instead of SMTP. It is
// the whole extension point: one method, so an app that sends through an API
// writes this instead of the framework carrying a driver for every provider.
//
// The message arrives already assembled — headers, multipart, encoding — so
// what is left here is the HTTP call.
type Resend struct {
	Key  string
	HTTP *http.Client
}

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

// SetupMailer picks the transport at startup: the provider when its key is
// there, the environment's answer otherwise.
func SetupMailer() *mail.Mailer {
	o := mail.FromEnv()
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		o.Transport = Resend{Key: key}
	}
	return mail.New(o)
}
