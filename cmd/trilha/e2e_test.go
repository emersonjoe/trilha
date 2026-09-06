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
	// trilha agents: opt-in, so `trilha new` above left nothing behind.
	agents := filepath.Join(proj, "AGENTS.md")
	if _, err := os.Stat(agents); err == nil {
		t.Fatal("trilha new must not write AGENTS.md by default")
	}
	if out := run(t, proj, cli, "agents"); !strings.Contains(out, "AGENTS.md") || !strings.Contains(out, "CLAUDE.md") {
		t.Fatal(out)
	}
	if b, _ := os.ReadFile(agents); !strings.Contains(string(b), "trilha gen") {
		t.Fatalf("AGENTS.md = %s", b)
	}
	if out := run(t, proj, cli, "agents"); !regexp.MustCompile(`AGENTS\.md\s+kept`).MatchString(out) {
		t.Fatal(out)
	}
	b, _ := os.ReadFile(agents)
	os.WriteFile(agents, append(b, []byte("\n## our own rules\n")...), 0o644)
	agCmd := exec.Command(cli, "agents")
	agCmd.Dir = proj
	if out, err := agCmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "modified locally") {
		t.Fatal(string(out), err)
	} else if b, _ := os.ReadFile(agents); !strings.Contains(string(b), "our own rules") {
		t.Fatal("must not overwrite AGENTS.md without --force")
	}
	if out := run(t, proj, cli, "agents", "--force"); !regexp.MustCompile(`AGENTS\.md\s+updated`).MatchString(out) {
		t.Fatal(out)
	}
	// A project scaffolded with --agents already has both files.
	withAI := filepath.Join(tmp, "com-agentes")
	run(t, tmp, cli, "new", withAI, "--trilha-dir", repo, "--no-tidy", "--agents")
	for _, f := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(withAI, f)); err != nil {
			t.Fatalf("new --agents missing %s", f)
		}
	}
	os.Remove(agents)
	os.Remove(filepath.Join(proj, "CLAUDE.md"))
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

	// #48: check is the single gate — the steps in order, in one command —
	// and #47: ctx is the map of the project, in one read.
	os.Setenv("TRILHA_SECRET", strings.Repeat("s", 32))
	if out := run(t, proj, cli, "check"); !strings.Contains(out, "✓ gen") || !strings.Contains(out, "✓ test") || !strings.Contains(out, "– openapi") {
		t.Fatal(out)
	}
	os.MkdirAll(filepath.Join(proj, "app", "nova"), 0o755)
	os.WriteFile(filepath.Join(proj, "app", "nova", "page.go"), []byte(page), 0o644)
	staleCheck := exec.Command(cli, "check", "--json")
	staleCheck.Dir = proj
	raw, err := staleCheck.Output() // the report goes to stdout; the failure line, to stderr
	if err == nil {
		t.Fatalf("a route added and not generated must fail check: %s", raw)
	}
	var report struct {
		OK    bool `json:"ok"`
		Steps []struct {
			Tool, Status string
		} `json:"steps"`
		Problems []struct {
			Tool, File, Message, Fix string
		} `json:"problems"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("check --json must write JSON: %v\n%s", err, raw)
	}
	if report.OK || report.Steps[0].Status != "failed" || len(report.Problems) == 0 {
		t.Fatalf("%s", raw)
	}
	if !strings.Contains(report.Problems[0].Fix, "trilha gen") {
		t.Fatalf("the problem must carry its conserto: %s", raw)
	}
	for _, s := range report.Steps[1:] {
		if s.Status != "not run" {
			t.Fatalf("%s ran after gen failed: %s", s.Tool, raw)
		}
	}
	if out := run(t, proj, cli, "check", "--fix"); !strings.Contains(out, "gen (fixed)") {
		t.Fatal(out)
	}
	ctxOut := run(t, proj, cli, "ctx")
	for _, want := range []string{"## Routes", "/api/hello", "/nova"} {
		if !strings.Contains(ctxOut, want) {
			t.Fatalf("ctx without %q:\n%s", want, ctxOut)
		}
	}
	var mapa struct {
		Module string `json:"module"`
		Routes []struct {
			Pattern string `json:"pattern"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(run(t, proj, cli, "ctx", "--json")), &mapa); err != nil {
		t.Fatalf("ctx --json must write JSON: %v", err)
	}
	if mapa.Module != "example.com/meu-app" || len(mapa.Routes) < 2 {
		t.Fatalf("%+v", mapa)
	}
	views := exec.Command(cli, "ctx", "--routes", "--types")
	views.Dir = proj
	if out, err := views.CombinedOutput(); err == nil || !strings.Contains(string(out), "choose one") {
		t.Fatalf("two views at once must be refused: %s", out)
	}
	os.Unsetenv("TRILHA_SECRET")
	os.RemoveAll(filepath.Join(proj, "app", "nova"))
	run(t, proj, cli, "gen")

	// #75: /.well-known/ is the one dot folder that becomes a route — and it has
	// to compile, since the Go tool does not match its import path in ./... .
	wk := filepath.Join(proj, "app", ".well-known", "security.txt")
	os.MkdirAll(wk, 0o755)
	os.WriteFile(filepath.Join(wk, "route.go"),
		[]byte("package security\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc GET(c *trilha.Ctx) error { return c.Text(200, \"Contact: mailto:x@example.com\\n\") }\n"), 0o644)
	run(t, proj, cli, "gen")
	if out := run(t, proj, cli, "routes"); !strings.Contains(out, "/.well-known/security.txt") {
		t.Fatalf("the .well-known route must be listed: %s", out)
	}
	run(t, proj, cli, "build", "-o", "bin/app")
	os.RemoveAll(filepath.Join(proj, "app", ".well-known"))

	// Every other dot folder still disappears, but it says so instead of
	// answering 404 later.
	os.MkdirAll(filepath.Join(proj, "app", ".oauth"), 0o755)
	os.WriteFile(filepath.Join(proj, "app", ".oauth", "route.go"),
		[]byte("package oauth\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc GET(c *trilha.Ctx) error { return c.Text(200, \"x\") }\n"), 0o644)
	hidden := exec.Command(cli, "gen")
	hidden.Dir = proj
	if out, err := hidden.CombinedOutput(); err == nil {
		t.Fatalf("a route inside a dot folder must not pass in silence: %s", out)
	} else if !strings.Contains(string(out), "app/.oauth/route.go") || !strings.Contains(string(out), ".well-known") {
		t.Fatalf("the error must name the file and the exception: %s", out)
	}
	os.RemoveAll(filepath.Join(proj, "app", ".oauth"))
	run(t, proj, cli, "gen")

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

	// #36: generate writes the folder the convention asks for, refuses to
	// overwrite and refuses page + route in the same folder.
	if out := run(t, proj, cli, "generate", "page", "/blog/{slug}"); !strings.Contains(out, "app/blog/slug_/page.go") || !strings.Contains(out, "/blog/{slug}") {
		t.Fatal(out)
	}
	if b, _ := os.ReadFile(filepath.Join(proj, "trilha_gen.go")); !strings.Contains(string(b), "/blog/{slug}") {
		t.Fatal("generate must leave trilha_gen.go up to date")
	}
	again := exec.Command(cli, "generate", "page", "/blog/{slug}")
	again.Dir = proj
	if out, err := again.CombinedOutput(); err == nil || !strings.Contains(string(out), "--force") {
		t.Fatalf("generating twice must refuse and point at --force: %s", out)
	}
	conflict := exec.Command(cli, "generate", "route", "/blog/{slug}")
	conflict.Dir = proj
	if out, err := conflict.CombinedOutput(); err == nil || !strings.Contains(string(out), "never both") {
		t.Fatalf("page + route in the same folder must be refused: %s", out)
	}
	run(t, proj, cli, "generate", "route", "/api/itens/{id}")
	run(t, proj, cli, "generate", "component", "Aviso")
	for _, f := range []string{"app/api/itens/id_/route.go", "internal/components/aviso.go"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("generate missing %s", f)
		}
	}
	unknown := exec.Command(cli, "generate", "layout", "/x")
	unknown.Dir = proj
	if out, err := unknown.CombinedOutput(); err == nil || !strings.Contains(string(out), "unknown kind") {
		t.Fatalf("only page, route and component: %s", out)
	}
	// The whole point of a skeleton is that it compiles before you touch it.
	run(t, proj, "go", "build", "./...")
	os.RemoveAll(filepath.Join(proj, "app", "blog"))
	os.RemoveAll(filepath.Join(proj, "app", "api", "itens"))
	os.RemoveAll(filepath.Join(proj, "internal"))
	run(t, proj, cli, "gen")

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

// TestEmbeddedAppE2E is issue #51 end to end: an app that lives inside a binary
// that already exists. What it proves is that the generated file needs no
// hand-written copy — `gen --check`, the thing that catches a route added and
// not generated, keeps working for a project shaped like this.
func TestEmbeddedAppE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	cli := filepath.Join(tmp, "trilha-cli")
	run(t, repo, "go", "build", "-o", cli, "./cmd/trilha")

	host := filepath.Join(tmp, "farol")
	crm := filepath.Join(host, "internal", "crm")
	mustWrite(t, filepath.Join(host, "go.mod"), "module example.com/farol\n\ngo 1.22\n\nrequire github.com/emersonjoe/trilha v0.0.0\n\nreplace github.com/emersonjoe/trilha => "+repo+"\n")
	mustWrite(t, filepath.Join(host, "main.go"), `package main

import (
	"net/http"
	"os"

	"example.com/farol/internal/crm"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/antigo", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("o roteador antigo")) })
	mux.Handle("/", crm.NewApp().Handler())
	http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), mux)
}
`)
	mustWrite(t, filepath.Join(crm, "crm.go"), "package crm\n\n// Nome is the module the host already had.\nconst Nome = \"crm\"\n")
	mustWrite(t, filepath.Join(crm, "app", "contatos", "page.go"), `package contatos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return h.P(h.Text("contatos do crm")), nil }
`)

	// gen adopts the package the directory declares: no flag, nothing to remember.
	out := run(t, crm, cli, "gen")
	if !strings.Contains(out, "trilha_gen.go") {
		t.Fatal(out)
	}
	src, err := os.ReadFile(filepath.Join(crm, "trilha_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "package crm\n") || !strings.Contains(string(src), "func NewApp() *trilha.App") {
		t.Fatalf("generated file is not importable by the host:\n%s", src)
	}
	if strings.Contains(string(src), "func main()") {
		t.Fatalf("generated func main() inside a package the host imports:\n%s", src)
	}
	if out := run(t, crm, cli, "gen", "--check"); !strings.Contains(out, "up to date") {
		t.Fatal(out)
	}

	// dev and build say what runs this app instead of failing at `go run`.
	for _, cmd := range []string{"dev", "build"} {
		c := exec.Command(cli, cmd)
		c.Dir = crm
		out, err := c.CombinedOutput()
		if err == nil {
			t.Fatalf("trilha %s accepted an embedded app:\n%s", cmd, out)
		}
		if !strings.Contains(string(out), "package crm") || !strings.Contains(string(out), "NewApp().Handler()") {
			t.Fatalf("trilha %s does not say what runs the app:\n%s", cmd, out)
		}
	}

	// The host binary compiles with the app inside it and serves both routers.
	bin := filepath.Join(tmp, "farol-bin")
	run(t, host, "go", "build", "-o", bin, ".")
	port := freePort(t)
	srv := exec.Command(bin)
	srv.Dir = host
	srv.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "TRILHA_ENV=dev")
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Process.Kill() }()
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	if body := waitGet(t, base+"/antigo"); !strings.Contains(body, "o roteador antigo") {
		t.Fatalf("host route: %s", body)
	}
	if body := waitGet(t, base+"/contatos"); !strings.Contains(body, "contatos do crm") {
		t.Fatalf("embedded route: %s", body)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGenerateContratoE2E is issue #49 end to end: what the flags write has to
// compile and pass trilha check with nobody editing it. It is the only place
// that proves the skeleton and the test it generates agree on the status.
func TestGenerateContratoE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	// The audit step of check reads the environment, and a missing secret is
	// about this machine, not about what generate wrote.
	t.Setenv("TRILHA_SECRET", "um-segredo-de-teste-com-mais-de-32-bytes")
	cli := filepath.Join(tmp, "trilha-cli")
	run(t, repo, "go", "build", "-o", cli, "./cmd/trilha")

	proj := filepath.Join(tmp, "loja")
	run(t, tmp, cli, "new", proj, "--module", "example.com/loja", "--trilha-dir", repo)

	out := run(t, proj, cli, "generate", "route", "/api/itens/{id}", "--methods", "GET,POST", "--bind", "Item")
	if !strings.Contains(out, "app/api/itens/id_/route.go") || !strings.Contains(out, "/api/itens/{id}") {
		t.Fatal(out)
	}
	run(t, proj, cli, "generate", "page", "/painel/contato", "--form", "Contact", "--layout", "app/painel/layout.go")
	if _, err := os.Stat(filepath.Join(proj, "app", "painel", "layout.go")); err != nil {
		t.Fatal("--layout must write the layout that is missing:", err)
	}
	run(t, proj, cli, "generate", "test", "/api/itens/{id}")
	run(t, proj, cli, "generate", "test", "/painel/contato")

	// The whole point: no edit between generating and the gate being green.
	if out := run(t, proj, cli, "check"); !strings.Contains(out, "test") {
		t.Fatal(out)
	}

	// A type the project already declares is imported, not declared twice.
	run(t, proj, cli, "generate", "route", "/api/itens", "--methods", "POST", "--bind", "Item")
	b, err := os.ReadFile(filepath.Join(proj, "app", "api", "itens", "route.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"example.com/loja/app/api/itens/id_"`) || strings.Contains(string(b), "type Item struct") {
		t.Fatal("the type that already exists is imported, not declared again:\n" + string(b))
	}

	// A method route.go could not export is a refusal, and it writes nothing.
	bad := exec.Command(cli, "generate", "route", "/api/x", "--methods", "TRACE")
	bad.Dir = proj
	if out, err := bad.CombinedOutput(); err == nil || !strings.Contains(string(out), "TRACE") {
		t.Fatal(string(out), err)
	}
	if _, err := os.Stat(filepath.Join(proj, "app", "api", "x")); err == nil {
		t.Fatal("a refusal must not leave a folder behind")
	}
}
