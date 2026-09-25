package uitest_test

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// screens is every page of the fixture a signed-in administrator can open:
// the recipes' screens and the patterns. One more screen is one more row.
var screens = []string{
	"/", "/admin", "/admin/usuarios", "/admin/auditoria", "/admin/aprovacoes", "/admin/busca",
	"/billing", "/billing/planos", "/billing/faturas", "/notificacoes", "/notificacoes/fila", "/conexoes",
	"/padroes/lista", "/padroes/formulario", "/padroes/envio", "/fluxos/ilha", "/fluxos/nav", "/fluxos/dica",
}

// formsWithoutToken lists, on the current page, the forms that post without
// the hidden CSRF field — in the DOM the browser built, scripts included.
const formsWithoutToken = `[...document.forms].filter((f) => f.method === "post" && !f.querySelector("[name=_csrf]"))` +
	`.map((f) => f.getAttribute("action") || f.id || "(form)")`

// Every screen, in the browser: the policy refused nothing — every inline
// script and style carries the nonce — and every form that posts carries its
// token. The control page proves the first check sees a refusal when there is
// one.
func TestUISecurityEveryScreen(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/semnonce")
		s.WaitJS("the control page's script was refused", `document.title !== "ran"`)
		if v := s.CSPViolations(); len(v) == 0 || !strings.Contains(strings.Join(v, " "), "script-src") {
			s.Fail(uitest.Failure{Step: "control", Want: "a script-src violation on the page without nonce", Got: strings.Join(v, "; "),
				Fix: "the CSP watch of uitest is not recording; nothing below can be trusted"})
		}

		signIn(s, adminEmail, adminPassword)
		for _, path := range screens {
			s.Navigate(path)
			if v := s.CSPViolations(); len(v) > 0 {
				s.Fail(uitest.Failure{Step: "CSP " + path, Want: "no violation", Got: strings.Join(v, "; "),
					Fix: "E_CSP_NONCE: write inline scripts and styles with trilha.NonceAttr(c), or move them to public/"})
			}
			var bad []string
			s.Eval(formsWithoutToken, &bad)
			if len(bad) > 0 {
				s.Fail(uitest.Failure{Step: "CSRF " + path, Want: "every posting form with _csrf", Got: strings.Join(bad, ", "),
					Fix: "put trilha.CSRFInput(c) inside the form"})
			}
		}
	})
}

// A secret saved through the connections screen never comes back in the
// HTML — not in the list, not in the form that edits it.
func TestUISecretNeverInHTML(t *testing.T) {
	const secret = "sk_uitest_4f9a1c7e2b8d6035_never_shown"
	run(t, func(s *uitest.Session) {
		signIn(s, adminEmail, adminPassword)
		s.Navigate("/conexoes")
		s.Fill("#name", "provedor")
		s.Fill("#url", "https://api.example.com")
		s.Select("#auth", "bearer")
		s.Fill("#secret", secret)
		s.Click(".ui-connections-form button[type=submit]")
		s.WantText("body", "provedor")

		pages := []string{"/conexoes"}
		var edit string
		s.Eval(`[...document.querySelectorAll("a[href*='conexoes?']")].map((a) => a.getAttribute("href"))[0] || ""`, &edit)
		if edit != "" {
			pages = append(pages, edit)
		}
		for _, p := range pages {
			s.Navigate(p)
			var html string
			s.Eval(`document.documentElement.outerHTML`, &html)
			if strings.Contains(html, secret) {
				s.Fail(uitest.Failure{Step: "secret " + p, Want: "the secret absent from the HTML", Got: "it is in the page",
					Fix: "show that a secret is set (ui.SecretFieldWithPresence), never its value"})
			}
		}
	})
}
