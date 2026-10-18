package trilha

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// discardWriter is a ResponseWriter that keeps nothing, so what the test
// measures is what the framework allocates and not the recorder's copy.
type discardWriter struct {
	hd http.Header
	n  int
}

func (d *discardWriter) Header() http.Header         { return d.hd }
func (d *discardWriter) WriteHeader(int)             {}
func (d *discardWriter) Write(b []byte) (int, error) { d.n += len(b); return len(b), nil }

// Spec 177: a page costs about its own size in memory, not three times it.
// The buffer grew by doubling on every request, and prepending the doctype
// copied the whole document once more.
func TestPageRenderAllocatesAboutItsSize(t *testing.T) {
	if raceDetector {
		t.Skip("under -race sync.Pool drops buffers on purpose; the measure is make test's")
	}
	a := New(Config{Env: Prod, Logger: quiet()})
	para := strings.Repeat("palavra ", 120)
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		items := make([]h.Node, 100)
		for i := range items {
			items[i] = h.P(h.Text(para))
		}
		return h.Div(items...), nil
	}})
	req := httptest.NewRequest("GET", "/", nil)
	w := &discardWriter{hd: http.Header{}}
	a.Handler().ServeHTTP(w, req) // warm up: routes, pools
	size := w.n
	if size < 90_000 {
		t.Fatalf("page is %d bytes, the test needs ~100 KB", size)
	}

	const runs = 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		w := &discardWriter{hd: http.Header{}}
		a.Handler().ServeHTTP(w, req)
	}
	runtime.ReadMemStats(&after)
	perReq := float64(after.TotalAlloc-before.TotalAlloc) / runs
	if perReq > 0.5*float64(size) {
		t.Fatalf("%.0f bytes allocated per request for a %d-byte page (%.1fx)", perReq, size, perReq/float64(size))
	}
}
