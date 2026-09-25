package uitest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// The login of the recipe, in the browser: a wrong password stays on the form
// with the message, the right one lands on the home, and the backoffice opens.
func TestUILoginFlow(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/entrar")
		s.Fill("#email", adminEmail)
		s.Fill("#password", "not-the-password")
		s.Click("button[type=submit]")
		s.WantText("[role=alert]", "")
		s.WantURL("/entrar")

		s.Fill("#password", adminPassword)
		s.Click("button[type=submit]")
		s.WantURL("/")

		s.Navigate("/admin")
		s.WantText("h1", "Admin")
	})
}

// mark puts a value on window that only a full reload takes away: a swap or a
// client navigation keeps it, and that is how a scenario tells them apart.
const mark, stillMarked = `window.__uitest = 1`, `window.__uitest === 1`

// The async form of spec 163: a 422 swaps the form in place and the focus
// lands on the field that failed; fixed, the save navigates.
func TestUIAsyncForm422Focus(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/padroes/formulario")
		s.Eval(mark, nil)
		// The name is left empty: the server's validation refuses it. (An
		// invalid e-mail would be the browser's own check, before any request.)
		s.Fill("#email", "ana@example.com")
		s.Click("#profile button[type=submit]")
		s.WantAttr("#name", "aria-invalid", "true")
		s.WantFocus("#name")
		s.WaitJS("the form swapped without a reload", stillMarked)

		s.Fill("#name", "Ana Lima")
		s.Click("#profile button[type=submit]")
		s.WaitJS("the save navigated for real", `window.__uitest === undefined`)
		s.WantURL("/padroes/formulario")
	})
}

// The list of spec 163: ordering asks for the fragment, swaps the table in
// place and puts the address in the bar — no reload.
func TestUIFragmentSwap(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/padroes/lista")
		s.Eval(mark, nil)
		s.Click("#orders thead a")
		s.WaitJS("the address has the ordering", `location.search.includes("sort=")`)
		s.WantText("#orders tbody", "Customer")
		s.WaitJS("the table swapped without a reload", stillMarked)
	})
}

// The upload of spec 163: the bar reports the bytes as they leave, a refused
// file comes back in the same panel with the focus on the field, and an
// accepted one navigates.
func TestUIUploadProgress(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "doc.pdf")
	txt := filepath.Join(dir, "notes.txt")
	body := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("0"), 512<<10)...)
	if err := os.WriteFile(pdf, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(txt, []byte("plain text, not accepted"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, func(s *uitest.Session) {
		s.Navigate("/padroes/envio")
		s.Eval(mark, nil)
		s.Upload("#files", txt)
		s.Click("#upload button[type=submit]")
		s.WantAttr("#files", "aria-invalid", "true")
		s.WantFocus("#files")
		s.WaitJS("the refusal swapped without a reload", stillMarked)

		// The progress events go to sessionStorage, which outlives the
		// navigation that ends an accepted upload.
		s.Eval(`document.addEventListener("trilha:upload", (e) => sessionStorage.setItem("sent", `+
			`JSON.stringify([e.detail.loaded, e.detail.total])))`, nil)
		s.Upload("#files", pdf)
		s.Click("#upload button[type=submit]")
		s.WaitJS("the accepted upload navigated", `window.__uitest === undefined && document.readyState === "complete"`)
		s.WantURL("/padroes/envio")
		s.WaitVisible("#upload #files")
		s.WaitJS("the bar reported every byte of the file", `(() => { const [loaded, total] = `+
			`JSON.parse(sessionStorage.getItem("sent") || "[0,0]"); return total > 512 * 1024 && loaded === total; })()`)
	})
}

// An island mounts over its fallback, receives its props and answers clicks.
func TestUIIslandHydrates(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/ilha")
		s.WantAttr("[data-trilha-island]", "data-trilha-mounted", "")
		s.WantText("#contador", "count 41")
		s.Click("#contador")
		s.WantText("#contador", "count 42")
		if c := s.Console(); len(c) > 0 {
			s.Fail(uitest.Failure{Step: "console", Want: "no errors", Got: strings.Join(c, "; "),
				Fix: "the island's module threw or was refused: check its default export and the CSP"})
		}
	})
}

// Client navigation replaces the region, keeps the document, and moves the
// focus to the new content — a screen reader starts reading there.
func TestUIClientNavFocus(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/nav")
		s.Eval(mark, nil)
		s.Click("#ir")
		s.WantURL("/fluxos/nav/b")
		s.WantText("#regiao h1", "Page B")
		s.WantFocus("#regiao")
		s.WaitJS("the navigation kept the document", stillMarked)
	})
}

// A tooltip opens on keyboard focus, is tied to its target by
// aria-describedby, and closes with Escape.
func TestUITooltipKeyboard(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/dica")
		s.Focus("#copiar")
		s.WantAttr("#copiar", "aria-describedby", "")
		tip := "#" + s.Attr("#copiar", "aria-describedby")
		s.WaitVisible(tip)
		s.WantAttr(tip, "role", "tooltip")
		s.WantText(tip, "Copies the address")
		s.Press("Escape")
		s.WaitJS("the tooltip closed", `(() => { const el = document.querySelector(`+"`"+tip+"`"+`); `+
			`return !el || el.hidden || getComputedStyle(el).display === "none" || getComputedStyle(el).visibility === "hidden"; })()`)
	})
}

// The pager works from the keyboard: Enter on "Next" swaps the page in, and
// the focus stays in the pager instead of falling back to the top of the
// document.
func TestUIPaginationKeyboard(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/padroes/lista")
		s.Eval(mark, nil)
		s.Focus("#orders a[rel=next]")
		s.Press("Enter")
		s.WaitJS("the address is on page 2", `new URLSearchParams(location.search).get("page") === "2"`)
		s.WantText("#orders [aria-current=page]", "2")
		s.WantFocus("#orders a[rel=next]")
		s.WaitJS("the page swapped without a reload", stillMarked)
	})
}
