package migrate

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden tree and the report")

const (
	nextDir   = "../../testdata/next"
	goldenDir = "../../testdata/next.golden"
)

// TestGoldenTree holds the two things the command promises: the same tree
// produces the same bytes, and the bytes are the ones committed.
func TestGoldenTree(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	files := p.Files()
	files = append(files, File{Path: "MIGRATION.md", Body: Report(p, "en")})
	files = append(files, File{Path: "MIGRATION.pt.md", Body: Report(p, "pt")})
	if *update {
		if err := os.RemoveAll(goldenDir); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			dst := filepath.Join(goldenDir, filepath.FromSlash(f.Path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, []byte(f.Body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[filepath.FromSlash(f.Path)] = true
		want, err := os.ReadFile(filepath.Join(goldenDir, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("%s: not in the golden tree", f.Path)
			continue
		}
		if string(want) != f.Body {
			t.Errorf("%s differs from the golden (run make golden):\n%s", f.Path, f.Body)
		}
	}
	err = filepath.WalkDir(goldenDir, func(fp string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if rel, _ := filepath.Rel(goldenDir, fp); !seen[rel] {
			t.Errorf("%s is in the golden tree and was not generated", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestScanIsDeterministic runs the scan twice: a map iteration leaking into
// the output would show up here before it showed up in a diff.
func TestScanIsDeterministic(t *testing.T) {
	first, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	if Report(first, "en") != Report(second, "en") {
		t.Error("two scans of the same tree wrote two reports")
	}
	a, b := first.Files(), second.Files()
	if len(a) != len(b) {
		t.Fatalf("%d files then %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("%s is not the same twice", a[i].Path)
		}
	}
}

// TestFolders checks the translation of every kind of segment the App Router
// has, which is the part a person would otherwise do by hand 47 times.
func TestFolders(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"page.go":                  "/",
		"marketing-/about/page.go": "/about",
		"documents/page.go":        "/documents",
		"documents/id_/page.go":    "/documents/{id}",
		"users/user_id_/page.go":   "/users/{user_id}",
		"files/path__/page.go":     "/files/{path...}",
		"docs/slug__/page.go":      "/docs/{slug...}",
		"dashboard/page.go":        "/dashboard",
		"dashboard/layout.go":      "/dashboard",
		"api/documents/route.go":   "/api/documents",
		"api/health/route.go":      "/api/health",
		"not_found.go":             "/",
		"error.go":                 "/",
	}
	got := map[string]string{}
	for _, pg := range p.Pages {
		key := pg.File
		if pg.Dir != "" {
			key = pg.Dir + "/" + pg.File
		}
		got[key] = pg.URL
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s was generated and is not expected", k)
		}
	}
}

// TestNotes: what has no equivalent is said, not written.
func TestNotes(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		NoteParallel: true, NoteIntercept: true, NoteLoading: true, NoteTemplate: true,
		NoteMiddleware: true, NoteRewrite: true, NoteRootLayout: true, NoteRenamed: true,
		NoteOptionalAll: true,
	}
	for _, n := range p.Notes {
		delete(want, n.Code)
	}
	for code := range want {
		t.Errorf("no note of kind %q", code)
	}
	for _, f := range p.Files() {
		if strings.Contains(f.Path, "sidebar") || strings.Contains(f.Path, "modal") {
			t.Errorf("%s: a route with no equivalent must not be written", f.Path)
		}
	}
}

// TestClasses checks the suggestion on the three screens written to earn one.
func TestClasses(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"dashboard/page.go":     ClassSPA,    // <svg> and onPointerDown
		"documents/id_/page.go": ClassIsland, // setInterval
		"documents/page.go":     ClassForm,   // a list and a filter
	}
	for _, pg := range p.Pages {
		key := pg.File
		if pg.Dir != "" {
			key = pg.Dir + "/" + pg.File
		}
		if w, ok := want[key]; ok && pg.Class != w {
			t.Errorf("%s = %s (%s), want %s", key, pg.Class, pg.Why, w)
		}
	}
}

// TestEndpoints: the addresses come out of the text with the method they are
// called with, template literal and all.
func TestEndpoints(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	var doc Page
	for _, pg := range p.Pages {
		if pg.Dir == "documents/id_" {
			doc = pg
		}
	}
	got := strings.Join(endpointStrings(doc.Endpoints), " | ")
	want := "GET /api/documents/:id | POST /api/documents/:id/reprocess | GET /api/documents/:id/status"
	if got != want {
		t.Errorf("endpoints = %q, want %q", got, want)
	}
}

// TestMethods: three exports, three handlers, and an arrow function counts.
func TestMethods(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, pg := range p.Pages {
		switch pg.Dir {
		case "api/documents":
			if strings.Join(pg.Methods, ",") != "GET,POST,DELETE" {
				t.Errorf("methods = %v", pg.Methods)
			}
		case "api/health":
			if strings.Join(pg.Methods, ",") != "GET" {
				t.Errorf("health = %v", pg.Methods)
			}
		}
	}
}

// TestWriteKeepsWhatIsThere: the second run of the command must not cost the
// screens ported between the two.
func TestWriteKeepsWhatIsThere(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Write(dir, p, false); err != nil {
		t.Fatal(err)
	}
	ported := filepath.Join(dir, "documents", "page.go")
	if err := os.WriteFile(ported, []byte("package documents // ported by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Write(dir, p, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Action != Kept {
			t.Errorf("%s = %s, want kept", r.Path, r.Action)
		}
	}
	if b, _ := os.ReadFile(ported); !strings.Contains(string(b), "by hand") {
		t.Fatal("the ported file was overwritten")
	}
	if res, err = Write(dir, p, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(ported); strings.Contains(string(b), "by hand") {
		t.Fatal("--force must overwrite")
	}
}
