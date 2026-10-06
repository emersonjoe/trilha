package uitest_test

import (
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// counted is a JavaScript condition: the server counted exactly want for uri
// ("gets 0 prefetches 1").
func counted(uri, want string) string {
	return `fetch("/fluxos/pre/contagem?u=" + encodeURIComponent("` + uri + `"), {headers: {"Trilha-Fragment": "n"}})` +
		`.then((r) => r.text()).then((t) => t.includes("` + want + `"))`
}

// waited is true once ms milliseconds passed since it was first asked: the
// way to prove that something did not happen, as a condition and not a sleep.
func waited(ms string) string {
	return `(window.__t0 ??= performance.now(), performance.now() - window.__t0 > ` + ms + `)`
}

// Resting the pointer on a link of a region with ui.Prefetch asks for the
// page once, as a prefetch; the click that follows swaps it in without a
// second GET, and says so in trilha:swap (#295).
func TestUIPrefetchOnIntent(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/pre")
		s.Eval(mark, nil)
		s.Eval(`document.addEventListener("trilha:swap", (e) => { window.__pre = e.detail.prefetched; })`, nil)
		s.Hover("#alvo")
		s.WaitJS("one prefetch of the target", counted("/fluxos/pre/alvo", "gets 0 prefetches 1"))
		s.Click("#alvo")
		s.WantText("#regiao h1", "Destination alvo")
		s.WaitJS("the swap says it was prefetched", `window.__pre === true`)
		s.WaitJS("the click asked nothing more", counted("/fluxos/pre/alvo", "gets 0 prefetches 1"))
		s.WaitJS("the document stayed", stillMarked)
	})
}

// A pointer that passes over a link without stopping asks for nothing, and
// neither does a link marked ui.NoPrefetch.
func TestUIPrefetchNeedsIntent(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/pre")
		s.Sweep("#alvo", "#longe")
		s.Hover("#fora")
		s.WaitJS("well past the 80 ms of intent", waited("400"))
		s.WaitJS("no prefetch of a link passed over", counted("/fluxos/pre/alvo", "gets 0 prefetches 0"))
		s.WaitJS("no prefetch of a link kept out", counted("/fluxos/pre/fora", "gets 0 prefetches 0"))
	})
}

// A prefetched page older than its TTL is asked again on the click.
func TestUIPrefetchExpires(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/pre")
		s.Hover("#curto")
		s.WaitJS("one prefetch", counted("/fluxos/pre/curto", "gets 0 prefetches 1"))
		s.Hover("#longe")
		s.WaitJS("past the 300 ms TTL", waited("600"))
		s.Click("#curto")
		s.WantText("#regiao h1", "Destination curto")
		s.WaitJS("the click asked again", counted("/fluxos/pre/curto", "gets 1 prefetches 1"))
	})
}

// With data saver on, nothing is prefetched.
func TestUIPrefetchSaveData(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/pre")
		s.Eval(`Object.defineProperty(navigator, "connection", {value: {saveData: true, effectiveType: "4g"}, configurable: true})`, nil)
		s.Hover("#alvo")
		s.WaitJS("well past the 80 ms of intent", waited("400"))
		s.WaitJS("no prefetch with data saver", counted("/fluxos/pre/alvo", "gets 0 prefetches 0"))
	})
}

// A prefetch answered with a redirect is not kept: the click asks again and
// follows the normal path to the destination.
func TestUIPrefetchRedirectNotKept(t *testing.T) {
	run(t, func(s *uitest.Session) {
		s.Navigate("/fluxos/pre")
		s.Hover("#sai")
		s.WaitJS("one prefetch", counted("/fluxos/pre/sai", "gets 0 prefetches 1"))
		s.Click("#sai")
		s.WantURL("/fluxos/semregiao")
		s.WaitJS("the click asked again", counted("/fluxos/pre/sai", "gets 1 prefetches 1"))
	})
}
