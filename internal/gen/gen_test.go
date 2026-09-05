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
	for _, app := range []string{"full", "minimal", "groups"} {
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
