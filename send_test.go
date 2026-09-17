package trilha

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// sendCtx is a Ctx over a recorder: enough for a response, which is all the
// sending half of the framework touches.
func sendCtx(method, target string, hdr http.Header) (*Ctx, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, nil)
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	a := &App{}
	return newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindAPI), rec
}

func TestAttachmentNamesTheFileBothWays(t *testing.T) {
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Attachment("relatório \"anual\".csv", strings.NewReader("a,b\n1,2\n"), ""); err != nil {
		t.Fatal(err)
	}
	d := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(d, `attachment; filename="relat_rio _anual_.csv"`) {
		t.Fatalf("fallback ASCII torto: %q", d)
	}
	if !strings.Contains(d, "filename*=UTF-8''relat%C3%B3rio%20%22anual%22.csv") {
		t.Fatalf("faltou o nome UTF-8: %q", d)
	}
	if strings.ContainsAny(d, "\r\n") {
		t.Fatalf("cabeçalho em mais de uma linha: %q", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("download sem nosniff")
	}
}

func TestAttachmentSanitisesThePath(t *testing.T) {
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Attachment("../../etc/passwd", strings.NewReader("x"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.Contains(d, `filename="passwd"`) {
		t.Fatalf("caminho virou nome: %q", d)
	}
}

func TestAttachmentDetectsTheType(t *testing.T) {
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Attachment("doc.pdf", strings.NewReader(pdfBytes), ""); err != nil {
		t.Fatal(err)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/pdf") {
		t.Fatalf("tipo %q", ct)
	}
	if rec.Body.String() != pdfBytes {
		t.Fatalf("corpo perdeu bytes do sniff: %q", rec.Body.String())
	}
}

func TestAttachmentServesARange(t *testing.T) {
	body := strings.Repeat("abcdefghij", 100) // 1000 bytes
	c, rec := sendCtx("GET", "/", http.Header{"Range": {"bytes=0-99"}})
	if err := c.Attachment("big.bin", strings.NewReader(body), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 0-99/1000" {
		t.Fatalf("Content-Range %q", got)
	}
	if rec.Body.Len() != 100 {
		t.Fatalf("%d bytes", rec.Body.Len())
	}
}

func TestAttachmentHeadHasNoBody(t *testing.T) {
	c, rec := sendCtx("HEAD", "/", nil)
	if err := c.Attachment("a.txt", strings.NewReader("hello"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD com corpo: %q", rec.Body.String())
	}
	// Without a Seeker there is nothing to seek, so no range is promised.
	c2, rec2 := sendCtx("HEAD", "/", nil)
	if err := c2.Attachment("a.txt", io.LimitReader(strings.NewReader("hello"), 5), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if rec2.Header().Get("Accept-Ranges") != "" {
		t.Fatal("prometeu Range sem Seeker")
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("HEAD com corpo: %q", rec2.Body.String())
	}
}

func TestAttachmentWithoutSeekerSendsItAll(t *testing.T) {
	c, rec := sendCtx("GET", "/", nil)
	src := io.LimitReader(strings.NewReader(strings.Repeat("x", 2000)), 2000)
	if err := c.Attachment("a.bin", src, ""); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rec.Body.Len() != 2000 {
		t.Fatalf("status %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestInlineRefusesWhatRuns(t *testing.T) {
	for _, ct := range []string{"text/html", "image/svg+xml", "application/xml", "application/xhtml+xml"} {
		c, rec := sendCtx("GET", "/", nil)
		err := c.Inline("x", strings.NewReader("<b>hi</b>"), ct)
		if err == nil {
			t.Fatalf("%s passou como inline", ct)
		}
		if rec.Code == http.StatusOK && rec.Body.Len() > 0 {
			t.Fatalf("%s escreveu corpo antes de recusar", ct)
		}
	}
}

func TestInlineOpensAPDF(t *testing.T) {
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Inline("contrato.pdf", strings.NewReader(pdfBytes), ""); err != nil {
		t.Fatal(err)
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "inline; ") {
		t.Fatalf("disposition %q", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("inline sem nosniff")
	}
}

func TestAttachmentFileOpensAndCloses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/nota.txt", []byte("linha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, rec := sendCtx("GET", "/", nil)
	if err := c.AttachmentFile(dir+"/nota.txt", ""); err != nil {
		t.Fatal(err)
	}
	if rec.Body.String() != "linha\n" {
		t.Fatalf("corpo %q", rec.Body.String())
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatal("arquivo em disco devia aceitar Range")
	}

	c2, _ := sendCtx("GET", "/", nil)
	err := c2.AttachmentFile(dir+"/nao-existe.txt", "")
	var he *HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusNotFound {
		t.Fatalf("arquivo ausente devia ser 404, veio %v", err)
	}
	c3, _ := sendCtx("GET", "/", nil)
	if err := c3.AttachmentFile(dir, ""); err == nil {
		t.Fatal("diretório passou como arquivo")
	}
}

func TestPipeCopiesOnlyTheClosedList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="a.pdf"`)
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Set-Cookie", "session=roubada")
		w.Header().Set("X-Internal", "no")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, pdfBytes)
	}))
	defer srv.Close()
	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Pipe(res); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get("Content-Disposition") != `attachment; filename="a.pdf"` {
		t.Fatalf("disposition %q", rec.Header().Get("Content-Disposition"))
	}
	if rec.Header().Get("ETag") != `"v1"` {
		t.Fatal("ETag não atravessou")
	}
	if rec.Header().Get("Set-Cookie") != "" || rec.Header().Get("X-Internal") != "" {
		t.Fatal("cabeçalho fora da lista atravessou")
	}
	if rec.Body.String() != pdfBytes {
		t.Fatal("corpo diferente")
	}
	if err := res.Body.Close(); err == nil {
		// Closing twice is fine; what matters is that Pipe already did it.
		_ = err
	}
}

func TestPipeKeepsTheStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "no")
	}))
	defer srv.Close()
	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Pipe(res); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestSendOfALargeBody(t *testing.T) {
	const n = 50 << 20
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Attachment("big.bin", io.LimitReader(zeros{}, n), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	if rec.Body.Len() != n {
		t.Fatalf("%d bytes", rec.Body.Len())
	}
}

// zeros is an endless reader: the point is the size of the body, not what is
// in it.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// stream is a body that cannot seek: the shape every answer of another service
// has, and the whole reason Size and ContentRange exist.
type stream struct{ io.Reader }

// rangeBody is 1000 bytes in which every ten-byte window is different, so an
// offset off by one shows up as the wrong bytes and not as the right length.
func rangeBody() string {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "%09d\n", i)
	}
	return b.String()
}

func TestSendCutsARangeOutOfAStream(t *testing.T) {
	body := rangeBody()
	for _, tc := range []struct {
		rng   string
		first int
		last  int
	}{
		{"bytes=10-19", 10, 19},
		{"bytes=990-", 990, 999},
		{"bytes=-10", 990, 999},
		{"bytes=0-1", 0, 1},
		{"bytes=10-100000", 10, 999},
	} {
		c, rec := sendCtx("GET", "/", http.Header{"Range": {tc.rng}})
		err := c.Send("trecho.wav", stream{strings.NewReader(body)}, "audio/wav", SendOpts{
			Inline: true,
			Size:   int64(len(body)),
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.rng, err)
		}
		if rec.Code != http.StatusPartialContent {
			t.Fatalf("%s: status %d", tc.rng, rec.Code)
		}
		want := fmt.Sprintf("bytes %d-%d/1000", tc.first, tc.last)
		if got := rec.Header().Get("Content-Range"); got != want {
			t.Fatalf("%s: Content-Range %q, queria %q", tc.rng, got, want)
		}
		n := tc.last - tc.first + 1
		if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(n) {
			t.Fatalf("%s: Content-Length %q", tc.rng, got)
		}
		if got := rec.Body.String(); got != body[tc.first:tc.last+1] {
			t.Fatalf("%s: corpo %q", tc.rng, got)
		}
		if rec.Header().Get("Accept-Ranges") != "bytes" {
			t.Fatalf("%s: 206 sem Accept-Ranges", tc.rng)
		}
		if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "inline; ") {
			t.Fatalf("%s: o pedaço saiu fora do envelope: %q", tc.rng, d)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: 206 sem nosniff", tc.rng)
		}
	}
}

// The first answer is what teaches the browser the size and the promise: an
// <audio> that never sees Accept-Ranges never asks for a range at all.
func TestSendWithASizePromisesRanges(t *testing.T) {
	body := rangeBody()
	c, rec := sendCtx("GET", "/", nil)
	err := c.Send("trecho.wav", stream{strings.NewReader(body)}, "audio/wav", SendOpts{
		Inline: true,
		Size:   int64(len(body)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatal("sem Accept-Ranges o navegador não pede pedaço")
	}
	if got := rec.Header().Get("Content-Length"); got != "1000" {
		t.Fatalf("Content-Length %q", got)
	}
	if rec.Body.String() != body {
		t.Fatal("corpo diferente")
	}
}

func TestSendAnswers416OutsideTheFile(t *testing.T) {
	body := rangeBody()
	for _, rng := range []string{"bytes=2000-3000", "bytes=1000-", "bytes=abc", "pixels=1-2"} {
		c, rec := sendCtx("GET", "/", http.Header{"Range": {rng}})
		err := c.Send("trecho.wav", stream{strings.NewReader(body)}, "audio/wav", SendOpts{
			Inline: true,
			Size:   int64(len(body)),
		})
		if err != nil {
			t.Fatalf("%s: %v", rng, err)
		}
		if rec.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Fatalf("%s: status %d", rng, rec.Code)
		}
		if got := rec.Header().Get("Content-Range"); got != "bytes */1000" {
			t.Fatalf("%s: Content-Range %q", rng, got)
		}
		if strings.Contains(rec.Body.String(), "000000000") {
			t.Fatalf("%s: o 416 veio com bytes do arquivo", rng)
		}
		if rec.Header().Get("Content-Disposition") != "" {
			t.Fatalf("%s: 416 não entrega arquivo nenhum", rng)
		}
	}
}

// More than one range is answered whole: 200 is a legal answer to any Range,
// and multipart/byteranges over a stream that cannot seek is a lot of work for
// something no <audio> asks for.
func TestSendWithManyRangesSendsItAll(t *testing.T) {
	body := rangeBody()
	c, rec := sendCtx("GET", "/", http.Header{"Range": {"bytes=0-9, 20-29"}})
	err := c.Send("trecho.wav", stream{strings.NewReader(body)}, "audio/wav", SendOpts{
		Inline: true,
		Size:   int64(len(body)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != body {
		t.Fatalf("status %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestSendHeadWithARangeHasNoBody(t *testing.T) {
	body := rangeBody()
	c, rec := sendCtx("HEAD", "/", http.Header{"Range": {"bytes=10-19"}})
	err := c.Send("trecho.wav", stream{strings.NewReader(body)}, "audio/wav", SendOpts{
		Inline: true,
		Size:   int64(len(body)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 10-19/1000" {
		t.Fatalf("Content-Range %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD com corpo: %q", rec.Body.String())
	}
}

// Without a Size nothing changes: the kit cannot invent the total of a
// Content-Range, and promising Range without knowing the size is worse than
// promising nothing.
func TestSendWithoutASizeIgnoresTheRange(t *testing.T) {
	body := rangeBody()
	c, rec := sendCtx("GET", "/", http.Header{"Range": {"bytes=10-19"}})
	if err := c.Send("trecho.wav", stream{strings.NewReader(body)}, "audio/wav", SendOpts{Inline: true}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != body {
		t.Fatalf("status %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec.Header().Get("Accept-Ranges") != "" {
		t.Fatal("prometeu Range sem saber o tamanho")
	}
}

// The body already is the slice: the caller forwarded the Range to the other
// service and is handing back its 206.
func TestSendPassesOnAPartialBody(t *testing.T) {
	c, rec := sendCtx("GET", "/", http.Header{"Range": {"bytes=10-19"}})
	err := c.Send("trecho.wav", stream{strings.NewReader("0123456789")}, "audio/wav", SendOpts{
		Inline:       true,
		ContentRange: "bytes 10-19/4096",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 10-19/4096" {
		t.Fatalf("Content-Range %q", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "10" {
		t.Fatalf("Content-Length %q", got)
	}
	if rec.Body.String() != "0123456789" {
		t.Fatalf("corpo cortado duas vezes: %q", rec.Body.String())
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "inline; ") {
		t.Fatalf("disposition %q", d)
	}
}

func TestSendRefusesABadContentRange(t *testing.T) {
	for _, cr := range []string{"bytes 19-10/4096", "10-19/4096", "bytes 10-19", "bytes a-b/4096", "bytes 10-19/4096\r\nX: y"} {
		c, rec := sendCtx("GET", "/", nil)
		err := c.Send("a.bin", stream{strings.NewReader("x")}, "audio/wav", SendOpts{ContentRange: cr})
		if err == nil {
			t.Fatalf("%q passou", cr)
		}
		if rec.Body.Len() > 0 {
			t.Fatalf("%q escreveu corpo antes de recusar", cr)
		}
	}
	// Two different claims about the same body is a bug, not a precedence.
	c, _ := sendCtx("GET", "/", nil)
	if err := c.Send("a.bin", stream{strings.NewReader("x")}, "audio/wav", SendOpts{Size: 10, ContentRange: "bytes 0-0/10"}); err == nil {
		t.Fatal("Size com ContentRange passou")
	}
}

func TestSendNeutralisesAnSVG(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
	c, rec := sendCtx("GET", "/", nil)
	err := c.Send("marca.svg", strings.NewReader(svg), "image/svg+xml", SendOpts{
		Inline:           true,
		NeutralizeScript: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != svg {
		t.Fatalf("status %d, corpo %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != neutralizedCSP {
		t.Fatalf("CSP %q", got)
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "inline; ") {
		t.Fatalf("disposition %q", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("SVG neutralizado sem nosniff")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("tipo %q", ct)
	}
}

// The option opens the script door and no other: what a viewer shows is a
// different list, and it did not change.
func TestNeutralizeScriptDoesNotOpenTheList(t *testing.T) {
	c, _ := sendCtx("GET", "/", nil)
	err := c.Send("pacote.zip", strings.NewReader("PK"), "application/zip", SendOpts{
		Inline:           true,
		NeutralizeScript: true,
	})
	if !errors.Is(err, ErrCannotInline) {
		t.Fatalf("zip inline devia continuar recusado, veio %v", err)
	}
}

// The screen needs to tell "this content does not show" from "this code is
// wrong", and that is all errors.Is has to answer.
func TestInlineRefusalIsTractable(t *testing.T) {
	c, rec := sendCtx("GET", "/", nil)
	err := c.Inline("marca.svg", strings.NewReader("<svg/>"), "image/svg+xml")
	if !errors.Is(err, ErrCannotInline) {
		t.Fatalf("recusa sem nome: %v", err)
	}
	if !strings.Contains(err.Error(), "image/svg+xml") {
		t.Fatalf("a mensagem não diz o tipo: %v", err)
	}
	if rec.Body.Len() > 0 {
		t.Fatal("escreveu corpo antes de recusar")
	}
}

// #251 — X-Robots-Tag is the one way a file that is not HTML tells an index
// to stay away, and it is what a service writes on a third party's document;
// it crosses. Config.PipeHeaders adds names to the list and never replaces
// it: Set-Cookie stays behind even when somebody lists it.
func TestPipeCarriesTheIndexingDirective(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		w.Header().Set("X-Request-Id", "abc")
		w.Header().Set("Set-Cookie", "session=roubada")
		io.WriteString(w, pdfBytes)
	}))
	defer srv.Close()
	fetch := func() *http.Response {
		res, err := http.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	c, rec := sendCtx("GET", "/", nil)
	if err := c.Pipe(fetch()); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("X-Robots-Tag died at the edge: %v", rec.Header())
	}
	if rec.Header().Get("X-Request-Id") != "" {
		t.Fatal("a header nobody listed travelled")
	}

	c2, rec2 := sendCtx("GET", "/", nil)
	c2.app.cfg.PipeHeaders = []string{"X-Request-Id", "Set-Cookie"}
	if err := c2.Pipe(fetch()); err != nil {
		t.Fatal(err)
	}
	if rec2.Header().Get("X-Request-Id") != "abc" || rec2.Header().Get("X-Robots-Tag") == "" {
		t.Fatalf("PipeHeaders replaced the list instead of adding to it: %v", rec2.Header())
	}
	if rec2.Header().Get("Set-Cookie") != "" {
		t.Fatal("Set-Cookie travelled because somebody listed it")
	}
}
