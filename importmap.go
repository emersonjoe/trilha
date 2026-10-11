package trilha

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// An island module is versioned by Asset (/ilha.js?v=…), and a module it
// imports by a relative path is not: "./motor.js" resolves to /motor.js, and a
// file of public/ without a version is cached for an hour. After a deploy the
// new island ran with the old motor (#298). An import map remaps the resolved
// address to the versioned one, so the import in the code stays as it is.

// importSpec finds the module specifiers of static imports and re-exports,
// side-effect imports and dynamic imports with a literal. It reads text, not a
// syntax tree: a commented-out import adds an entry that is never asked for,
// which is harmless, and a specifier built at run time is not seen.
var importSpec = regexp.MustCompile(`(?:\bfrom\s*|\bimport\s*\(\s*|\bimport\s+)["']([^"'\n]+)["']`)

// importMap is the map of the modules the islands srcs reach through their
// imports: the absolute address each import resolves to, and the versioned
// one. Bare names ("lit") and other origins belong to the app's own map, if
// it has one, and files that are not in Config.Public are left out.
func (a *App) importMap(srcs []string) map[string]string {
	out := map[string]string{}
	if a.cfg.Public == nil {
		return out
	}
	seen := map[string]bool{}
	queue := []string{}
	for _, s := range srcs {
		queue = append(queue, path.Clean("/"+strings.SplitN(s, "?", 2)[0]))
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if seen[p] {
			continue
		}
		seen[p] = true
		for _, dep := range a.moduleImports(p) {
			if a.assetVersion(strings.TrimPrefix(dep, "/")) == "" {
				continue
			}
			out[a.cfg.BasePath+dep] = a.Asset(dep)
			queue = append(queue, dep)
		}
	}
	for _, s := range srcs { // the entry modules are already versioned
		delete(out, a.cfg.BasePath+path.Clean("/"+s))
	}
	return out
}

// moduleImports reads the module at p and returns the paths its imports
// resolve to, relative ("./x.js", "../x.js") and absolute ("/x.js") alike.
// The answer is kept per version of the file, so a page does not read its
// modules on every request and an edit in dev is seen on the next one.
func (a *App) moduleImports(p string) (out []string) {
	key := p + "?" + a.assetVersion(strings.TrimPrefix(p, "/"))
	if v, ok := a.moduleDeps.Load(key); ok {
		return v.([]string)
	}
	src, err := fs.ReadFile(a.cfg.Public, strings.TrimPrefix(p, "/"))
	if err != nil {
		return nil
	}
	defer func() { a.moduleDeps.Store(key, out) }()
	for _, m := range importSpec.FindAllSubmatch(src, -1) {
		spec := string(m[1])
		switch {
		case strings.HasPrefix(spec, "./"), strings.HasPrefix(spec, "../"):
			out = append(out, path.Join(path.Dir(p), spec))
		case strings.HasPrefix(spec, "/") && !strings.HasPrefix(spec, "//"):
			out = append(out, path.Clean(spec))
		}
	}
	return out
}

// injectImportMap writes the map as the first thing in <head>: an import map
// has to be in the document before the first module is fetched. Inline, it
// needs the response's nonce like any other script.
func injectImportMap(out []byte, m map[string]string, nonce string) []byte {
	if len(m) == 0 {
		return out
	}
	lower := bytes.ToLower(out)
	i := bytes.Index(lower, []byte("<head"))
	if i < 0 {
		return out
	}
	end := bytes.IndexByte(out[i:], '>')
	if end < 0 {
		return out
	}
	at := i + end + 1
	js, err := json.Marshal(map[string]any{"imports": m})
	if err != nil {
		return out
	}
	// encoding/json escapes <, > and & already, so the map cannot close the
	// script element it sits in.
	script := `<script type="importmap" nonce="` + nonce + `">` + string(js) + `</script>`
	res := make([]byte, 0, len(out)+len(script))
	res = append(res, out[:at]...)
	res = append(res, script...)
	return append(res, out[at:]...)
}
