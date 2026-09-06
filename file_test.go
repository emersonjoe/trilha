package trilha

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00"
	pdfBytes = "%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n"
)

// upload builds a multipart request the way a browser would, and returns the
// Ctx a handler would see.
func upload(field, filename, content string) *Ctx {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if field != "" {
		// CreateFormFile escapes the name, so a name with a separator still
		// travels — which is the point of the sanitising test.
		fw, _ := w.CreateFormFile(field, filename)
		fw.Write([]byte(content))
	}
	w.WriteField("outro", "campo")
	w.Close()
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return &Ctx{r: req, app: &App{cfg: Config{MaxBodyBytes: 8 << 20}}}
}

func fieldErrs(t *testing.T, err error) FieldErrors {
	t.Helper()
	if err == nil {
		t.Fatal("wanted an error, got nil")
	}
	fe, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("wanted FieldErrors, got %T: %v", err, err)
	}
	return fe
}

func TestFileTooBig(t *testing.T) {
	c := upload("arquivo", "foto.png", pngBytes+strings.Repeat("x", 4096))
	up, err := c.File("arquivo", FileRules{MaxSize: 1024})
	if up != nil {
		t.Fatalf("wanted no upload, got %+v", up)
	}
	if got := fieldErrs(t, err)["arquivo"]; !strings.Contains(got, "1 KB") {
		t.Fatalf("message %q does not name the limit", got)
	}
}

func TestFileTypeComesFromTheContent(t *testing.T) {
	// A PDF renamed to .png and announced as an image is still a PDF.
	c := upload("arquivo", "foto.png", pdfBytes)
	if _, err := c.File("arquivo", FileRules{Accept: []string{"image/png"}}); fieldErrs(t, err)["arquivo"] == "" {
		t.Fatal("wanted a message for the lied type")
	}

	c = upload("arquivo", "foto.png", pdfBytes)
	up, err := c.File("arquivo", FileRules{Accept: []string{"application/pdf"}})
	if err != nil {
		t.Fatalf("wanted the PDF accepted: %v", err)
	}
	defer up.Close()
	if up.MIME != "application/pdf" || up.Ext != ".pdf" {
		t.Fatalf("MIME %q, Ext %q", up.MIME, up.Ext)
	}
	// The file was sniffed, so it has to be back at the start.
	buf := make([]byte, 8)
	if n, _ := up.File.Read(buf); string(buf[:n]) != "%PDF-1.7" {
		t.Fatalf("file is not at the start: %q", buf[:n])
	}
}

func TestFileAcceptWildcard(t *testing.T) {
	c := upload("arquivo", "foto.png", pngBytes)
	up, err := c.File("arquivo", FileRules{Accept: []string{"image/*"}})
	if err != nil {
		t.Fatalf("image/* should accept a PNG: %v", err)
	}
	up.Close()
}

func TestFileNameIsSanitised(t *testing.T) {
	for _, tc := range []struct{ sent, want string }{
		{"../../etc/passwd", "passwd"},
		{`..\..\windows\system32\cmd.exe`, "cmd.exe"},
		{"relatório final.pdf", "relatório final.pdf"},
	} {
		c := upload("arquivo", tc.sent, pdfBytes)
		up, err := c.File("arquivo", FileRules{})
		if err != nil {
			t.Fatalf("%q: %v", tc.sent, err)
		}
		if up.Name != tc.want {
			t.Errorf("%q became %q, wanted %q", tc.sent, up.Name, tc.want)
		}
		up.Close()
	}
}

// A name with a control character or 300 characters does not survive a
// multipart header, but it survives a client that writes the request by hand.
func TestSafeName(t *testing.T) {
	for _, tc := range []struct{ sent, want string }{
		{"nota\x00\n.pdf", "nota.pdf"},
		{"...", "file"},
		{"", "file"},
		{"/", "file"},
		{strings.Repeat("a", 300) + ".pdf", strings.Repeat("a", 96) + ".pdf"},
		{strings.Repeat("a", 300), strings.Repeat("a", 100)},
	} {
		if got := safeName(tc.sent); got != tc.want {
			t.Errorf("safeName(%q) = %q, wanted %q", tc.sent, got, tc.want)
		}
	}
}

func TestFileMissing(t *testing.T) {
	c := upload("", "", "")
	if _, err := c.File("arquivo", FileRules{}); fieldErrs(t, err)["arquivo"] != ValidationMessages["required"] {
		t.Fatalf("wanted the required message, got %v", err)
	}
	c = upload("", "", "")
	up, err := c.File("arquivo", FileRules{Optional: true})
	if up != nil || err != nil {
		t.Fatalf("optional and absent should be (nil, nil), got (%v, %v)", up, err)
	}
}

func TestFileSaveStaysInTheDirectory(t *testing.T) {
	dir := t.TempDir()
	c := upload("arquivo", "../../nota.pdf", pdfBytes)
	up, err := c.File("arquivo", FileRules{})
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	path, err := up.Save(filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "uploads", "nota.pdf"); path != want {
		t.Fatalf("saved at %q, wanted %q", path, want)
	}
	if b, _ := os.ReadFile(path); string(b) != pdfBytes {
		t.Fatalf("content on disk: %q", b)
	}
	// Windows has no Unix mode bits: os.Stat reports 0666 for any writable file,
	// so 0600 is only an assertion where it means something.
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}

	// The second file with the same name does not eat the first one.
	c2 := upload("arquivo", "nota.pdf", pdfBytes+"segundo")
	up2, err := c2.File("arquivo", FileRules{})
	if err != nil {
		t.Fatal(err)
	}
	defer up2.Close()
	path2, err := up2.Save(filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	if path2 == path {
		t.Fatal("the second save overwrote the first")
	}
	if b, _ := os.ReadFile(path); string(b) != pdfBytes {
		t.Fatalf("the first file changed: %q", b)
	}
}

func TestFileMessagesFollowTheLocale(t *testing.T) {
	old := ValidationMessages
	UseValidationPTBR()
	defer func() { ValidationMessages = old; BindInvalid = "invalid value" }()

	c := upload("arquivo", "foto.png", pdfBytes)
	if got := fieldErrs(t, mustErr(c.File("arquivo", FileRules{Accept: []string{"image/png"}})))["arquivo"]; !strings.Contains(got, "não permitido") {
		t.Fatalf("message in English: %q", got)
	}
}

func mustErr(_ *Upload, err error) error { return err }

// uploads builds a multipart request with several files under the same name,
// which is what a dropzone without JavaScript sends.
func uploads(field string, files ...[2]string) *Ctx {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, f := range files {
		fw, _ := w.CreateFormFile(field, f[0])
		fw.Write([]byte(f[1]))
	}
	w.Close()
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return &Ctx{r: req, app: &App{cfg: Config{MaxBodyBytes: 8 << 20}}}
}

func TestFilesReadsEveryFileInOrder(t *testing.T) {
	c := uploads("arquivos", [2]string{"a.png", pngBytes}, [2]string{"b.pdf", pdfBytes})
	ups, err := c.Files("arquivos", FileRules{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll(ups)
	if len(ups) != 2 || ups[0].Name != "a.png" || ups[1].Name != "b.pdf" {
		t.Fatalf("%+v", ups)
	}
	if ups[0].MIME != "image/png" || ups[1].MIME != "application/pdf" {
		t.Fatalf("types: %q %q", ups[0].MIME, ups[1].MIME)
	}
}

// A message about the second file names the second file: the queue shows it on
// the line that is wrong.
func TestFilesNameTheFileThatFailed(t *testing.T) {
	c := uploads("arquivos", [2]string{"a.png", pngBytes}, [2]string{"b.pdf", pdfBytes})
	ups, err := c.Files("arquivos", FileRules{Accept: []string{"image/*"}})
	if ups != nil {
		t.Fatalf("wanted no upload, got %+v", ups)
	}
	errs := fieldErrs(t, err)
	if errs["arquivos[1]"] == "" || errs.Has("arquivos[0]") || errs.Has("arquivos") {
		t.Fatalf("%+v", errs)
	}
}

func TestFilesCountAndOptional(t *testing.T) {
	c := uploads("arquivos", [2]string{"a.png", pngBytes}, [2]string{"b.png", pngBytes})
	if _, err := c.Files("arquivos", FileRules{MaxFiles: 1}); !strings.Contains(fieldErrs(t, err)["arquivos"], "1") {
		t.Fatalf("MaxFiles não recusou a lista: %v", err)
	}

	empty := uploads("outro")
	if _, err := empty.Files("arquivos", FileRules{}); !fieldErrs(t, err).Has("arquivos") {
		t.Fatal("campo vazio devia ser obrigatório")
	}
	ups, err := uploads("outro").Files("arquivos", FileRules{Optional: true})
	if err != nil || len(ups) != 0 {
		t.Fatalf("optional: %v %+v", err, ups)
	}
}

// The empty file input a browser still sends is not a file.
func TestFilesIgnoresTheEmptyInput(t *testing.T) {
	c := uploads("arquivos", [2]string{"", ""}, [2]string{"a.png", pngBytes})
	ups, err := c.Files("arquivos", FileRules{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll(ups)
	if len(ups) != 1 || ups[0].Name != "a.png" {
		t.Fatalf("%+v", ups)
	}
}
