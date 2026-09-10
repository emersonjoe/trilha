package migrate

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// importRe finds the relative imports of a module — what the file pulls in
// from beside it — together with the clause that says which names it asked
// for. A package from node_modules is not one of these and is not followed:
// the point is the component the author wrote, not the library.
//
// The clause matters because of the barrel. `import { DataTable } from
// "@/components"` asks for one name, and an index.ts that re-exports twenty
// files is an index and not a dependency — reading it as one put the shell's
// chat inside twenty screens that never render it.
var importRe = regexp.MustCompile(`(?s)\b(?:import|export)\b([^'";]*?)\bfrom\s*['"]([^'"]+)['"]`)

// reexportRe reads one line of a barrel: the names it forwards, or a star,
// and the module they come from.
var reexportRe = regexp.MustCompile(`(?s)^\s*export\s+(?:type\s+)?(\*|\{[^}]*\})(?:\s+as\s+\w+)?\s*from\s*['"]([^'"]+)['"]\s*;?\s*$`)

// commentRe is what has to come out before a file can be called a barrel: a
// barrel with a licence header on top is still a barrel.
var commentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

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
	var walk func(from, body string, depth int)
	// take follows one import: the file, or — when the file turns out to be a
	// barrel — only the re-export lines that forward a name somebody asked for.
	var take func(file string, want map[string]bool, depth int)

	take = func(file string, want map[string]bool, depth int) {
		if depth > maxDepth || len(out) >= maxFiles || file == "" || out[file] != "" || file == source {
			return
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return
		}
		body := string(b)
		if lines, ok := barrel(body); ok {
			// Re-exporting is not using. The barrel itself is not work — it is
			// a table of contents — so it is not recorded, and only the line
			// that forwards the wanted name is followed.
			for _, spec := range through(lines, want) {
				take(resolve(root, join(base, file, spec)), all, depth+1)
			}
			return
		}
		out[file] = body
		if len(out) >= maxFiles {
			return
		}
		walk(file, body, depth+1)
	}

	walk = func(from, body string, depth int) {
		if depth > maxDepth || len(out) >= maxFiles {
			return
		}
		for _, m := range importRe.FindAllStringSubmatch(body, -1) {
			spec := m[2]
			if !local(spec) {
				continue
			}
			take(resolve(root, join(base, from, spec)), wanted(m[1]), depth)
		}
	}

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source)))
	if err != nil {
		return out
	}
	walk(source, string(b), 1)
	return out
}

// all is the request of somebody who asked for everything: a namespace import,
// or anything reached past a barrel.
var all = map[string]bool{"*": true}

// local says whether an import specifier names a file of this project.
func local(spec string) bool {
	return strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "@/")
}

// join turns an import specifier into a path from the project root, resolving
// the @/ alias the way a Next project sets it up.
func join(base, from, spec string) string {
	if strings.HasPrefix(spec, "@/") {
		return path.Join(base, strings.TrimPrefix(spec, "@/"))
	}
	target := path.Join(path.Dir(from), spec)
	// An import that climbs above the project is not ours to read.
	if strings.HasPrefix(target, "..") {
		return ""
	}
	return target
}

// wanted reads the names an import clause asked for. A default import and a
// namespace import both ask for everything the module has, and say so.
func wanted(clause string) map[string]bool {
	out := map[string]bool{}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(clause), "type"))
	if i := strings.Index(rest, "{"); i >= 0 {
		j := strings.Index(rest[i:], "}")
		if j < 0 {
			return all
		}
		for _, part := range strings.Split(rest[i+1:i+j], ",") {
			// "Chat as ChatWidget" is asking the module for Chat.
			if f := strings.Fields(strings.TrimSpace(part)); len(f) > 0 {
				out[strings.TrimPrefix(f[0], "type")] = true
			}
		}
		// `import Shell, { Chat } from "x"` asks for both, and the default
		// half has no name of its own here.
		if strings.Contains(rest[:i], ",") || strings.Contains(rest[:i], "*") {
			out["*"] = true
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

// barrel says whether a file is an index and nothing else: every statement in
// it forwards something from somewhere. Anything less certain is read as the
// file it is, which is what the analyzer did before there was a barrel at all.
func barrel(body string) ([]string, bool) {
	var lines []string
	for _, raw := range strings.Split(commentRe.ReplaceAllString(body, ""), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if !reexportRe.MatchString(line) {
			return nil, false
		}
		lines = append(lines, line)
	}
	return lines, len(lines) > 0
}

// through picks the re-export lines that forward one of the wanted names. A
// star forwards names it does not spell, so it is followed only when nothing
// named answered — otherwise one `export *` would undo the whole point.
func through(lines []string, want map[string]bool) []string {
	var named, stars []string
	for _, line := range lines {
		m := reexportRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == "*" {
			stars = append(stars, m[2])
			continue
		}
		if want["*"] || exports(m[1], want) {
			named = append(named, m[2])
		}
	}
	if len(named) > 0 {
		return named
	}
	return stars
}

// exports says whether a re-export clause forwards a wanted name. The name
// that counts is the one on the outside: `{ default as Chat }` gives Chat.
func exports(clause string, want map[string]bool) bool {
	for _, part := range strings.Split(strings.Trim(clause, "{}"), ",") {
		f := strings.Fields(strings.TrimSpace(part))
		if len(f) == 0 {
			continue
		}
		name := f[0]
		if len(f) >= 3 && f[len(f)-2] == "as" {
			name = f[len(f)-1]
		}
		if want[strings.TrimPrefix(name, "type")] {
			return true
		}
	}
	return false
}

// resolve turns an import specifier into the file it names, the way a bundler
// would: the extension is usually left out, and a folder means its index.
func resolve(root, target string) string {
	if target == "" {
		return ""
	}
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
