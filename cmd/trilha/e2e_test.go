package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestE2E runs `trilha new`, `trilha build` and boots the binary.
func TestE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	cli := filepath.Join(tmp, "trilha-cli")
	run(t, repo, "go", "build", "-o", cli, "./cmd/trilha")

	proj := filepath.Join(tmp, "meu-app")
	out := run(t, tmp, cli, "new", proj, "--module", "example.com/meu-app", "--trilha-dir", repo)
	if !strings.Contains(out, "projeto criado") {
		t.Fatal(out)
	}
	for _, f := range []string{"go.mod", "trilha_gen.go", "app/page.go", "app/layout.go", "app/api/hello/route.go", "public/style.css", "public/ui.css", "public/ui.theme.css", "public/ui.js", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	out = run(t, proj, cli, "routes")
	if !strings.Contains(out, "/api/hello") || !strings.Contains(out, "app/page.go") {
		t.Fatal(out)
	}
	// trilha ui: no-op on a fresh project; keeps the theme; refuses edited kit files without --force.
	if out := run(t, proj, cli, "ui"); !regexp.MustCompile(`ui\.css\s+mantido`).MatchString(out) {
		t.Fatal(out)
	}
	theme := filepath.Join(proj, "public", "ui.theme.css")
	css := filepath.Join(proj, "public", "ui.css")
	os.WriteFile(theme, []byte(":root{--primary:red}"), 0o644)
	os.WriteFile(css, []byte("/* edited */"), 0o644)
	uiCmd := exec.Command(cli, "ui")
	uiCmd.Dir = proj
	if out, err := uiCmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "modificado") {
		t.Fatal(string(out), err)
	} else if b, _ := os.ReadFile(css); string(b) != "/* edited */" {
		t.Fatal("must not overwrite without --force")
	}
	if out := run(t, proj, cli, "ui", "--force"); !regexp.MustCompile(`ui\.css\s+atualizado`).MatchString(out) || !strings.Contains(out, "seu tema") {
		t.Fatal(out)
	}
	if b, _ := os.ReadFile(theme); string(b) != ":root{--primary:red}" {
		t.Fatal("theme must be preserved")
	}
	run(t, proj, cli, "build", "-o", "bin/app")
	os.Setenv("TRILHA_SECRET", strings.Repeat("s", 32))
	if out := run(t, proj, cli, "audit", "--no-vuln"); !strings.Contains(out, "✓ TRILHA_SECRET definido") || !strings.Contains(out, "✓ trilha_gen.go atualizado") {
		t.Fatal(out)
	}
	os.Unsetenv("TRILHA_SECRET")
	run(t, proj, cli, "export", "-o", "out")
	for _, f := range []string{"out/index.html", "out/404.html", "out/style.css", "out/.trilha-export"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("export missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(proj, "out", "api")); err == nil {
		t.Fatal("api must not be exported")
	}

	// Boot the binary from a different directory: public/ must be embedded.
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bin := exec.CommandContext(ctx, filepath.Join(proj, "bin", "app"))
	bin.Dir = tmp
	bin.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "TRILHA_ENV=prod")
	bin.Stdout, bin.Stderr = io.Discard, io.Discard
	if err := bin.Start(); err != nil {
		t.Fatal(err)
	}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	body := waitGet(t, base+"/")
	if !strings.Contains(body, "Olá, meu-app!") || strings.Contains(body, "_trilha/events") {
		t.Fatal(body)
	}
	if !strings.Contains(body, `href="/ui.css"`) || !strings.Contains(body, "ui-card") {
		t.Fatal("ui kit missing in page:", body)
	}
	if b := waitGet(t, base+"/ui.css"); !strings.Contains(b, ".ui-btn") {
		t.Fatal("public not embedded:", b)
	}
	if b := waitGet(t, base+"/api/hello"); !strings.Contains(b, `"hello":"meu-app"`) {
		t.Fatal(b)
	}
	if b := waitGet(t, base+"/nada"); !strings.Contains(b, "404") {
		t.Fatal(b)
	}
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitGet(t *testing.T, url string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return string(b)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", url)
	return ""
}
