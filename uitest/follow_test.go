package uitest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// countToasts counts, from now on, every toast that appears with text — a
// message shown twice is the bug, and one that fades is still counted.
func countToasts(s *uitest.Session, text string) {
	s.Eval(`window.__toasts = 0; new MutationObserver((ms) => ms.forEach((m) => m.addedNodes.forEach((n) => { `+
		`if (n.nodeType === 1 && (n.matches(".ui-toast") ? [n] : [...n.querySelectorAll(".ui-toast")]).some((t) => t.textContent.includes(`+
		"`"+text+"`"+`))) window.__toasts++; }))).observe(document.body, {childList: true, subtree: true})`, nil)
}

const noToastOnScreen = `![...document.querySelectorAll(".ui-toast")].some((t) => t.textContent.includes("Item added"))`

// A ui.Swap form whose route answers c.Flash + c.Redirect, on a page with a
// client-navigation region, follows the redirect in place: the document
// stays, the region and the bar go to the destination, the toast shows once,
// a reload does not show it again, and Back does not post again (#291).
func TestUISwapFollowsRedirect(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens/novo")
		s.Eval(mark, nil)
		countToasts(s, "Item added: gamma")
		s.Fill("#form-segue-name", "gamma")
		s.Click("#form-segue button")
		s.WantURL("/fluxos/itens")
		s.WantText("#regiao h1", "Items")
		s.WantText("#lista", "gamma")
		s.WaitJS("the toast showed once", `window.__toasts === 1`)
		s.WaitJS("the document stayed", stillMarked)

		s.Eval(`history.back()`, nil)
		s.WantURL("/fluxos/itens/novo")
		s.WantText("#regiao h1", "New item")
		s.WaitJS("Back did not post again", `document.getElementById("form-segue-name").value === ""`)

		s.Navigate("/fluxos/itens")
		s.WantText("#lista", "gamma")
		s.WaitJS("a reload does not repeat the message", noToastOnScreen)
	})
}

// RedirectReload loads the destination whole, and the message comes with it.
func TestUISwapRedirectReload(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens/novo")
		s.Eval(mark, nil)
		s.Fill("#form-recarrega-name", "delta")
		s.Click("#form-recarrega button")
		s.WaitJS("the page loaded for real", `window.__uitest === undefined && document.readyState === "complete"`)
		s.WantURL("/fluxos/itens")
		s.WantText(".ui-toaster", "Item added: delta")
	})
}

// A destination without the region loads whole, and the message the
// followed answer carried is shown there once.
func TestUISwapFollowFallsBack(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens/novo")
		s.Eval(mark, nil)
		s.Fill("#form-fora-name", "epsilon")
		s.Click("#form-fora button")
		s.WaitJS("the page loaded for real", `window.__uitest === undefined && document.readyState === "complete"`)
		s.WantURL("/fluxos/semregiao")
		s.WantText(".ui-toaster", "Item added: epsilon")
		s.WaitJS("the message is there once", `[...document.querySelectorAll(".ui-toast")].filter((t) => t.textContent.includes("epsilon")).length === 1`)
		s.Navigate("/fluxos/semregiao")
		s.WaitJS("and not again", `![...document.querySelectorAll(".ui-toast")].some((t) => t.textContent.includes("epsilon"))`)
	})
}

// An upload whose route redirects follows the same way (#291).
func TestUIUploadFollowsRedirect(t *testing.T) {
	file := filepath.Join(t.TempDir(), "zeta.txt")
	if err := os.WriteFile(file, []byte("zeta"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens/anexo")
		s.Eval(mark, nil)
		s.Upload("#arquivo", file)
		s.Click("#envio button")
		s.WantURL("/fluxos/itens")
		s.WantText("#lista", "zeta.txt")
		s.WantText(".ui-toaster", "Attached: zeta.txt")
		s.WaitJS("the document stayed", stillMarked)
	})
}

// A plain form inside the region navigates in place: a GET filter is a new
// address with Back to the previous filter, a POST with a redirect lands on
// the destination with one GET and the toast once, and Back does not post
// again (#292).
func TestUIRegionFormsNavigate(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens")
		s.Eval(mark, nil)
		s.Fill("#q", "alp")
		s.Click("#filtro button")
		s.WantURL("/fluxos/itens?q=alp")
		s.WaitJS("the list is filtered", `!document.getElementById("lista").textContent.includes("beta")`)
		s.Eval(`history.back()`, nil)
		s.WantURL("/fluxos/itens")
		s.WantText("#lista", "beta")
		s.WaitJS("the filter kept the document", stillMarked)

		s.Click("#ir-novo")
		s.WantText("#regiao h1", "New item")
		countToasts(s, "Item added: eta")
		s.Fill("#form-simples-name", "eta")
		s.Click("#form-simples button")
		s.WantURL("/fluxos/itens")
		s.WantText("#lista", "eta")
		s.WaitJS("the toast showed once", `window.__toasts === 1`)
		s.WaitJS("the post kept the document", stillMarked)
		views := s.Text("#visitas")
		s.Eval(`history.back()`, nil)
		s.WantURL("/fluxos/itens/novo")
		s.WaitJS("Back did not post again", `document.getElementById("form-simples-name").value === ""`)
		s.Eval(`history.forward()`, nil)
		s.WantURL("/fluxos/itens")
		s.WaitJS("the destination is a fresh GET, one per visit", `document.getElementById("visitas").textContent !== `+"`"+views+"`")
	})
}

// A plain form refused with a whole 422 page swaps the region, puts the focus
// on the field and leaves the address alone (#292).
func TestUIRegionForm422(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens/novo")
		s.Eval(mark, nil)
		s.Click("#form-simples button")
		s.WantAttr("#form-simples-name", "aria-invalid", "true")
		s.WantFocus("#form-simples-name")
		s.WantURL("/fluxos/itens/novo")
		s.WaitJS("the refusal kept the document", stillMarked)
	})
}

// A link inside the region to an address that redirects is one GET, and the
// destination's message shows (#292).
func TestUIRegionLinkRedirect(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/itens")
		s.Eval(mark, nil)
		before := s.Text("#visitas")
		s.Click("#ir-antigo")
		s.WantURL("/fluxos/itens")
		s.WantText(".ui-toaster", "This address moved")
		s.WaitJS("the redirect kept the document", stillMarked)
		s.WaitJS("one GET of the destination", `document.getElementById("visitas").textContent === "views: " + (parseInt(`+"`"+before+"`"+`.split(" ")[1]) + 1)`)
	})
}

// The server says which address the fragment drew: a POST that pushes it,
// a link whose canonical address takes its place, and searches that add an
// entry only when asked (#293).
func TestUIServerDeclaresURL(t *testing.T) {
	run(t, func(s *uitest.Session) {
		// Each block starts on a fresh entry with nothing ahead of it:
		// navigating to the address already in the bar would replace the
		// entry, and the counts below would be off by the forward ones.
		s.Navigate("/fluxos/itens/painel")
		s.Eval(`window.__h = history.length`, nil)
		s.Click("#canonico")
		s.WantText("#painel", "document 3")
		s.WantURL("/fluxos/itens/painel?doc=3")
		s.WaitJS("one new entry, with the canonical address", `history.length === window.__h + 1`)

		s.Navigate("/fluxos/itens/painel")
		s.Eval(`window.__h = history.length`, nil)
		for _, q := range []string{"1", "2"} {
			s.Fill("#busca-q", q)
			s.Click("#busca button")
			s.WantText("#painel", "document "+q)
		}
		s.WaitJS("a search replaces its entry", `history.length === window.__h`)
		for _, q := range []string{"4", "5"} {
			s.Fill("#busca-push-q", q)
			s.Click("#busca-push button")
			s.WantText("#painel", "document "+q)
		}
		s.WaitJS("with PushHistory, one entry per search", `history.length === window.__h + 2`)

		s.Navigate("/fluxos/itens/painel")
		s.Click("#criar button")
		s.WantText("#painel", "document 7")
		s.WantURL("/fluxos/itens/painel?doc=7")
		s.Eval(`history.back()`, nil)
		s.WantURL("/fluxos/itens/painel")
		s.WantText("#painel", "no document")
	})
}
