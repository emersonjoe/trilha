package main

import (
	"context"
	"encoding/json"
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
	t.Setenv("TRILHA_LANG", "en") // messages asserted below are English; pt is checked at the end
	cli := filepath.Join(tmp, "trilha-cli")
	run(t, repo, "go", "build", "-o", cli, "./cmd/trilha")

	proj := filepath.Join(tmp, "meu-app")
	out := run(t, tmp, cli, "new", proj, "--module", "example.com/meu-app", "--trilha-dir", repo)
	if !strings.Contains(out, "project created") {
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
	if out := run(t, proj, cli, "ui"); !regexp.MustCompile(`ui\.css\s+kept`).MatchString(out) {
		t.Fatal(out)
	}
	theme := filepath.Join(proj, "public", "ui.theme.css")
	css := filepath.Join(proj, "public", "ui.css")
	os.WriteFile(theme, []byte(":root{--primary:red}"), 0o644)
	os.WriteFile(css, []byte("/* edited */"), 0o644)
	uiCmd := exec.Command(cli, "ui")
	uiCmd.Dir = proj
	if out, err := uiCmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "modified locally") {
		t.Fatal(string(out), err)
	} else if b, _ := os.ReadFile(css); string(b) != "/* edited */" {
		t.Fatal("must not overwrite without --force")
	}
	if out := run(t, proj, cli, "ui", "--force"); !regexp.MustCompile(`ui\.css\s+updated`).MatchString(out) || !strings.Contains(out, "your theme") {
		t.Fatal(out)
	}
	if b, _ := os.ReadFile(theme); string(b) != ":root{--primary:red}" {
		t.Fatal("theme must be preserved")
	}
	run(t, proj, cli, "build", "-o", "bin/app")
	os.Setenv("TRILHA_SECRET", strings.Repeat("s", 32))
	if out := run(t, proj, cli, "audit", "--no-vuln"); !strings.Contains(out, "✓ TRILHA_SECRET set") || !strings.Contains(out, "✓ trilha_gen.go up to date") {
		t.Fatal(out)
	}
	os.Unsetenv("TRILHA_SECRET")
	// #18: gen --check is the one line a CI needs to catch a stale trilha_gen.go.
	if out := run(t, proj, cli, "gen", "--check"); !strings.Contains(out, "up to date") {
		t.Fatal(out)
	}
	os.MkdirAll(filepath.Join(proj, "app", "nova"), 0o755)
	page := "package nova\n\nimport (\n\t\"github.com/emersonjoe/trilha\"\n\t\"github.com/emersonjoe/trilha/h\"\n)\n\nfunc Page(c *trilha.Ctx) (h.Node, error) { return h.P(h.Text(\"nova\")), nil }\n"
	os.WriteFile(filepath.Join(proj, "app", "nova", "page.go"), []byte(page), 0o644)
	checkCmd := exec.Command(cli, "gen", "--check")
	checkCmd.Dir = proj
	if out, err := checkCmd.CombinedOutput(); err == nil {
		t.Fatalf("a route missing from trilha_gen.go must fail: %s", out)
	} else if !strings.Contains(string(out), "out of date") || !strings.Contains(string(out), "/nova") {
		t.Fatalf("the diff must show what is missing: %s", out)
	}
	os.RemoveAll(filepath.Join(proj, "app", "nova"))
	run(t, proj, cli, "gen", "--check")
	run(t, proj, cli, "export", "-o", "out")
	for _, f := range []string{"out/index.html", "out/404.html", "out/style.css", "out/.trilha-export"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("export missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(proj, "out", "api")); err == nil {
		t.Fatal("api must not be exported")
	}

	// #31: the document comes out of the routes, and --check is the line that
	// keeps it from drifting from them.
	doc := run(t, proj, cli, "openapi", "-o", "-")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(doc), &parsed); err != nil {
		t.Fatalf("openapi -o - must write JSON: %v\n%s", err, doc)
	}
	if parsed["openapi"] != "3.1.0" {
		t.Fatal(doc)
	}
	if _, ok := parsed["paths"].(map[string]any)["/api/hello"]; !ok {
		t.Fatal("the scaffolded API route is missing:", doc)
	}
	if out := run(t, proj, cli, "openapi"); !strings.Contains(out, "openapi.json") {
		t.Fatal(out)
	}
	if out := run(t, proj, cli, "openapi", "--check"); !strings.Contains(out, "up to date") {
		t.Fatal(out)
	}
	os.MkdirAll(filepath.Join(proj, "app", "api", "novo"), 0o755)
	route := "package novo\n\nimport (\n\t\"net/http\"\n\n\t\"github.com/emersonjoe/trilha\"\n)\n\n// GET answers nothing useful.\nfunc GET(c *trilha.Ctx) error { return c.JSON(http.StatusOK, map[string]string{}) }\n"
	os.WriteFile(filepath.Join(proj, "app", "api", "novo", "route.go"), []byte(route), 0o644)
	staleCmd := exec.Command(cli, "openapi", "--check")
	staleCmd.Dir = proj
	if out, err := staleCmd.CombinedOutput(); err == nil {
		t.Fatalf("a route missing from openapi.json must fail: %s", out)
	} else if !strings.Contains(string(out), "out of date") {
		t.Fatalf("%s", out)
	}
	os.RemoveAll(filepath.Join(proj, "app", "api", "novo"))
	os.Remove(filepath.Join(proj, "openapi.json"))

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
	if !strings.Contains(body, "Hello, meu-app!") || !strings.Contains(body, `<html lang="en">`) || strings.Contains(body, "_trilha/events") {
		t.Fatal(body)
	}
	if !strings.Contains(body, `href="/ui.css?v=`) || !strings.Contains(body, "ui-card") {
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

	// Portuguese: CLI messages follow TRILHA_LANG; --lang picks the scaffold texts.
	t.Setenv("TRILHA_LANG", "pt_BR")
	if out := run(t, proj, cli, "routes"); !strings.Contains(out, "MÉTODOS") {
		t.Fatal(out)
	}
	projPT := filepath.Join(tmp, "app-pt")
	out = run(t, tmp, cli, "new", projPT, "--module", "example.com/app-pt", "--trilha-dir", repo, "--no-tidy")
	if !strings.Contains(out, "projeto criado") {
		t.Fatal(out)
	}
	if b, _ := os.ReadFile(filepath.Join(projPT, "app", "page.go")); !strings.Contains(string(b), "Olá, app-pt!") {
		t.Fatal("--lang default must follow the CLI language:", string(b))
	}
	if b, _ := os.ReadFile(filepath.Join(projPT, "app", "layout.go")); !strings.Contains(string(b), `h.Lang("pt-BR")`) {
		t.Fatal(string(b))
	}
	// Explicit --lang en wins over TRILHA_LANG.
	projEN := filepath.Join(tmp, "app-en")
	run(t, tmp, cli, "new", projEN, "--module", "example.com/app-en", "--trilha-dir", repo, "--no-tidy", "--lang", "en")
	if b, _ := os.ReadFile(filepath.Join(projEN, "app", "page.go")); !strings.Contains(string(b), "Hello, app-en!") {
		t.Fatal(string(b))
	}
	bad := exec.Command(cli, "new", filepath.Join(tmp, "x"), "--lang", "fr", "--no-tidy")
	bad.Dir = tmp
	if out, err := bad.CombinedOutput(); err == nil || !strings.Contains(string(out), "--lang deve ser en ou pt") {
		t.Fatal(string(out), err)
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
