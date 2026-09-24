package ctx

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/recipes"
)

// InstalledRecipes names the recipes a project brought in: the markers the
// recipes themselves wrote into setup.go are the record, and each file a
// recipe brings is looked up where it actually landed. Nothing here trusts
// the registry alone — a marker whose files are gone is not installed, and
// the CLI and the MCP context tools read the same answer.
func InstalledRecipes(root string) []RecipeInfo {
	src, err := os.ReadFile(filepath.Join(root, "app", "setup.go"))
	if err != nil {
		return nil
	}
	var marker = regexp.MustCompile(`^// trilha:add ([a-z0-9-]+)$`)
	seen := map[string]bool{}
	var names []string
	for _, line := range strings.Split(string(src), "\n") {
		if m := marker.FindStringSubmatch(strings.TrimSpace(line)); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	sort.Strings(names)
	var out []RecipeInfo
	for _, name := range names {
		r, err := recipes.Get(name)
		if err != nil {
			continue // a marker nobody answers to is not a recipe
		}
		info := RecipeInfo{Name: r.Name, Doc: r.Doc}
		for _, f := range r.Files {
			rest := strings.ReplaceAll(f.Rel, "{{.At}}", "")
			for _, cand := range []string{rest, filepath.ToSlash(filepath.Join("app", rest)), filepath.ToSlash(filepath.Join("app/admin", rest))} {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(cand))); err == nil {
					info.Files = append(info.Files, cand)
					break
				}
			}
		}
		out = append(out, info)
	}
	return out
}
