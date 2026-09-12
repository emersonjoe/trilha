package docs

import (
	"strings"
	"testing"
)

// templateRecipes is what `trilha new --template app` writes today, in the
// order recipeList (cmd/trilha/new.go) asks for them. The page is the tour of
// these eight screens, so it names all eight.
var templateRecipes = []string{"login", "audit", "api-keys", "settings", "users", "permissions", "profile", "tenant"}

// notInTemplate is the other half of the answer: four recipes exist and the
// template does not start with them. A chapter that lists eleven screens and
// shows a project with eight sends somebody looking for a folder that is not
// there, so the page names these as the ones that are one command away.
var notInTemplate = []string{"approvals", "search", "connections", "share-link"}

// TestAdminAppPage is the alarm for #194: the app template is the most
// finished thing in the framework and the site had no page showing it — what
// the command writes, which screens exist, who gets in where.
func TestAdminAppPage(t *testing.T) {
	pages := []struct{ locale, section, slug string }{
		{"en", "cookbook", "admin-app"},
		{"pt", "receitas", "app-administravel"},
	}
	for _, p := range pages {
		page, ok := Get(p.locale, p.section, p.slug)
		if !ok {
			t.Fatalf("no %s/%s page", p.section, p.slug)
		}
		for _, r := range templateRecipes {
			if !strings.Contains(page.Body, r) {
				t.Errorf("%s: does not name the %q recipe the template writes", page.Path(), r)
			}
		}
		for _, r := range notInTemplate {
			if !strings.Contains(page.Body, r) {
				t.Errorf("%s: does not say %q is not in the template", page.Path(), r)
			}
		}
		// The seven steps of the chapter end in production, and the first
		// one is the command itself: without it the page is a description
		// of a project nobody has.
		for _, want := range []string{"trilha new empresa --template app", "trilha generate crud", "auth.Tenant", "trilha check"} {
			if !strings.Contains(page.Body, want) {
				t.Errorf("%s: does not mention %q", page.Path(), want)
			}
		}
	}
}

// TestAdminAppIsListed is the other half: a page nobody links to is a page
// nobody reads. It belongs in the cookbook table and in the list of example
// applications, which is where somebody looking for a complete app goes.
func TestAdminAppIsListed(t *testing.T) {
	cases := []struct{ locale, section, index, examples, href string }{
		{"en", "cookbook", "", "learn", "/cookbook/admin-app"},
		{"pt", "receitas", "", "aprender", "/pt/receitas/app-administravel"},
	}
	for _, c := range cases {
		idx, ok := Get(c.locale, c.section, c.index)
		if !ok {
			t.Fatalf("no %s index", c.section)
		}
		if !strings.Contains(idx.Body, c.href) {
			t.Errorf("%s index: the table does not link %s", c.section, c.href)
		}
		slug := "examples"
		if c.locale == "pt" {
			slug = "exemplos"
		}
		ex, ok := Get(c.locale, c.examples, slug)
		if !ok {
			t.Fatalf("no %s/%s page", c.examples, slug)
		}
		if !strings.Contains(ex.Body, c.href) {
			t.Errorf("%s/%s: does not link %s", c.examples, slug, c.href)
		}
	}
}
