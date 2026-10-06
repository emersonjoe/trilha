package uitest_test

import (
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

const sheetShown = `!document.getElementById("leitor").hidden`

// A link marked ui.SheetOpen opens the panel and asks its address for the
// body only: one GET with the fragment, the preview's frame inside, the focus
// on the panel's title. Another link swaps the body. Without JavaScript the
// same address answers the whole page (#296).
func TestUISheetOpensOnDemand(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/docs")
		s.Eval(mark, nil)
		s.Click("#doc-7")
		s.WaitJS("the panel opened", sheetShown)
		s.WantFocus("#leitor-title")
		s.WantAttr("#doc-7", "aria-expanded", "true")
		s.WantText("#leitor-body .doc", "Document 7")
		s.WaitVisible("#leitor-body iframe")
		s.WaitJS("one GET of the body as a fragment", counted("/fluxos/docs/7#leitor-body", "gets 1 prefetches 0"))
		s.WaitJS("and none of the whole page", counted("/fluxos/docs/7", "gets 0 prefetches 0"))
		s.WantURL("/fluxos/docs")

		s.Click("#doc-8")
		s.WantText("#leitor-body .doc", "Document 8")
		s.WantAttr("#doc-7", "aria-expanded", "false")
		s.WantAttr("#doc-8", "aria-expanded", "true")
		s.WaitJS("the panel did not reload the page", stillMarked)
		s.WaitJS("without JavaScript the link is the document's page", `fetch("/fluxos/docs/7").then((r) => r.status === 200 && r.text()).then((t) => !!t && t.includes("<h1>Document 7</h1>"))`)
	})
}

// The panel lives outside the region: navigating the region leaves it open,
// with the same frame, which the server does not see again.
func TestUISheetSurvivesNavigation(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/docs")
		s.Click("#doc-7")
		s.WaitVisible("#leitor-body iframe")
		s.WaitJS("the frame asked for its file", counted("/fluxos/docs/7/arquivo", "gets 1 prefetches 0"))
		s.Eval(`document.querySelector("#leitor-body iframe").__uitest = 1`, nil)
		s.Click("#ir-outra")
		s.WantText("#regiao h1", "Other list")
		s.WaitJS("the panel is still open", sheetShown)
		s.WaitJS("with the same frame", `document.querySelector("#leitor-body iframe")?.__uitest === 1`)
		s.WaitJS("which did not load again", counted("/fluxos/docs/7/arquivo", "gets 1 prefetches 0"))
	})
}

// Escape closes the panel, empties it and gives the focus back to the link,
// whose aria-expanded follows.
func TestUISheetEscapeRestoresFocus(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/docs")
		s.Click("#doc-7")
		s.WantText("#leitor-body .doc", "Document 7")
		s.Press("Escape")
		s.WaitJS("the panel closed", `document.getElementById("leitor").hidden`)
		s.WantFocus("#doc-7")
		s.WantAttr("#doc-7", "aria-expanded", "false")
		s.WaitJS("and let go of its frame", `!document.querySelector("#leitor-body iframe")`)
	})
}

// On a phone the panel covers the screen, and opening it closes the shell's
// drawer: the two would fight for the same edge.
func TestUISheetOnPhone(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Viewport(375, 812)
		s.Navigate("/fluxos/docs")
		s.Eval(`document.documentElement.classList.add("ui-drawer-open")`, nil)
		s.Click("#doc-7")
		s.WaitJS("the panel covers the width", `document.getElementById("leitor").getBoundingClientRect().width === innerWidth`)
		s.WaitJS("the drawer closed", `!document.documentElement.classList.contains("ui-drawer-open")`)
	})
}

// A dialog trigger with an address and ui.Swap on the dialog's body opens the
// dialog and loads the body, without touching the address (#296, step 1).
func TestUIDialogLoadsOnDemand(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/docs")
		s.Eval(mark, nil)
		s.Click("#ver-7")
		s.WaitJS("the dialog opened", `document.getElementById("ver").open`)
		s.WantText("#ver-body", "Document 7 in a dialog")
		s.WantURL("/fluxos/docs")
		s.WaitJS("the dialog did not reload the page", stillMarked)
	})
}
