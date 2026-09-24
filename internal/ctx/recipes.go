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
// recipe brings is looked up where it actually landed. A recipe that writes
// no marker — users, mail, a backoffice made of other recipes — is
// recognised by its files instead: every file it writes outside the screen
// folder has to be there, because one of them alone proves nothing. The CLI
// and the MCP context tools read the same answer.
func InstalledRecipes(root string) []RecipeInfo {
	var marked []string
	if src, err := os.ReadFile(filepath.Join(root, "app", "setup.go")); err == nil {
		var marker = regexp.MustCompile(`^// trilha:add ([a-z0-9-]+)$`)
		for _, line := range strings.Split(string(src), "\n") {
			if m := marker.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				marked = append(marked, m[1])
			}
		}
	}
	seen := map[string]bool{}
	var names []string
	for _, name := range marked {
		if _, err := recipes.Get(name); err == nil && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, r := range recipes.All() {
		if !seen[r.Name] && !writesMarker(r) && hasFixedFiles(root, r) {
			seen[r.Name] = true
			names = append(names, r.Name)
		}
	}
	sort.Strings(names)
	out := make([]RecipeInfo, 0, len(names))
	for _, name := range names {
		info, _ := RecipeInfoOf(root, name)
		out = append(out, info)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// RecipeInfoOf describes one recipe as it sits in the project at root: its
// documentation and the files it brought, each where it actually landed —
// a screen may be under app/ or under app/admin/.
func RecipeInfoOf(root, name string) (RecipeInfo, error) {
	r, err := recipes.Get(name)
	if err != nil {
		return RecipeInfo{}, err
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
	return info, nil
}

// writesMarker says whether the recipe records itself in setup.go.
func writesMarker(r recipes.Recipe) bool {
	for _, in := range r.Setup {
		if in.Marker == "// trilha:add "+r.Name {
			return true
		}
	}
	return false
}

// hasFixedFiles says whether every file the recipe writes outside the screen
// folder exists, and that there is at least one.
func hasFixedFiles(root string, r recipes.Recipe) bool {
	n := 0
	for _, f := range r.Files {
		if strings.Contains(f.Rel, "{{") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.Rel))); err != nil {
			return false
		}
		n++
	}
	return n > 0
}
