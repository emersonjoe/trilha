// Package scaffold writes a new project from embedded templates.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

	T map[string]string // filled by Write from Lang
}

// Write creates the project at dir. Existing files are never overwritten.
func Write(dir string, d Data) ([]string, error) {
	if d.Lang == "" {
		d.Lang = "en"
	}
	d.T = texts[d.Lang]
	if d.T == nil {
		return nil, errors.New("scaffold: unknown language " + d.Lang)
	}
	var written []string
	err := fs.WalkDir(templates, "templates", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, "templates/")
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
