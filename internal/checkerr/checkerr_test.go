package checkerr

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestErrorCatalogComplete is the alarm that keeps the catalog the source:
// every stable code the source code mentions has to have an entry, so a new
// code cannot start life unexplained. Test files are skipped — they are
// allowed to invent codes to test the catalog itself.
func TestErrorCatalogComplete(t *testing.T) {
	var codeRe = regexp.MustCompile(`\bE_[A-Z][A-Z_]*\b`)
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "tmp", "specs":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range codeRe.FindAllString(string(b), -1) {
			if m == "E_" {
				continue
			}
			if _, ok := ByCode(m); !ok {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s: %s is not in the catalog", filepath.ToSlash(rel), m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestByCode reads the two lookup rules: case-insensitive exact, and family
// by prefix for the codes that carry a dynamic tail.
func TestByCode(t *testing.T) {
	d, ok := ByCode("e_duplicate_route")
	if !ok || d.Code != "E_DUPLICATE_ROUTE" || d.Fix == "" {
		t.Fatalf("ByCode exact: %+v %v", d, ok)
	}
	d, ok = ByCode("E_VULN_GO-2026-0001")
	if !ok || !strings.HasPrefix(d.Code, "E_VULN") || !strings.Contains(d.Fix, "go get") {
		t.Fatalf("ByCode family: %+v %v", d, ok)
	}
	if _, ok := ByCode("E_NAO_EXISTE"); ok {
		t.Fatal("an unknown code must not resolve")
	}
	// Every entry answers a reader: title, cause, fix and a page to open.
	for _, d := range Docs() {
		if d.Title == "" || d.Cause == "" || d.Fix == "" || d.Doc != "/docs/errors/"+d.Code {
			t.Errorf("%+v: incomplete entry", d.Code)
		}
	}
}
