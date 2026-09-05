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
	for _, app := range []string{"full", "minimal"} {
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
