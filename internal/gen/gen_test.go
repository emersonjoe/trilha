package gen

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/emersonjoe/trilha/internal/scan"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGolden(t *testing.T) {
	for _, app := range []string{"full", "minimal", "groups", "embedded"} {
		t.Run(app, func(t *testing.T) {
			res, err := scan.Scan(filepath.Join("..", "..", "testdata", "apps", app), "example.com/"+app)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Generate(res)
			if err != nil {
				t.Fatal(err)
			}
			again, _ := Generate(res)
			if !bytes.Equal(got, again) {
				t.Fatal("generator is not deterministic")
			}
			golden := filepath.Join("..", "..", "testdata", "golden", app+".go.golden")
			if *update {
				_ = os.MkdirAll(filepath.Dir(golden), 0o755)
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden (run with -update): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch for %s:\n%s", app, got)
			}
		})
	}
}

// TestEmbeddedPackage locks the shape an app inside an existing binary needs:
// the package the directory already declares, an exported constructor for the
// host to call, and no func main() to collide with the host's own.
func TestEmbeddedPackage(t *testing.T) {
	res, err := scan.Scan(filepath.Join("..", "..", "testdata", "apps", "embedded"), "example.com/host/internal/crm")
	if err != nil {
		t.Fatal(err)
	}
	if res.Package != "crm" {
		t.Fatalf("package = %q, want crm (taken from the hand-written file)", res.Package)
	}
	got, err := Generate(res)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("package crm\n")) {
		t.Fatalf("generated the wrong package clause:\n%s", got)
	}
	if !bytes.Contains(got, []byte("func NewApp() *trilha.App")) {
		t.Fatalf("constructor is not exported, so the host cannot call it:\n%s", got)
	}
	if bytes.Contains(got, []byte("func main()")) {
		t.Fatalf("generated func main() outside package main:\n%s", got)
	}
}

// TestGeneratedFileRemembersPackage: the choice survives in the generated file,
// so `trilha gen --check` in the CI agrees without repeating --package.
func TestGeneratedFileRemembersPackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app", "page.go"), []byte("package app\n\nfunc Page() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := scan.Scan(dir, "example.com/x")
	if err != nil {
		t.Fatal(err)
	}
	if res.Package != "main" {
		t.Fatalf("package = %q, want main for a directory with nothing else in it", res.Package)
	}
	res.Package = "crm" // what --package does on the first run
	src, err := Generate(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), src, 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := scan.Scan(dir, "example.com/x")
	if err != nil {
		t.Fatal(err)
	}
	if again.Package != "crm" {
		t.Fatalf("second scan lost the package: %q", again.Package)
	}
}

func TestCustomMainAndKind(t *testing.T) {
	res, err := scan.Scan(filepath.Join("..", "..", "testdata", "apps", "custom_main"), "example.com/custom_main")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Generate(res)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("func main()")) {
		t.Fatalf("generated main with a hand-written main present:\n%s", got)
	}
	if !bytes.Contains(got, []byte("a.OnShutdown(app.Shutdown)")) {
		t.Fatalf("Shutdown hook missing:\n%s", got)
	}
	// #15: Config returning an error is checked, not discarded.
	if !bytes.Contains(got, []byte("if err := app.Config(&cfg); err != nil {")) {
		t.Fatalf("Config error not checked:\n%s", got)
	}
	// #18: go generate ./... works without knowing the tool's name.
	if !bytes.Contains(got, []byte("//go:generate trilha gen")) {
		t.Fatalf("go:generate directive missing:\n%s", got)
	}
	res, err = scan.Scan(filepath.Join("..", "..", "testdata", "apps", "dotdir"), "example.com/dotdir")
	if err != nil {
		t.Fatal(err)
	}
	got, _ = Generate(res)
	for _, want := range []string{`app_app_css "example.com/dotdir/app/app.css"`, `Pattern: "/app.css"`, `Kind:    app_app_css.Kind,`, `Pattern: "/manifest.webmanifest"`, "func main()"} {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
