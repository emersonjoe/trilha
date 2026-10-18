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
		// The refusal is a new page at the same address: wait for its message,
		// not for the address, or the next Fill types into the page going away.
		s.WantText("main", "Wrong e-mail or password")
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

// An island's module that imports another by a relative path gets the
// versioned URL from the page's import map, so a deploy that changes only the
// imported file is not served from the cache (#298).
func TestUIIslandImportIsVersioned(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/ilha/salto")
		s.WantText("#concluir", "finish")
		s.WaitJS("the relative import went to the versioned URL", `performance.getEntriesByType("resource")`+
			`.some((r) => /\/salto\/motor\.js\?v=/.test(r.name))`)
		if c := s.Console(); len(c) > 0 {
			s.Fail(uitest.Failure{Step: "console", Want: "no errors", Got: strings.Join(c, "; "),
				Fix: "the import map was refused (nonce) or points at a file that is not in public/"})
		}
	})
}

// An island posts to a route.go that redirects: the browser goes to the
// destination, with the message, instead of island.post receiving its HTML
// (#302) — and the post carries the CSRF token the route now asks of a request
// with the session's cookies (#300).
func TestUIIslandPostRedirects(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/ilha/salto")
		s.Click("#concluir")
		s.WantURL("/fluxos/ilha/feito")
		s.WantText("#feito", "Finished")
		s.WaitJS("the flash of the route showed on the destination", `document.body.textContent.includes("Finished.")`)
	})
}

// Client navigation replaces the region, keeps the document, and moves the
// focus to the new content's heading — a screen reader starts reading there.
func TestUIClientNavFocus(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/nav")
		s.Eval(mark, nil)
		s.Click("#ir")
		s.WantURL("/fluxos/nav/b")
		s.WantText("#regiao h1", "Page B")
		s.WantFocus("#regiao h1")
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

// tick reads the number the changing fragment shows.
const tick = `+(/Tick (\d+)/.exec(document.querySelector("#mudando").textContent) || [0, 0])[1]`

// A poll that brings back the same fragment leaves it alone: the island keeps
// its count and the element is the same object, while the fragment that did
// change is replaced (#306) — with its island mounted again, which the swap
// of ui.live.js used to leave on the fallback for good.
func TestUIPollKeepsUnchangedFragment(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/vigia")
		s.WantText("#contador", "count 7")
		s.Click("#contador")
		s.WantText("#contador", "count 8")
		s.Eval(`document.activeElement.blur(); document.querySelector("#parado").__marca = 1;`+
			`document.querySelector("#mudando").__marca = 1; window.__t1 = `+tick, nil)
		// The first answer is compared with the page's own DOM (what the island
		// drew left out), the next ones with the last answer: none may swap.
		s.WaitJS("three ticks went by", tick+` >= window.__t1 + 3`)
		s.WaitJS("the unchanged fragment is the same element", `document.querySelector("#parado").__marca === 1`)
		s.WantText("#contador", "count 8")
		s.WaitJS("the changed fragment was replaced", `document.querySelector("#mudando").__marca === undefined`)
		s.WaitJS("the island of the replaced fragment is mounted again",
			`document.querySelector("#mudando [data-trilha-island]").textContent === "count 100"`)
	})
}

// A long streamed answer costs one layout per frame, not one per token: the
// bubble grew by rebuilding its whole text and the log read scrollHeight —
// a forced layout — after every one of the 1500 tokens (spec 178).
func TestUIChatStreamsWithoutLayoutPerToken(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/conversa")
		s.Eval(`(() => {
			const log = document.getElementById("chat-log");
			const get = Object.getOwnPropertyDescriptor(Element.prototype, "scrollHeight").get;
			window.__reads = 0;
			Object.defineProperty(log, "scrollHeight", { get() { window.__reads++; return get.call(this); } });
		})()`, nil)
		s.Fill("#chat-input", "tell me a long story")
		s.Click("#chat-form button")
		s.WaitJS("the whole answer arrived in the bubble",
			`[...document.querySelectorAll("#chat-log .ui-msg")].some((m) => (m.textContent.match(/w/g) || []).length === 1500)`)
		s.WaitJS("the log scrolled at most once per frame, not once per token", `window.__reads < 150`)
	})
}

// Twenty islands on a page watch the document with one observer, not twenty:
// every change anywhere on the page ran one callback per island to ask
// whether it had left (spec 178).
func TestUIIslandsShareOneObserver(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/ilhas")
		s.WaitJS("the twenty islands mounted",
			`document.querySelectorAll("[data-trilha-mounted] button").length === 20`)
		s.WaitJS("one observer watches the document for all of them", `window.__docObservers === 1`)
	})
}
