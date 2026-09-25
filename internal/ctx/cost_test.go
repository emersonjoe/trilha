package ctx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/recipes"
)

// TestRecipeCtxPackCost holds every recipe's price to what it actually costs:
// the recipe is installed — with what it needs first — into a project that has
// nothing else, and `ctx --pack <recipe>` is measured the way the command
// measures it. A recipe that grew a screen grows its price in the same commit,
// and the failure says the new number.
func TestRecipeCtxPackCost(t *testing.T) {
	for _, r := range recipes.All() {
		r := r
		t.Run(r.Name, func(t *testing.T) {
			root := minimalProject(t)
			installWithNeeds(t, root, r)
			c, err := Build(root, "example.com/x", "test")
			if err != nil {
				t.Fatal(err)
			}
			info, err := RecipeInfoOf(root, r.Name)
			if err != nil {
				t.Fatal(err)
			}
			p, err := c.PackOf(r.Name, []RecipeInfo{info}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if p.Used != r.CtxPackCost {
				t.Errorf("CtxPackCost = %d, the pack costs %d est. tokens: update the recipe", r.CtxPackCost, p.Used)
			}
			if r.CtxPackCost <= 0 {
				t.Error("a recipe with no price is a recipe nobody can budget for")
			}
		})
	}
}

// A recipe that writes no marker of its own — users only ties itself to
// others — is still recognised as installed: by its files.
func TestInstalledRecipesWithoutMarker(t *testing.T) {
	root := minimalProject(t)
	r, err := recipes.Get("users")
	if err != nil {
		t.Fatal(err)
	}
	installWithNeeds(t, root, r)
	var names []string
	for _, info := range InstalledRecipes(root) {
		names = append(names, info.Name)
	}
	got := strings.Join(names, " ")
	for _, want := range []string{"login", "users"} {
		if !strings.Contains(" "+got+" ", " "+want+" ") {
			t.Errorf("installed = %q, missing %s", got, want)
		}
	}

	// And a backoffice made of recipes: itself, and each of its parts, with
	// the screens found where they landed — under app/admin/.
	root = minimalProject(t)
	admin, err := recipes.Get("admin")
	if err != nil {
		t.Fatal(err)
	}
	installWithNeeds(t, root, admin)
	byName := map[string]RecipeInfo{}
	for _, info := range InstalledRecipes(root) {
		byName[info.Name] = info
	}
	for _, want := range []string{"admin", "approvals", "audit", "login", "search", "users"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("installed = %v, missing %s", byName, want)
		}
	}
	if files := strings.Join(byName["audit"].Files, " "); !strings.Contains(files, "app/admin/auditoria/page.go") {
		t.Errorf("audit's screen was not found under app/admin/: %s", files)
	}
}

func minimalProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/x\n\ngo 1.22\n")
	write("app/page.go", "package app\n\nimport (\n\t\"github.com/emersonjoe/trilha\"\n\t\"github.com/emersonjoe/trilha/h\"\n)\n\n"+
		"// Page is the home.\nfunc Page(c *trilha.Ctx) (h.Node, error) { return h.Text(\"x\"), nil }\n")
	return root
}

// installWithNeeds adds what the recipe needs, depth first, and then the
// recipe — the order somebody following the refusals would take.
func installWithNeeds(t *testing.T, root string, r recipes.Recipe) {
	t.Helper()
	for _, need := range r.Needs {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(need.File))); err == nil {
			continue
		}
		dep, err := recipes.Get(need.Recipe)
		if err != nil {
			t.Fatal(err)
		}
		installWithNeeds(t, root, dep)
	}
	if _, err := recipes.Add(root, r, recipes.Options{Module: "example.com/x", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
}
