package uitest_test

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// signIn goes through the login screen of the recipe, as a person does.
func signIn(s *uitest.Session, email, password string) {
	s.Navigate("/entrar")
	s.Fill("#email", email)
	s.Fill("#password", password)
	s.Click("button[type=submit]")
	s.WantURL("/")
}

// The billing screens as its administrator sees them: the subscriptions, a
// plan the server refuses marked on its field, the plan saved and listed, and
// the export offered.
func TestUIBillingScreens(t *testing.T) {
	run(t, func(s *uitest.Session) {
		signIn(s, adminEmail, adminPassword)
		s.Navigate("/billing")
		s.WaitVisible("h1")

		s.Navigate("/billing/planos")
		s.Fill("#nome", "Pro")
		s.Fill("#centavos", "4900")
		s.Fill("#moeda", "xx") // two letters: only the server's len=3 refuses it
		s.Select("#intervalo", "month")
		s.Click("form[action='/billing/planos'] button[type=submit]")
		s.WantAttr("#moeda", "aria-invalid", "true")
		s.WantAttr("#nome", "value", "Pro")
		// The form has no swap: the 422 is a whole page, and the kit still
		// puts the focus on the field that failed (spec 165).
		s.WantFocus("#moeda")

		s.Fill("#moeda", "brl")
		s.Click("form[action='/billing/planos'] button[type=submit]")
		s.WantURL("/billing/planos")
		s.WantText("table", "Pro")

		s.Navigate("/billing/faturas")
		s.WaitVisible("a[href='/billing/faturas/csv']")
	})
}

// The notification preferences: chosen on the screen, saved, and the screen
// shows them back after the redirect.
func TestUINotifyPreferences(t *testing.T) {
	run(t, func(s *uitest.Session) {
		signIn(s, adminEmail, adminPassword)
		s.Navigate("/notificacoes")
		s.Select("#canal", "mail")
		s.Click("#digesto")
		s.Fill("#silencio_de", "22")
		s.Fill("#silencio_ate", "7")
		s.Click("form[action='/notificacoes'] button[type=submit]")
		s.WantURL("/notificacoes")
		s.WantAttr("#silencio_de", "value", "22")
		s.WantAttr("#silencio_ate", "value", "7")
		s.WantAttr("#digesto", "checked", "")
	})
}

// The backoffice door, walked by people: a stranger is sent to the login, a person
// invited by the administrator sets a password through the link and signs
// in — and is still refused, because being somebody is not being an admin.
func TestUIAdminDefaultDeny(t *testing.T) {
	run(t, func(s *uitest.Session) {
		// A stranger's browser is sent to the login, with the way back.
		s.Navigate("/admin")
		s.WantURL("/entrar?next=%2Fadmin")

		signIn(s, adminEmail, adminPassword)
		s.Navigate("/admin/usuarios")
		s.Fill("#email", "leitor@example.com")
		s.Fill("#nome", "Leitor")
		s.Click("form[action='/admin/usuarios'] button[type=submit]")
		link := strings.TrimSpace(s.Text("code"))
		if !strings.HasPrefix(link, "/convite/") {
			s.Fail(uitest.Failure{Step: "invite link", Selector: "code", Want: "/convite/<token>", Got: link,
				Fix: "the users screen shows the link in a flash after the invitation"})
		}

		s.ClearCookies()
		s.Navigate(link)
		s.Fill("#password", "a-password-for-the-reader")
		s.Click("button[type=submit]")
		s.WantURL("/entrar") // the password is set; the session opens by signing in
		signIn(s, "leitor@example.com", "a-password-for-the-reader")
		// Refused inside the app's layout, by the error page the login recipe
		// brings (spec 165) — not the framework's bare one.
		s.Navigate("/admin")
		s.WantText("main h1", "No access")
		s.WaitVisible("header a[href='/']")
		s.Navigate("/admin/usuarios")
		s.WantText("main h1", "No access")
	})
}
