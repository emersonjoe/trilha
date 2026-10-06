package uitest_test

import (
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// A client navigation into a page whose region brings scripts runs them, the
// way the full page would: the island mounts, the live runtime fills the
// deferred part — and the inline script stays inert, because running HTML
// that came from a response is the road to XSS (#290).
func TestUIClientNavRunsRegionScripts(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/nav")
		s.Eval(mark, nil)
		s.Click("#ir-vivo")
		s.WantURL("/fluxos/nav/vivo")
		s.WantAttr("#regiao [data-trilha-island]", "data-trilha-mounted", "")
		s.WantText("#contador", "count 41")
		s.WantText("#adiado", "Deferred part arrived")
		s.WaitJS("the app's script ran once", `window.__runs === 1`)
		s.WaitJS("the inline script did not run", `window.__inlineRan === undefined`)
		s.WaitJS("the navigation kept the document", stillMarked)

		// The same file, on the next page: once per document, not per page.
		s.Click("#ir-vivo2")
		s.WantText("#regiao h1", "Second live page")
		s.WaitJS("the script did not run a second time", `window.__runs === 1`)
		s.WaitJS("the navigation kept the document", stillMarked)
	})
}

// trilha:before-swap fires on both paths of a swap with the old element still
// on the page — the one moment a page script can take its listeners down.
func TestUIBeforeSwapEvent(t *testing.T) {
	const listen = `window.__before = []; document.addEventListener("trilha:before-swap", (e) => ` +
		`window.__before.push(e.detail.id + ":" + e.detail.target.isConnected))`
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/nav")
		s.Eval(listen, nil)
		s.Click("#ir")
		s.WantText("#regiao h1", "Page B")
		s.WaitJS("the navigation announced the swap first", `window.__before.join() === "regiao:true"`)

		s.Navigate("/padroes/lista")
		s.Eval(listen, nil)
		s.Click("#orders thead a")
		s.WaitJS("the fragment swap announced it first", `window.__before.join() === "orders:true"`)
	})
}

// The client navigation is announced as a full one would be: the focus goes
// to the new heading and the announcer reads the new title. Back announces
// too (#297).
func TestUIClientNavAnnounces(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/nav")
		s.Eval(mark, nil)
		s.Click("#ir")
		s.WantText("#regiao h1", "Page B")
		s.WantFocus("#regiao h1")
		s.WantAttr("#trilha-route-announcer", "aria-live", "assertive")
		s.WantText("#trilha-route-announcer", "Second page")
		s.WaitJS("the announcer is visually hidden", `document.getElementById("trilha-route-announcer").classList.contains("ui-sr")`)

		s.Eval(`history.back()`, nil)
		s.WantText("#regiao h1", "Page A")
		s.WantText("#trilha-route-announcer", "First page")
		s.WaitJS("Back kept the document", stillMarked)
	})
}

// A region marked ui.NavigateFocus("region") keeps the focus on itself after
// a client navigation, as before #297.
func TestUIClientNavFocusRegion(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/nav")
		s.Click("#ir-foco")
		s.WantText("#regiao h1", "Region focus")
		s.WantFocus("#regiao")
	})
}

// Two quick clicks on the pages of a swapped list: the newer one wins, the
// bar and the content agree, there is one new history entry, and the waiting
// mark stays on until the last answer lands (#294).
func TestUISwapNewestClickWins(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/lenta")
		s.Eval(`window.__h = history.length`, nil)
		s.Click("#p2")
		s.WantAttr("#lista", "data-trilha-pending", "")
		// From here, every time the mark comes off is counted: the aborted
		// request must not take it off from under the one still in the air.
		s.Eval(`window.__offs = 0; new MutationObserver((ms) => ms.forEach((m) => { `+
			`if (m.attributeName === "data-trilha-pending" && !m.target.hasAttribute("data-trilha-pending")) window.__offs++; })).`+
			`observe(document.body, {attributes: true, subtree: true, attributeFilter: ["data-trilha-pending"]})`, nil)
		s.Click("#p3")
		s.WantText("#pagina", "page 3")
		s.WantURL("/fluxos/lenta?p=3")
		s.WaitJS("one new history entry", `history.length === window.__h + 1`)
		s.WaitJS("the mark came off once, at the end", `window.__offs <= 1 && !document.getElementById("lista").hasAttribute("data-trilha-pending")`)
	})
}

// Two quick clicks on a save swapped in place send one POST, and the address
// stays where it was (#294, spec 057 SC-003).
func TestUISwapPostOnce(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/lenta")
		s.Eval(`document.querySelector("#salvar button").click(); document.querySelector("#salvar button").click()`, nil)
		s.WantText("#salvos", "saves: 1")
		s.WantURL("/fluxos/lenta")
		s.Navigate("/fluxos/lenta")
		s.WantText("#salvos", "saves: 1")
	})
}
