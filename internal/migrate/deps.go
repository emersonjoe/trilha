package migrate

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// importRe finds the relative imports of a module: what the page pulls in from
// beside it. A package from node_modules is not one of these and is not
// followed — the point is the component the author wrote, not the library.
var importRe = regexp.MustCompile(`(?m)from\s*['"](\.{1,2}/[^'"]+|@/[^'"]+)['"]`)

// The caps. A page that reaches thirty files or three levels has already told
// us what it is; going further turns one classification into a crawl of the
// whole application, and the answer would not change.
const (
	maxDepth = 3
	maxFiles = 30
)

// deps returns the sources a page pulls in through its own imports, keyed by
// the path each was read from, relative to root and slash-separated.
//
// It exists because in a real Next application the page is thin: the screen
// that is hardest to port is a fifty-line page.tsx importing a component of
// three hundred lines, and reading the page alone puts it in the easiest class.
func deps(root, source string) map[string]string {
	out := map[string]string{}
	// "@/x" is the alias every Next project sets up, and it points at src/
	// when there is one and at the root when there is not.
	base := ""
	if fi, err := os.Stat(filepath.Join(root, "src")); err == nil && fi.IsDir() {
		base = "src"
	}
	var walk func(from string, depth int)
	walk = func(from string, depth int) {
		if depth > maxDepth || len(out) >= maxFiles {
			return
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from)))
		if err != nil {
			return
		}
		for _, m := range importRe.FindAllStringSubmatch(string(body), -1) {
			spec := m[1]
			var target string
			if strings.HasPrefix(spec, "@/") {
				target = path.Join(base, strings.TrimPrefix(spec, "@/"))
			} else {
				target = path.Join(path.Dir(from), spec)
			}
			// An import that climbs above the project is not ours to read.
			if strings.HasPrefix(target, "..") {
				continue
			}
			file := resolve(root, target)
			if file == "" || out[file] != "" || file == source {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				continue
			}
			out[file] = string(b)
			if len(out) >= maxFiles {
				return
			}
			walk(file, depth+1)
		}
	}
	walk(source, 1)
	return out
}

// resolve turns an import specifier into the file it names, the way a bundler
// would: the extension is usually left out, and a folder means its index.
func resolve(root, target string) string {
	for _, try := range []string{
		target + ".tsx", target + ".ts", target + ".jsx", target + ".js",
		path.Join(target, "index.tsx"), path.Join(target, "index.ts"),
	} {
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(try))); err == nil && !fi.IsDir() {
			return try
		}
	}
	return ""
}
