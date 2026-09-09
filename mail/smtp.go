package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// SMTP delivers through a mail server. It is net/smtp with the three things
// net/smtp leaves to you: a deadline, the choice between STARTTLS and implicit
// TLS, and an auth that works with servers that only speak LOGIN.
type SMTP struct {
	// Addr is host:port. 465 means implicit TLS; anything else is plain with
	// STARTTLS, which is what 587 and 25 do.
	Addr string
	// User and Pass authenticate. Empty means no authentication, which is
	// what a relay inside the network usually wants.
	User string
	Pass string
	// ImplicitTLS starts the connection encrypted instead of negotiating
	// STARTTLS. Port 465 implies it, and this field is for the server that
	// does the same thing on another port.
	ImplicitTLS bool
	// TLS, when set, replaces the configuration used for STARTTLS and for the
	// implicit connection. It exists for the server with a private CA; it is
	// not a place to turn verification off.
	TLS *tls.Config
	// AllowInsecureAuth sends the password over a connection that was never
	// encrypted. It is off, and it stays off unless someone types this field:
	// PLAIN over a clear channel is the password in the clear, and a server
	// that offers it is a server misconfigured, not a reason to comply.
	AllowInsecureAuth bool
}

// Deliver implements Transport.
func (s *SMTP) Deliver(ctx context.Context, from string, to []string, raw []byte) error {
	host, _, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return fmt.Errorf("mail: address %q: %w", s.Addr, err)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return err
	}
	// The deadline is the context's, which is what makes a handler that gave
	// up hang up: without it a slow server holds the request until TCP
	// notices, minutes later.
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	tlsCfg := s.TLS
	if tlsCfg == nil {
		tlsCfg = &tls.Config{ServerName: host}
	} else if tlsCfg.ServerName == "" {
		tlsCfg = tlsCfg.Clone()
		tlsCfg.ServerName = host
	}
	encrypted := false
	if s.ImplicitTLS || strings.HasSuffix(s.Addr, ":465") {
		conn = tls.Client(conn, tlsCfg)
		encrypted = true
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()

	if !encrypted {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
			encrypted = true
		}
	}
	if s.User != "" {
		if !encrypted && !s.AllowInsecureAuth {
			return errors.New("mail: the server does not offer STARTTLS and the password would travel in the clear (SMTP.AllowInsecureAuth to send it anyway)")
		}
		if err := c.Auth(s.auth(host)); err != nil {
			return fmt.Errorf("mail: authenticating: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mail: MAIL FROM %s: %w", from, err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("mail: RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// auth picks what the server can do. PLAIN is the standard and what almost
// everything speaks; LOGIN is the one Office 365 and a few appliances insist
// on, and it is not in the standard library.
func (s *SMTP) auth(host string) smtp.Auth {
	return &autoAuth{user: s.User, pass: s.Pass, host: host, insecure: s.AllowInsecureAuth}
}

type autoAuth struct {
	user, pass, host string
	insecure         bool // the caller accepted a clear channel
	login            bool // the server chose LOGIN
	step             int
}

func (a *autoAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// Second guard, and not a duplicate of the one in Deliver: that one knows
	// what the server offered, this one knows what was actually negotiated,
	// and net/smtp calls this one for every method it tries.
	if !server.TLS && !a.insecure {
		return "", nil, errors.New("mail: refusing to authenticate over an unencrypted connection (SMTP.AllowInsecureAuth to send it anyway)")
	}
	for _, m := range server.Auth {
		if strings.EqualFold(m, "PLAIN") {
			return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
		}
	}
	for _, m := range server.Auth {
		if strings.EqualFold(m, "LOGIN") {
			a.login = true
			return "LOGIN", nil, nil
		}
	}
	// Nothing announced: PLAIN is the safe guess, and the server answers 504
	// if it disagrees — which is a clearer failure than picking nothing.
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
}

func (a *autoAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	if !a.login {
		return nil, errors.New("mail: the server asked for more and this method has nothing else to say")
	}
	a.step++
	switch a.step {
	case 1:
		return []byte(a.user), nil
	case 2:
		return []byte(a.pass), nil
	}
	return nil, errors.New("mail: LOGIN asked for a third answer")
}
