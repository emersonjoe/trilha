package trilha

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("ADDR", "")
	t.Setenv("TRILHA_ENV", "DEV")
	cfg := ConfigFromEnv()
	if cfg.Addr != ":8080" || cfg.Env != Dev {
		t.Fatalf("%+v", cfg)
	}
	t.Setenv("ADDR", "127.0.0.1:1")
	if cfg := ConfigFromEnv(); cfg.Addr != "127.0.0.1:1" {
		t.Fatal(cfg.Addr)
	}
	t.Setenv("TRILHA_ENV", "")
	if cfg := ConfigFromEnv(); cfg.Env != Prod {
		t.Fatal(cfg.Env)
	}
}

func TestPublicFS(t *testing.T) {
	emb := fstest.MapFS{"public/a.txt": {Data: []byte("embedded")}}
	t.Setenv("TRILHA_ENV", "prod")
	fsys := PublicFS(emb, "public")
	if b, err := readAll(fsys, "a.txt"); err != nil || b != "embedded" {
		t.Fatal(b, err)
	}
	dir := t.TempDir()
	_ = os.WriteFile(dir+"/a.txt", []byte("disk"), 0o644)
	t.Setenv("TRILHA_ENV", "dev")
	if b, _ := readAll(PublicFS(emb, dir), "a.txt"); b != "disk" {
		t.Fatal(b)
	}
	if PublicFS(nil, "nope") != nil {
		t.Fatal("nil embedded and no dir should be nil")
	}
}

func readAll(fsys fs.FS, name string) (string, error) {
	if fsys == nil {
		return "", errors.New("nil fs")
	}
	b, err := fs.ReadFile(fsys, name)
	return string(b), err
}

func TestCtxHelpers(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/q", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		c.Header("X-Test", "1")
		c.SetCookie(&http.Cookie{Name: "s", Value: "v"})
		return c.Text(200, c.Query("a")+"|"+c.Form("b")+"|"+c.RequestID())
	}}})
	rec := get(t, a, "GET", "/q?a=1&b=2", "", nil)
	if rec.Header().Get("X-Test") != "1" || !strings.HasPrefix(rec.Body.String(), "1|2|") || len(rec.Body.String()) < 8 {
		t.Fatal(rec.Body.String())
	}
	if rec.Result().Cookies()[0].Value != "v" {
		t.Fatal("cookie")
	}
}

// Issue #24: the body limit protects every route, so raising it for the one
// route that receives a file must not raise it for the rest.
func TestAllowBodyIsPerRequest(t *testing.T) {
	app := New(Config{Logger: quiet(), CSRFForAPI: false})
	var read int
	app.Register(Route{Pattern: "/api/anexo", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		if c.Query("grande") != "" {
			c.AllowBody(4 << 20)
			if err := c.NoReadDeadline(); err != nil {
				return err
			}
		}
		n, err := io.Copy(io.Discard, c.Request().Body)
		read = int(n)
		if err != nil {
			return err
		}
		return c.JSON(200, map[string]int{"bytes": read})
	}}})
	body := strings.Repeat("x", 2<<20)
	post := func(url string) *httptest.ResponseRecorder {
		read = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/octet-stream")
		app.Handler().ServeHTTP(rec, req)
		return rec
	}
	if rec := post("/api/anexo"); rec.Code != 413 {
		t.Fatalf("default limit must still answer 413, got %d", rec.Code)
	}
	if rec := post("/api/anexo?grande=1"); rec.Code != 200 || read != len(body) {
		t.Fatalf("AllowBody: %d, read %d of %d", rec.Code, read, len(body))
	}
	if rec := post("/api/anexo"); rec.Code != 413 {
		t.Fatalf("the raised limit must not leak to the next request: %d", rec.Code)
	}
}
