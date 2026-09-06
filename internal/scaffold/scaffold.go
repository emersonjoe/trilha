// Package scaffold writes a new project from embedded templates.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

//go:embed templates
var templates embed.FS

// Data fills the templates.
type Data struct {
	Module string
	Name   string
	Lang   string // "en" (default) or "pt": language of the generated texts
	// Template is the shape of the project: "blog" (default) is the small
	// site every framework starts you with; "app" is the management app —
	// login, shell, dashboard and a listing — which is where most people
	// were going anyway.
	Template string

	T map[string]string // filled by Write from Lang
}

// Templates lists the shapes trilha new can write, in the order they are
// offered.
func Templates() []string { return []string{"blog", "app"} }

// Write creates the project at dir. Existing files are never overwritten.
func Write(dir string, d Data) ([]string, error) {
	if d.Lang == "" {
		d.Lang = "en"
	}
	d.T = texts[d.Lang]
	if d.T == nil {
		return nil, errors.New("scaffold: unknown language " + d.Lang)
	}
	if d.Template == "" {
		d.Template = "blog"
	}
	if !slices.Contains(Templates(), d.Template) {
		return nil, errors.New("scaffold: unknown template " + d.Template + " (" + strings.Join(Templates(), ", ") + ")")
	}
	// base is what every project has; the rest is the shape that was asked
	// for. A file of the shape wins over one of the base with the same name.
	var written []string
	roots := []string{"templates/base", "templates/" + d.Template}
	err := fs.WalkDir(templates, ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		root := ""
		for _, r := range roots {
			if strings.HasPrefix(p, r+"/") {
				root = r
			}
		}
		if root == "" {
			return nil
		}
		rel := strings.TrimPrefix(p, root+"/")
		rel = strings.TrimSuffix(rel, ".tmpl")
		if rel == "gitignore" {
			rel = ".gitignore"
		}
		src, err := templates.ReadFile(p)
		if err != nil {
			return err
		}
		t, err := template.New(rel).Delims("{{", "}}").Parse(string(src))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, d); err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(dst); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	})
	if err != nil {
		return written, err
	}
	res, err := WriteUI(dir, false, false, false)
	for _, r := range res {
		if r.Action == UICreated {
			written = append(written, "public/"+r.File)
		}
	}
	if errors.Is(err, ErrUIModified) {
		err = nil
	}
	return written, err
}
