package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/emersonjoe/trilha/ai/mcp"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
	cli := buildCLI(t, repo, tmp)

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
	// trilha ui describe: the catalogue ships with the CLI, so it answers from
	// tmp — outside any project — and the name is read as it is typed.
	out = run(t, tmp, cli, "ui", "describe")
	if !strings.Contains(out, "forms") || !strings.Contains(out, "Field") || !strings.Contains(out, "components.") {
		t.Fatal(out)
	}
	for _, name := range []string{"Field", "field", "ui.Field"} {
		out = run(t, tmp, cli, "ui", "describe", name)
		if !strings.Contains(out, "ui.Field(id, label string") || !strings.Contains(out, "example:") || !strings.Contains(out, "see:") {
			t.Fatalf("describe %s = %s", name, out)
		}
	}
	out = run(t, tmp, cli, "ui", "describe", "Field", "--json")
	var comp struct{ Name, Signature, Summary string }
	if err := json.Unmarshal([]byte(out), &comp); err != nil || comp.Name != "Field" || comp.Summary == "" {
		t.Fatalf("describe --json = %s (%v)", out, err)
	}
	out = run(t, tmp, cli, "ui", "describe", "--json")
	var all []struct{ Name, Kind, Group string }
	if err := json.Unmarshal([]byte(out), &all); err != nil || len(all) < 50 {
		t.Fatalf("catalog = %d components (%v)", len(all), err)
	}
	descCmd := exec.Command(cli, "ui", "describe", "Fild")
	descCmd.Dir = tmp
	if out, err := descCmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "did you mean: Field") {
		t.Fatal(string(out), err)
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
	bin := exec.CommandContext(ctx, exeName(filepath.Join(proj, "bin", "app")))
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

// buildCLI compiles the CLI into tmp and gives back the path that runs it.
// It exists so that no test has to remember exeName: on Windows a binary
// without the extension is one exec.LookPath refuses, and four tests added at
// once forgot it, which is the sign that the knowledge belongs here and not in
// every caller. The windows job is what caught them.
func buildCLI(t *testing.T, repo, tmp string) string {
	t.Helper()
	cli := exeName(filepath.Join(tmp, "trilha-cli"))
	run(t, repo, "go", "build", "-o", cli, "./cmd/trilha")
	return cli
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

// runErr is run for a command that is supposed to fail: it hands back the
// output and the error instead of ending the test.
func runErr(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
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
	cli := buildCLI(t, repo, tmp)

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
	bin := exeName(filepath.Join(tmp, "farol-bin"))
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
	cli := buildCLI(t, repo, tmp)

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

// TestAddE2E is issue #116 end to end: every recipe, applied to a project
// nobody has touched, and `trilha check` green afterwards.
//
// This is the guarantee that decides whether recipes age well. One that broke
// quietly is worse than no recipe at all, because whoever ran it already has
// its code inside their project — so the CI applies each of them for real.
func TestAddE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	t.Setenv("TRILHA_SECRET", "um-segredo-de-teste-com-mais-de-32-bytes")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "loja")
	run(t, tmp, cli, "new", proj, "--module", "example.com/loja", "--trilha-dir", repo)

	// The listing is how anybody finds out a recipe exists.
	lista := run(t, proj, cli, "add")
	for _, nome := range []string{"audit", "api-keys", "blob", "login", "mail", "permissions", "profile", "settings", "tasks", "users", "webhooks"} {
		if !strings.Contains(lista, nome) {
			t.Fatalf("`trilha add` does not list %s:\n%s", nome, lista)
		}
	}

	// A recipe written on top of another refuses before it writes anything:
	// half a recipe in a project is worse than none, because now there are
	// files to delete before trying again.
	if out, err := runErr(t, proj, cli, "add", "users"); err == nil {
		t.Fatalf("add users wrote without login:\n%s", out)
	} else if !strings.Contains(out, "login") {
		t.Fatalf("the refusal does not say what to run first:\n%s", out)
	}

	// Every recipe into the same project: they have to coexist, because a
	// project that wants one usually wants two.
	for _, nome := range []string{"audit", "settings", "api-keys", "login", "users", "permissions", "profile", "webhooks", "tasks", "mail", "blob"} {
		out := run(t, proj, cli, "add", nome)
		if !strings.Contains(out, "  + ") {
			t.Fatalf("add %s wrote nothing:\n%s", nome, out)
		}
	}

	// The whole point: no edit between adding and the gate being green.
	if out := run(t, proj, cli, "check"); !strings.Contains(out, "test") {
		t.Fatal(out)
	}

	// A second run adds nothing and duplicates nothing.
	again := run(t, proj, cli, "add", "audit")
	if strings.Contains(again, "  + ") {
		t.Fatalf("running it again wrote a file:\n%s", again)
	}
	setup, err := os.ReadFile(filepath.Join(proj, "app", "setup.go"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(setup), "trilha:add audit"); n != 1 {
		t.Fatalf("the marker appears %d times:\n%s", n, setup)
	}
	// And the gate is still green after the second run.
	run(t, proj, cli, "check")
}

// TestGenerateCrudE2E is issue #115 end to end: from a struct to the screens,
// with nobody editing anything in between. It is the only place that proves
// the five generated files agree with each other — the store the pages use,
// the form the store accepts, and the test that walks all three.
func TestGenerateCrudE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	t.Setenv("TRILHA_SECRET", "um-segredo-de-teste-com-mais-de-32-bytes")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "loja")
	run(t, tmp, cli, "new", proj, "--module", "example.com/loja", "--trilha-dir", repo)

	// The struct the project already has, which is the whole premise: a CRUD
	// is generated for something that exists.
	dir := filepath.Join(proj, "internal", "docs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tipo, err := os.ReadFile(filepath.Join(repo, "testdata", "crud", "tipo.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tipo.go"), tipo, 0o644); err != nil {
		t.Fatal(err)
	}

	out := run(t, proj, cli, "generate", "crud", "docs.Tipo", "--at", "app/admin/tipos")
	for _, quero := range []string{
		"internal/docs/tipo_store.go", "app/admin/tipos/page.go",
		"app/admin/tipos/new/page.go", "app/admin/tipos/id_/page.go",
		"tipo_crud_test.go", "app/setup.go", "/admin/tipos",
	} {
		if !strings.Contains(out, quero) {
			t.Fatalf("generate crud did not report %s:\n%s", quero, out)
		}
	}

	// The whole point: no edit between generating and the gate being green.
	// The generated test is inside it, so a CRUD whose screens disagree fails
	// here rather than on somebody's first afternoon with it.
	if out := run(t, proj, cli, "check"); !strings.Contains(out, "test") {
		t.Fatal(out)
	}

	// Generating again writes nothing — there is no --force, because a CRUD is
	// what somebody generates after editing one — and it says what running it
	// again is worth: with the struct unchanged, that the screens have
	// everything.
	out2 := run(t, proj, cli, "generate", "crud", "docs.Tipo", "--at", "app/admin/tipos")
	if !strings.Contains(out2, "already generated") || !strings.Contains(out2, "every field") {
		t.Fatal("the second run does not say what it found:\n" + out2)
	}

	// And with a field the screens do not have — which is the reason somebody
	// runs it a second time — it says which, and where it goes.
	caminhoTipo := filepath.Join(proj, "internal", "docs", "tipo.go")
	src, err := os.ReadFile(caminhoTipo)
	if err != nil {
		t.Fatal(err)
	}
	tag := "`" + `json:"descricao" form:"descricao" validate:"max=200"` + "`"
	grown := strings.Replace(string(src), "type Tipo struct {",
		"type Tipo struct {\n\tDescricao string "+tag, 1)
	mustWrite(t, caminhoTipo, grown)
	out3 := run(t, proj, cli, "generate", "crud", "docs.Tipo", "--at", "app/admin/tipos")
	for _, want := range []string{"Descricao", "ui.Field(\"descricao\"", "app/admin/tipos/new/page.go:"} {
		if !strings.Contains(out3, want) {
			t.Fatalf("the diff does not say %q:\n%s", want, out3)
		}
	}
	// Still nothing written: the file on disk is the one that was edited.
	after, err := os.ReadFile(filepath.Join(proj, "app", "admin", "tipos", "new", "page.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "descricao") {
		t.Fatal("the second run wrote into the form")
	}
}

// TestTemplateAppE2E is issue #65 end to end: `trilha new --template app` has
// to produce a project that is already green — it compiles, `trilha check`
// passes and the tests that come with it pass — without a single edit. A
// skeleton that needs a fix before it runs is not a starting point.
func TestTemplateAppE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "gestao")
	run(t, tmp, cli, "new", proj, "--module", "example.com/gestao", "--template", "app", "--trilha-dir", repo)
	for _, f := range []string{
		"app/page.go", "app/middleware.go", "app/login/page.go", "app/logout/route.go",
		"app/items/page.go", "app/items/new/page.go", "app/items/id_/page.go",
		"internal/store/store.go", "internal/session/session.go", "app_test.go",
		"go.mod", "trilha_gen.go", "public/ui.css",
	} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	// The middleware at the root of app/ is what protects the tree, so every
	// route below it shows up in the listing of routes.
	out := run(t, proj, cli, "routes")
	for _, want := range []string{"/login", "/items", "/items/new", "/items/{id}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("route %s missing from:\n%s", want, out)
		}
	}
	// check is gen + gofmt + vet + test + audit: the whole bar the project is
	// held to from its first day.
	t.Setenv("TRILHA_SECRET", "an-e2e-secret-with-more-than-32-bytes!!")
	if out := run(t, proj, cli, "check"); !strings.Contains(out, "ok") {
		t.Fatal(out)
	}

	// --with "" is a choice and not an absence: somebody who typed it asked
	// for the skeleton, and gets it.
	pelado := filepath.Join(tmp, "pelado")
	run(t, tmp, cli, "new", pelado, "--module", "example.com/pelado", "--template", "app",
		"--with", "", "--trilha-dir", repo, "--no-tidy")
	if _, err := os.Stat(filepath.Join(pelado, "app", "admin", "auditoria")); err == nil {
		t.Fatal(`--with "" still wrote the administration screens`)
	}
	// And the recipes are not the app template's alone.
	blog := filepath.Join(tmp, "blogue")
	run(t, tmp, cli, "new", blog, "--module", "example.com/blogue",
		"--with", "audit", "--trilha-dir", repo, "--no-tidy")
	if _, err := os.Stat(filepath.Join(blog, "app", "auditoria", "page.go")); err != nil {
		t.Fatal("--with audit did nothing on the blog template:", err)
	}

	// An unknown shape is a message, not a stack trace.
	cmd := exec.Command(cli, "new", filepath.Join(tmp, "nope"), "--template", "banana", "--no-tidy")
	cmd.Dir = tmp
	if b, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(b), "--template") {
		t.Fatalf("expected a complaint about --template, got %v\n%s", err, b)
	}
}

// TestMigrateNextE2E is issue #59 end to end: the skeleton the migration writes
// has to compile in a real project, not only match a golden. What it proves is
// that a team can run the command on day one, run `trilha gen`, and still have
// something that builds — the port then happens screen by screen, on green.
func TestMigrateNextE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "portado")
	run(t, tmp, cli, "new", proj, "--module", "example.com/portado", "--trilha-dir", repo)

	next := filepath.Join(repo, "testdata", "next")
	out := run(t, proj, cli, "migrate", "next", next, "--dry-run")
	if !strings.Contains(out, "would be written") {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(proj, "MIGRATION.md")); err == nil {
		t.Fatal("--dry-run must write nothing")
	}
	out = run(t, proj, cli, "migrate", "next", next)
	// The page trilha new wrote is kept: the migration adds, it does not take over.
	if !regexp.MustCompile(`app/page\.go\s+kept`).MatchString(out) {
		t.Fatal(out)
	}
	if !strings.Contains(out, "no equivalent here") {
		t.Fatal(out)
	}
	report, err := os.ReadFile(filepath.Join(proj, "MIGRATION.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| `app/documents/[id]/page.tsx`", "B — polling", "root layout", "trilha ui describe"} {
		if !strings.Contains(string(report), want) {
			t.Fatalf("MIGRATION.md missing %q:\n%s", want, report)
		}
	}
	// The routes are new, so the generated file is stale until `gen` runs.
	checkCmd := exec.Command(cli, "gen", "--check")
	checkCmd.Dir = proj
	if _, err := checkCmd.CombinedOutput(); err == nil {
		t.Fatal("gen --check must fail before gen")
	}
	run(t, proj, cli, "gen")
	run(t, proj, cli, "gen", "--check")
	run(t, proj, "go", "build", "./...")
	run(t, proj, "go", "vet", "./...")
	out = run(t, proj, cli, "routes")
	for _, want := range []string{"/documents/{id}", "/files/{path...}", "/api/documents"} {
		if !strings.Contains(out, want) {
			t.Fatalf("routes missing %s:\n%s", want, out)
		}
	}

	// The other half of the same migration (issue #59 writes the screens, #61
	// writes the types they read): the client of the API that stayed where it
	// was has to compile inside the project that just appeared.
	doc := filepath.Join(repo, "testdata", "openapi", "acervo.json")
	out = run(t, proj, cli, "client", doc)
	if !strings.Contains(out, "internal/api/client.go") || !strings.Contains(out, "oneOf/anyOf") {
		t.Fatal(out)
	}
	gen := filepath.Join(proj, "internal", "api", "client.go")
	if b, _ := os.ReadFile(gen); !strings.Contains(string(b), "func (g *Documents) List(") {
		t.Fatalf("client.go = %.400s", b)
	}
	run(t, proj, "go", "build", "./...")
	if out := run(t, proj, cli, "client", doc, "--check"); !strings.Contains(out, "up to date") {
		t.Fatal(out)
	}
	// The document moved and nobody regenerated: that is what --check is for.
	b, _ := os.ReadFile(gen)
	os.WriteFile(gen, append(b, []byte("\n// touched\n")...), 0o644)
	stale := exec.Command(cli, "client", doc, "--check")
	stale.Dir = proj
	if out, err := stale.CombinedOutput(); err == nil || !strings.Contains(string(out), "out of date") {
		t.Fatal(string(out), err)
	}
}

// Issue #70: the types of the islands come out of the same command that keeps
// trilha_gen.go honest, so nobody has to remember a second one.
func TestIslandTypesE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "ilhas")
	run(t, tmp, cli, "new", proj, "--module", "example.com/ilhas", "--trilha-dir", repo)
	dts := filepath.Join(proj, "public", "islands.d.ts")

	// An app with no island gets no declaration file.
	run(t, proj, cli, "gen")
	if _, err := os.Stat(dts); err == nil {
		t.Fatal("an app without islands should not get islands.d.ts")
	}

	page := filepath.Join(proj, "app", "page.go")
	src := `package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// CounterProps is what the counter starts from.
type CounterProps struct {
	Start int      ` + "`json:\"start\"`" + `
	Tags  []string ` + "`json:\"tags,omitempty\"`" + `
}

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(c.Island("/counter.js", CounterProps{Start: 1})), nil
}
`
	if err := os.WriteFile(page, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := run(t, proj, cli, "gen")
	if !strings.Contains(out, "islands.d.ts") {
		t.Fatal(out)
	}
	got, err := os.ReadFile(dts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"/counter.js": CounterProps;`,
		"start: number;",
		"tags?: string[];",
		"export interface TrilhaIsland {",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("islands.d.ts missing %q:\n%s", want, got)
		}
	}
	run(t, proj, cli, "gen", "--check")

	// A prop that changed and a file nobody regenerated: --check is the line in
	// the CI that says so.
	if err := os.WriteFile(dts, append(got, []byte("// stale\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := exec.Command(cli, "gen", "--check")
	stale.Dir = proj
	if out, err := stale.CombinedOutput(); err == nil {
		t.Fatalf("gen --check accepted a stale islands.d.ts:\n%s", out)
	}

	// The last island gone takes the generated file with it.
	run(t, proj, cli, "gen")
	if err := os.WriteFile(page, []byte(strings.Replace(src,
		`c.Island("/counter.js", CounterProps{Start: 1})`, `h.Text("nada")`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, proj, cli, "gen")
	if _, err := os.Stat(dts); err == nil {
		t.Fatal("the declaration outlived the last island")
	}
}

// Issue #70: a JavaScript module an island needs comes down once, lands in the
// repository and is pinned. There is no resolver, no manifest and no install
// step — the file is the dependency, and vendor.lock is the proof.
func TestVendorE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	const module = "export const h = () => {};\n"
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/preact@10.19.3" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		io.WriteString(w, module)
	}))
	defer cdn.Close()

	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	cli := buildCLI(t, repo, tmp)
	proj := filepath.Join(tmp, "ilhas")
	run(t, tmp, cli, "new", proj, "--module", "example.com/ilhas", "--trilha-dir", repo)

	out := run(t, proj, cli, "vendor", "preact@10.19.3", "--from", cdn.URL)
	if !strings.Contains(out, "public/vendor/preact.js") {
		t.Fatal(out)
	}
	got, err := os.ReadFile(filepath.Join(proj, "public", "vendor", "preact.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != module {
		t.Fatalf("what came down is not what was served: %q", got)
	}
	lock, err := os.ReadFile(filepath.Join(proj, "vendor.lock"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(module))
	for _, want := range []string{"preact 10.19.3", hex.EncodeToString(sum[:]), "public/vendor/preact.js", cdn.URL} {
		if !strings.Contains(string(lock), want) {
			t.Fatalf("vendor.lock missing %q:\n%s", want, lock)
		}
	}
	run(t, proj, cli, "vendor", "--check")
	if out := run(t, proj, cli, "vendor"); !strings.Contains(out, "preact") {
		t.Fatal(out)
	}

	// A file that changed under a version that did not is exactly what the lock
	// is for.
	if err := os.WriteFile(filepath.Join(proj, "public", "vendor", "preact.js"),
		[]byte(module+"// and something else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := exec.Command(cli, "vendor", "--check")
	bad.Dir = proj
	outb, err := bad.CombinedOutput()
	if err == nil || !strings.Contains(string(outb), "changed since it was pinned") {
		t.Fatalf("--check accepted a changed module: %v\n%s", err, outb)
	}

	// And a module somebody dropped in by hand is not vendored, it is just there.
	run(t, proj, cli, "vendor", "preact@10.19.3", "--from", cdn.URL)
	if err := os.WriteFile(filepath.Join(proj, "public", "vendor", "htm.js"), []byte("//\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loose := exec.Command(cli, "vendor", "--check")
	loose.Dir = proj
	outl, err := loose.CombinedOutput()
	if err == nil || !strings.Contains(string(outl), "not in vendor.lock") {
		t.Fatalf("--check accepted an unpinned module: %v\n%s", err, outl)
	}
}

// Spec 061 (#50): the MCP server is a wrapper, and this is what proves it —
// the tool answers byte for byte what the command answers. It also checks the
// part that is a security property and not a feature: without --write, the
// tool that writes is not in tools/list at all.
func TestMCPServerE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "servido")
	run(t, tmp, cli, "new", proj, "--module", "example.com/servido", "--trilha-dir", repo)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	dial := func(args ...string) *mcp.Client {
		t.Helper()
		c, err := mcp.Dial(ctx, func(context.Context) (mcp.Transport, error) {
			cmd := exec.CommandContext(ctx, cli, args...)
			cmd.Dir = proj // the project is decided here, not by any argument
			cmd.Stderr = io.Discard
			in, err := cmd.StdinPipe()
			if err != nil {
				return nil, err
			}
			out, err := cmd.StdoutPipe()
			if err != nil {
				return nil, err
			}
			if err := cmd.Start(); err != nil {
				return nil, err
			}
			return mcp.Pipe(out, in, func() error {
				_ = in.Close()
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return nil
			}), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}

	names := func(c *mcp.Client) []string {
		t.Helper()
		list, err := c.ListTools(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, i := range list {
			out = append(out, i.Name)
		}
		sort.Strings(out)
		return out
	}

	// Read-only by default: generate is not offered, so it cannot be called.
	ro := dial("mcp")
	got := names(ro)
	if strings.Join(got, ",") != "check,describe_project,routes,ui_describe" {
		t.Fatalf("tools without --write = %v", got)
	}
	if !refused(t, ctx, ro, "generate", `{"kind":"page","target":"/x"}`) {
		t.Fatal("a tool that was never offered answered anyway")
	}

	// The proof that it is a wrapper: the same bytes as the command.
	res, err := ro.CallTool(ctx, "routes", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if direct := run(t, proj, cli, "routes"); res.Text() != direct {
		t.Fatalf("the tool and the command disagree:\ntool: %q\ncmd:  %q", res.Text(), direct)
	}

	// The catalogue answers without a project and without a process.
	if res, err := ro.CallTool(ctx, "ui_describe", json.RawMessage(`{"component":"Field"}`)); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(res.Text(), "Field") {
		t.Fatalf("ui_describe = %q", res.Text())
	}
	if !refused(t, ctx, ro, "ui_describe", `{"component":"../../etc/passwd"}`) {
		t.Fatal("a name that is a path was accepted")
	}

	// With --write the tool appears, and writing is all it does differently.
	rw := dial("mcp", "--write")
	if got := names(rw); strings.Join(got, ",") != "check,describe_project,generate,routes,ui_describe" {
		t.Fatalf("tools with --write = %v", got)
	}
	if _, err := rw.CallTool(ctx, "generate", json.RawMessage(`{"kind":"page","target":"/relatorio"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(proj, "app", "relatorio", "page.go")); err != nil {
		t.Fatalf("generate did not write the page: %v", err)
	}
	// And a refusal stays a refusal even when the tool is offered.
	if !refused(t, ctx, rw, "generate", `{"kind":"page","target":"/../../etc/x"}`) {
		t.Fatal("a path that climbs out of the project was accepted")
	}
}

// refused says whether the server turned the call down. A tool error travels
// inside the result (isError), not as a transport error: that is the MCP
// contract, and a test that checks the Go error instead would pass while the
// server happily answered.
func refused(t *testing.T, ctx context.Context, c *mcp.Client, tool, args string) bool {
	t.Helper()
	res, err := c.CallTool(ctx, tool, json.RawMessage(args))
	if err != nil {
		return true
	}
	return res.IsError
}
