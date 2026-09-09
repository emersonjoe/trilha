package mail

import (
	"net/url"
	"os"
	"strings"
)

// FromEnv builds the Options from the environment, which is what makes the
// same binary write files on a laptop and send mail on a server.
//
//	TRILHA_MAIL_URL=smtp://user:pass@smtp.org.br:587?from=Acervo+<no-reply@org.br>
//	TRILHA_MAIL_URL=smtps://user:pass@smtp.org.br:465
//	TRILHA_MAIL_FROM=Acervo <no-reply@org.br>          (or the from= above)
//	TRILHA_MAIL_DIR=mail                                (dev only, default ./mail)
//
// Unset, it depends on where the app is running, and the difference is the
// point:
//
//   - TRILHA_ENV=dev writes .eml files into ./mail and says where;
//   - anywhere else there is no transport, and Send answers ErrNotConfigured.
//     A production app that quietly writes invitations into a directory is an
//     app whose users are never invited, and nobody finds out for a week.
//
// A URL it cannot read is a panic at boot rather than a mailer that does
// something else: this runs once, at startup, where a wrong value is still
// cheap to notice.
func FromEnv() Options {
	o := Options{From: strings.TrimSpace(os.Getenv("TRILHA_MAIL_FROM"))}
	raw := strings.TrimSpace(os.Getenv("TRILHA_MAIL_URL"))
	if raw == "" {
		if strings.EqualFold(os.Getenv("TRILHA_ENV"), "dev") {
			dir := strings.TrimSpace(os.Getenv("TRILHA_MAIL_DIR"))
			if dir == "" {
				dir = "mail"
			}
			o.Transport = Dir(dir)
			if o.From == "" {
				o.From = "Trilha <no-reply@localhost>"
			}
		}
		return o
	}
	u, err := url.Parse(raw)
	if err != nil {
		panic("mail: TRILHA_MAIL_URL is not a URL: " + err.Error())
	}
	if u.Scheme != "smtp" && u.Scheme != "smtps" {
		panic("mail: TRILHA_MAIL_URL needs smtp:// or smtps://, got " + u.Scheme + "://")
	}
	s := &SMTP{Addr: u.Host, ImplicitTLS: u.Scheme == "smtps"}
	if u.Port() == "" {
		if u.Scheme == "smtps" {
			s.Addr = u.Host + ":465"
		} else {
			s.Addr = u.Host + ":587"
		}
	}
	if u.User != nil {
		s.User = u.User.Username()
		s.Pass, _ = u.User.Password()
	}
	q := u.Query()
	// insecure_auth is spelled out because a flag that turns off an encryption
	// check should be readable in the deployment file that sets it.
	s.AllowInsecureAuth = q.Get("insecure_auth") == "1" || q.Get("insecure_auth") == "true"
	if o.From == "" {
		o.From = strings.TrimSpace(q.Get("from"))
	}
	o.Transport = s
	if o.From == "" {
		panic("mail: no sender — set TRILHA_MAIL_FROM or ?from= in TRILHA_MAIL_URL")
	}
	return o
}
