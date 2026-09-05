package trilha

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoExternalDeps guards constitution principle II: runtime and CLI depend
// only on the standard library.
func TestNoExternalDeps(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	out, err := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".", "./h", "./cmd/trilha").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" && !strings.HasPrefix(line, "github.com/emersonjoe/trilha") {
			t.Errorf("external dependency: %s", line)
		}
	}
}
