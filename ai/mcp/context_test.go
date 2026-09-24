package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/ai"
)

// contextProject is a tiny project for the context tools: one page, one
// route, a recipe marker in setup.go, and a file worth searching.
func contextProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/loja\n\ngo 1.22\n")
	write("app/setup.go", `package app

import (
	"github.com/emersonjoe/trilha"
)

// Setup runs once.
func Setup(a *trilha.App) error {
	// trilha:add login
	trilha.Provide(a, usuarios.New(a.Logger()))
	return nil
}
`)
	write("app/page.go", `package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /.
func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Loja"))), nil
}
`)
	write("app/entrar/page.go", `package entrar

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /entrar.
func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Entrar"))), nil
}
`)
	return dir
}

func callTool(t *testing.T, tool *ai.Tool, args string) string {
	t.Helper()
	out, err := tool.Func(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("%s: %v", tool.Name, err)
	}
	return out
}

// TestGetContext walks the tools the way a model would: the whole map first,
// then a recipe slice, then a budget that cuts and names the cut.
func TestGetContext(t *testing.T) {
	root := contextProject(t)
	tools := ContextTools(ContextOpts{Root: root, Version: "0.0.0"})
	_ = tools
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want get_context and search_code", len(tools))
	}
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want get_context and search_code", len(tools))
	}
	get := tools[0]

	whole := callTool(t, get, `{"pack":"app"}`)
	for _, want := range []string{"example.com/", "GET /", "login", "tokens est."} {
		if !strings.Contains(whole, want) {
			t.Errorf("the whole map lacks %q:\n%s", want, whole)
		}
	}

	slice := callTool(t, get, `{"pack":"login"}`)
	if !strings.Contains(slice, "/entrar") {
		t.Errorf("the login slice lacks the route the recipe brought:\n%s", slice)
	}
	if strings.Contains(slice, "- `GET /` ") {
		t.Errorf("the login slice answers the whole map:\n%s", slice)
	}

	tight := callTool(t, get, `{"pack":"app","budget":40}`)
	if !strings.Contains(tight, "cut:") {
		t.Errorf("a 40-token budget cut nothing, or said nothing about it:\n%s", tight)
	}

	if _, err := get.Func(context.Background(), json.RawMessage(`{"pack":"billing"}`)); err == nil {
		t.Error("an unknown pack must say what is installed")
	}
}

// TestSearchCode is the window contract: path:line and one line after, a
// handful of matches at most, and never the whole file.
func TestSearchCode(t *testing.T) {
	root := contextProject(t)
	out, err := searchCode(root, "entrar", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "app/entrar/page.go:") {
		t.Fatalf("no path:line window for the page that says entrar:\n%s", out)
	}
	if !strings.Contains(out, "tokens est.") {
		t.Fatalf("the answer prices itself:\n%s", out)
	}
	// A query nobody wrote answers with a sentence, not an error.
	if out, err := searchCode(root, "faturamento-mensal-xyz", 5); err != nil || !strings.Contains(out, "no match") {
		t.Fatalf("empty answer = %q, %v", out, err)
	}
	// The whole file never comes back: the window is a line and its neighbor.
	if strings.Count(out, "\n") > 10 {
		t.Fatalf("the window is too wide:\n%s", out)
	}
}
