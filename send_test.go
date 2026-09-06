package trilha

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
