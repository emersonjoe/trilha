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
// The password need not live in the environment, where docker inspect and
// /proc/<pid>/environ show it. Either the whole URL comes from a file, or the
// URL goes without a password and the password comes from a file — what the
// secrets: of compose and Swarm deliver in /run/secrets:
//
//	TRILHA_MAIL_URL_FILE=/run/secrets/mail_url
//	TRILHA_MAIL_URL=smtps://user@smtp.org.br:465
//	TRILHA_MAIL_PASSWORD_FILE=/run/secrets/mail_password
//
// Files are read once, here, and trimmed (echo leaves a newline). Two sources
// for the same thing is a panic: which one wins is not something to find out
// from a bounce.
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
	raw := strings.TrimSpace(fromFile("TRILHA_MAIL_URL", os.Getenv("TRILHA_MAIL_URL")))
	passFile := os.Getenv("TRILHA_MAIL_PASSWORD_FILE")
	if raw == "" {
		if passFile != "" {
			panic("mail: TRILHA_MAIL_PASSWORD_FILE is set without TRILHA_MAIL_URL")
		}
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
	if passFile != "" {
		if s.Pass != "" {
			panic("mail: TRILHA_MAIL_URL has a password and TRILHA_MAIL_PASSWORD_FILE is set; keep one")
		}
		if s.User == "" {
			panic("mail: TRILHA_MAIL_PASSWORD_FILE is set but TRILHA_MAIL_URL has no user")
		}
		s.Pass = fromFile("TRILHA_MAIL_PASSWORD", "")
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

// fromFile answers value, or the trimmed content of the file name_FILE points
// to. Both set, or a file that cannot be read, is a panic at boot.
func fromFile(name, value string) string {
	file := os.Getenv(name + "_FILE")
	if file == "" {
		return value
	}
	if strings.TrimSpace(value) != "" {
		panic("mail: " + name + " and " + name + "_FILE are both set; keep one")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		panic("mail: " + name + "_FILE: " + err.Error())
	}
	return strings.TrimSpace(string(b))
}
