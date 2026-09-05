package trilha

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// exportMarker guards Export from wiping a directory it did not create.
const exportMarker = ".trilha-export"

// AddExportPath registers extra paths to render on Export, typically pages
// under dynamic routes (e.g. "/blog/ola"). Call it from Setup.
func (a *App) AddExportPath(paths ...string) {
	a.exportExtra = append(a.exportExtra, paths...)
}

// BasePath returns the URL prefix the app is served under ("" or "/docs").
func (a *App) BasePath() string { return a.cfg.BasePath }

// Base returns the app's base path for building links: c.Base()+"/aprender".
func (c *Ctx) Base() string { return c.app.cfg.BasePath }

// Run serves the app, or exports it when TRILHA_EXPORT names a directory.
// The generated main calls this.
func Run(a *App) {
	if dir := os.Getenv("TRILHA_EXPORT"); dir != "" {
		if err := a.Export(dir); err != nil {
			Fatal(err)
		}
		return
	}
	Fatal(a.ListenAndServe())
}

// ExportPaths lists what Export will render: static page routes plus the
// paths added with AddExportPath, sorted and deduplicated.
func (a *App) ExportPaths() []string {
	set := map[string]bool{}
	for p, r := range a.routes {
		if r.Page != nil && !strings.Contains(p, "{") {
			set[p] = true
		}
	}
	for _, p := range a.exportExtra {
		set[p] = true
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Export renders the app as a static site into dir: one index.html per
// path, 404.html from the not-found page, and a copy of public/. The
// directory is emptied first, but only if Export created it before.
func (a *App) Export(dir string) error {
	a.applyConfig()
	if err := prepareExportDir(dir); err != nil {
		return err
	}
	env := a.cfg.Env
	a.cfg.Env = Prod
	defer func() { a.cfg.Env = env }()

	for _, p := range a.ExportPaths() {
		body, code, err := a.render(p)
		if err != nil {
			return err
		}
		if code != http.StatusOK {
			return fmt.Errorf("export: %s respondeu %d", p, code)
		}
		out := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, "/")), "index.html")
		if err := writeFile(out, body); err != nil {
			return err
		}
	}
	body, code, err := a.render("/__trilha_export_not_found__")
	if err != nil {
		return err
	}
	if code != http.StatusNotFound {
		return fmt.Errorf("export: página 404 respondeu %d", code)
	}
	if err := writeFile(filepath.Join(dir, "404.html"), body); err != nil {
		return err
	}
	if a.cfg.Public != nil {
		if err := copyFS(dir, a.cfg.Public); err != nil {
			return err
		}
	}
	a.log.Info("export done", "dir", dir, "pages", len(a.ExportPaths()))
	return nil
}

func (a *App) render(p string) ([]byte, int, error) {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	req := httptest.NewRequest(http.MethodGet, p, nil)
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, req)
	if rec.Code >= 300 && rec.Code < 400 {
		return nil, rec.Code, fmt.Errorf("export: %s redireciona para %s; exporte o destino", p, rec.Header().Get("Location"))
	}
	return rec.Body.Bytes(), rec.Code, nil
}

func prepareExportDir(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case len(entries) > 0:
		if _, err := os.Stat(filepath.Join(dir, exportMarker)); err != nil {
			return fmt.Errorf("export: %s não está vazio e não foi criado pelo trilha; apague-o ou escolha outra pasta", dir)
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, exportMarker), []byte("gerado por trilha export\n"), 0o644)
}

func writeFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, data, 0o644)
}

func copyFS(dir string, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		src, err := fsys.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		f, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, src)
		return err
	})
}
