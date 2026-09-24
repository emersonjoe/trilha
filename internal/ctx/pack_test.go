package ctx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/tokbudget"
)

func pack(t *testing.T, name string, installed []RecipeInfo, budget int) *Pack {
	t.Helper()
	c := build(t, "full")
	p, err := c.PackOf(name, installed, budget)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPackWholeApp is the no-budget pack: every route, the recipes handed in,
// the conventions the tree actually uses, and the footer with the estimate
// that the Used field answers for.
func TestPackWholeApp(t *testing.T) {
	p := pack(t, "app", []RecipeInfo{{Name: "login", Doc: "/reference/auth"}}, 0)
	if len(p.Routes) != 8 {
		t.Fatalf("routes = %d, want the whole map's 8", len(p.Routes))
	}
	if len(p.Recipes) != 1 || p.Recipes[0].Name != "login" {
		t.Fatalf("recipes: %+v", p.Recipes)
	}
	if len(p.Conventions) == 0 {
		t.Fatal("a full app uses conventions; none were named")
	}
	var apis []string
	for _, r := range p.Routes {
		if strings.HasSuffix(r.File, "route.go") {
			apis = append(apis, r.Pattern)
		}
	}
	if len(p.APIPaths) != len(apis) {
		t.Fatalf("api paths = %v, want %v", p.APIPaths, apis)
	}
	md := p.Markdown()
	if !strings.Contains(md, "tokens est. (4 chars/token)") {
		t.Fatalf("footer without the est. label:\n%s", md)
	}
	if p.Used != tokbudget.Estimate(md) {
		t.Fatalf("Used = %d, the rendered page costs %d", p.Used, tokbudget.Estimate(md))
	}
	// JSON and Markdown answer for the same numbers.
	j, err := p.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(j), `"used_tokens"`) {
		t.Fatalf("json without the used field:\n%s", j)
	}
}

// TestPackCutOrder is the contract the doc-comment states: under a budget
// that fits only some of it, contracts go first, then recipes, then
// conventions — and routes are never cut.
func TestPackCutOrder(t *testing.T) {
	c := build(t, "full")
	all, err := c.PackOf("app", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// A budget that fits the routes and nothing else: everything else is cut,
	// in the documented order.
	tight, err := c.PackOf("app", []RecipeInfo{{Name: "login"}}, len(all.Routes)*20)
	if err != nil {
		t.Fatal(err)
	}
	if len(tight.Routes) != 8 {
		t.Fatalf("routes were cut: %d remain", len(tight.Routes))
	}
	want := []string{"contracts", "recipes", "conventions"}
	if strings.Join(tight.Truncated, ",") != strings.Join(want, ",") {
		t.Fatalf("truncated = %v, want %v", tight.Truncated, want)
	}
	// A generous budget cuts nothing.
	roomy, err := c.PackOf("app", []RecipeInfo{{Name: "login"}}, tokbudget.Estimate(all.Markdown())+100)
	if err != nil {
		t.Fatal(err)
	}
	if len(roomy.Truncated) != 0 {
		t.Fatalf("roomy budget still cut %v", roomy.Truncated)
	}
}

// TestPackRecipeSlice keeps only the routes the recipe's files answer.
func TestPackRecipeSlice(t *testing.T) {
	login := RecipeInfo{Name: "login", Files: []string{"app/blog/novo/page.go"}}
	p := pack(t, "login", []RecipeInfo{login}, 0)
	if len(p.Routes) != 1 || p.Routes[0].Pattern != "/blog/novo" {
		t.Fatalf("slice = %+v, want only /blog/novo", p.Routes)
	}
	if len(p.Recipes) != 1 || p.Recipes[0].Name != "login" {
		t.Fatalf("recipes: %+v", p.Recipes)
	}
}

// TestPackUnknownRecipe says what is installed instead of a bare no.
func TestPackUnknownRecipe(t *testing.T) {
	c := build(t, "full")
	_, err := c.PackOf("billing", []RecipeInfo{{Name: "login"}}, 0)
	if err == nil || !strings.Contains(err.Error(), "billing") || !strings.Contains(err.Error(), "login") {
		t.Fatalf("err = %v, want it naming the recipe and what is installed", err)
	}
}

// TestCtxNeverLeaksSecrets is the negative test the spec asks for: a secret
// written into a Provide call — the worst case, because it is in the source
// the map quotes — must not survive into any output. Names and types stay;
// the value does not.
func TestCtxNeverLeaksSecrets(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := "segredo-super-secreto-nao-pode-vazar"
	setup := `package app

import (
	"github.com/emersonjoe/trilha"
)

// Setup runs once.
func Setup(a *trilha.App) error {
	trilha.Provide(a, trilha.NewConnections(trilha.ConnectionsOpts{
		Kinds: []trilha.ConnectionKind{{Name: "provider", Secret: trilha.Secret("` + secret + `")}},
	}))
	return nil
}
`
	if err := os.WriteFile(filepath.Join(app, "setup.go"), []byte(setup), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Build(dir, "example.com/leak", "0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.PackOf("app", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"markdown":     c.Markdown(Compact),
		"markdown all": c.Markdown(All),
		"pack":         p.Markdown(),
	}
	j, err := c.JSON()
	if err != nil {
		t.Fatal(err)
	}
	outputs["json"] = string(j)
	pj, err := p.JSON()
	if err != nil {
		t.Fatal(err)
	}
	outputs["pack json"] = string(pj)
	for name, out := range outputs {
		if strings.Contains(out, secret) {
			t.Errorf("%s: the secret leaked:\n%s", name, out)
		}
	}
	// The map still says what is provided — the shape, not the value.
	if !strings.Contains(outputs["markdown"], "provides") {
		t.Error("the mask ate the map: the provided value is gone")
	}
	// And the unit itself masks what it is handed.
	if got := maskLiterals(`Kinds: []trilha.ConnectionKind{{Name: "x"}}`); strings.Contains(got, "x") {
		t.Errorf("maskLiterals kept a literal: %q", got)
	}
}
