package docs

import (
	"strings"
	"testing"
)

// recipePage is one cookbook page that teaches, by hand, what a `trilha add`
// recipe already writes into a project.
type recipePage struct {
	// Slug is the English cookbook slug; PT is the Portuguese one — the two
	// sections do not share slugs (spec 143).
	Slug, PT, Recipe string
}

// recipePages is the map the issue (#193) asked for: cookbook/database,
// sessions, uploads, files, email, tasks and webhooks each teach what one
// `trilha add` recipe writes; uploads and files both map to blob, one for
// the low-level c.File rules and one for the module built on top of them.
var recipePages = []recipePage{
	{"database", "banco-de-dados", "store"},
	{"sessions", "sessoes", "login"},
	{"uploads", "uploads", "blob"},
	{"files", "arquivos", "blob"},
	{"email", "email", "mail"},
	{"tasks", "tarefas", "tasks"},
	{"webhooks", "webhooks", "webhooks"},
}

// TestCookbookLinksRecipe is the alarm for #193: a cookbook page that
// explains by hand what a recipe already writes has to say so, in both
// locales — otherwise the shortcut only ever shows up as one line in the
// reference/cli.md#trilha-add table.
func TestCookbookLinksRecipe(t *testing.T) {
	for _, rp := range recipePages {
		en, ok := Get("en", "cookbook", rp.Slug)
		if !ok {
			t.Fatalf("no en cookbook/%s page", rp.Slug)
		}
		want := "trilha add " + rp.Recipe
		if !strings.Contains(en.Body, want) {
			t.Errorf("cookbook/%s: does not mention %q", rp.Slug, want)
		}

		pt, ok := Get("pt", "receitas", rp.PT)
		if !ok {
			t.Fatalf("no pt receitas/%s page", rp.PT)
		}
		if !strings.Contains(pt.Body, want) {
			t.Errorf("receitas/%s: does not mention %q", rp.PT, want)
		}
	}
}

// TestCookbookIndexListsRecipe is the alarm for the table in cookbook/index:
// every page that has a recipe names it, as code, in both locales. The
// column header alone ("Recipe") already existed before spec 143 as the
// label of the page-name column, so the check is for the recipe names
// themselves.
func TestCookbookIndexListsRecipe(t *testing.T) {
	en, ok := Get("en", "cookbook", "")
	if !ok {
		t.Fatal("no en cookbook index")
	}
	pt, ok := Get("pt", "receitas", "")
	if !ok {
		t.Fatal("no pt receitas index")
	}
	seen := map[string]bool{}
	for _, rp := range recipePages {
		if seen[rp.Recipe] {
			continue
		}
		seen[rp.Recipe] = true
		code := "`" + rp.Recipe + "`"
		if !strings.Contains(en.Body, code) {
			t.Errorf("cookbook/index: table does not name %s", code)
		}
		if !strings.Contains(pt.Body, code) {
			t.Errorf("receitas/index: a tabela não nomeia %s", code)
		}
	}
}

// TestCLIDocumentsAddMechanics is the alarm for the other half of #193:
// trilha new --with and the Needs mechanism a recipe uses to refuse when
// the one it depends on is missing, neither of which had a name on the
// page before spec 143.
func TestCLIDocumentsAddMechanics(t *testing.T) {
	cases := []struct{ locale, section string }{
		{"en", "reference"},
		{"pt", "referencia"},
	}
	for _, c := range cases {
		p, ok := Get(c.locale, c.section, "cli")
		if !ok {
			t.Fatalf("no %s cli page", c.locale)
		}
		if !strings.Contains(p.Body, "--with") {
			t.Errorf("%s reference/cli: does not mention --with", c.locale)
		}
		if !strings.Contains(p.Body, "Needs") {
			t.Errorf("%s reference/cli: does not mention Needs", c.locale)
		}
	}
}
