package migrate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Analysis is what a .tsx says about itself. It is read with regular
// expressions over the text, the way the audit reads a Go file: a TypeScript
// parser without external dependencies would be a project of its own, and the
// five questions that matter here are answered by five expressions. The rule
// is printed in the report so it can be argued with.
type Analysis struct {
	Client    bool           // 'use client' at the top
	Hooks     map[string]int // useState, useEffect, useRef...
	Endpoints []Endpoint     // what it calls
	Signals   []string       // the names below that matched
	Class     string         // A, B or C
	Why       string         // the reason for the class, in one clause
}

// Endpoint is one address the screen calls, with the method it calls it with.
type Endpoint struct {
	Method string
	Path   string
}

// The classes, in the order the report explains them.
const (
	ClassForm   = "A" // form and list: no island signal, no polling
	ClassIsland = "B" // small island: polling, modal, tabs, upload
	ClassSPA    = "C" // pointer, drawing surface or editor
)

// hookNames are counted because the count is the size of the job: a screen
// with two useState is a form, one with fourteen is an application.
var hookNames = []string{"useState", "useEffect", "useRef", "useMemo", "useCallback", "useContext"}

// spaSignals are what cannot become a form: the browser is doing the work.
var spaSignals = []struct {
	name string
	re   *regexp.Regexp
}{
	{"pointer", regexp.MustCompile(`onPointer(Down|Move|Up)|onMouseMove|onDrag[A-Z]|draggable=`)},
	{"drawing", regexp.MustCompile(`<svg\b|<canvas\b|getContext\(`)},
	{"editor", regexp.MustCompile(`contentEditable|execCommand|Slate|ProseMirror|Tiptap`)},
	{"chart", regexp.MustCompile(`\bd3[-.]|recharts|chart\.js|Chart\(`)},
}

// islandSignals are what a form cannot do alone, but the kit can.
var islandSignals = []struct {
	name string
	re   *regexp.Regexp
}{
	{"polling", regexp.MustCompile(`setInterval\(|usePoll\(|refetchInterval|EventSource\(`)},
	{"modal", regexp.MustCompile(`<Dialog\b|<Modal\b|showModal\(|<AlertDialog\b`)},
	{"tabs", regexp.MustCompile(`<Tabs\b|role="tab"|<TabsTrigger\b`)},
	{"upload", regexp.MustCompile(`type="file"|new FormData\(|<Dropzone\b`)},
	{"storage", regexp.MustCompile(`localStorage|sessionStorage`)},
	{"raw html", regexp.MustCompile(`dangerouslySetInnerHTML`)},
}

var (
	useClientRe = regexp.MustCompile(`(?m)^\s*['"]use client['"]`)
	// The three quote characters are three branches because Go's regexp has
	// no backreference: a template literal may hold a quote, and
	// `/api/users/${params["user-id"]}` is exactly the call worth reading.
	callRe      = regexp.MustCompile("\\b(apiGet|apiPost|apiPut|apiPatch|apiDelete|fetch)\\s*\\(\\s*(?:\"([^\"]*)\"|'([^']*)'|`([^`]*)`)")
	methodRe    = regexp.MustCompile(`method\s*:\s*["'` + "`" + `](\w+)`)
	exportRe    = regexp.MustCompile(`(?m)^export\s+(?:async\s+)?function\s+([A-Z]+)\s*\(`)
	exportVarRe = regexp.MustCompile(`(?m)^export\s+const\s+([A-Z]+)\s*=`)
	rewriteRe   = regexp.MustCompile(`source\s*:\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `][^}]*?destination\s*:\s*["'` + "`" + `]([^"'` + "`" + `]+)`)
	interpRe    = regexp.MustCompile(`\$\{([^}]*)\}`)
	identRe     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	keyRe       = regexp.MustCompile(`\[\s*["']([^"']+)["']\s*\]\s*$`)
)

// analyze reads one source file.
func analyze(src string) Analysis {
	a := Analysis{Client: useClientRe.MatchString(src), Hooks: map[string]int{}}
	for _, h := range hookNames {
		// useRef<any>(null) is a useRef: the type argument sits between the
		// name and the parenthesis, and counting the name alone would count
		// the import line too.
		if n := len(regexp.MustCompile(`\b`+h+`\s*(<[^>]*>)?\s*\(`).FindAllString(src, -1)); n > 0 {
			a.Hooks[h] = n
		}
	}
	a.Endpoints = endpointsOf(src)
	var spa, island []string
	for _, s := range spaSignals {
		if s.re.MatchString(src) {
			spa = append(spa, s.name)
		}
	}
	for _, s := range islandSignals {
		if s.re.MatchString(src) {
			island = append(island, s.name)
		}
	}
	a.Signals = append(append([]string{}, spa...), island...)
	switch {
	case len(spa) > 0:
		a.Class, a.Why = ClassSPA, strings.Join(spa, " and ")
	case len(island) > 0:
		a.Class, a.Why = ClassIsland, strings.Join(island, " and ")
	default:
		a.Class, a.Why = ClassForm, "no island signal"
	}
	return a
}

// endpointsOf collects the addresses the file calls, deduplicated and sorted:
// what the screen needs from the server, without opening the screen.
func endpointsOf(src string) []Endpoint {
	seen := map[Endpoint]bool{}
	for _, m := range callRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		raw := ""
		for _, g := range [][2]int{{m[4], m[5]}, {m[6], m[7]}, {m[8], m[9]}} {
			if g[0] >= 0 {
				raw = src[g[0]:g[1]]
			}
		}
		p := normalizePath(raw)
		if !strings.HasPrefix(p, "/") {
			continue
		}
		method := "GET"
		switch name {
		case "fetch":
			tail := src[m[1]:min(len(src), m[1]+200)]
			if mm := methodRe.FindStringSubmatch(tail); mm != nil {
				method = strings.ToUpper(mm[1])
			}
		default:
			method = strings.ToUpper(strings.TrimPrefix(name, "api"))
		}
		seen[Endpoint{Method: method, Path: p}] = true
	}
	out := make([]Endpoint, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// normalizePath turns a template literal into an address a person reads:
// `/api/documents/${id}` becomes /api/documents/:id.
func normalizePath(p string) string {
	return interpRe.ReplaceAllStringFunc(p, func(m string) string {
		inner := strings.TrimSpace(interpRe.FindStringSubmatch(m)[1])
		// params["user-id"] is a name, written the way TypeScript has to
		// write it when the name is not an identifier.
		if k := keyRe.FindStringSubmatch(inner); k != nil {
			inner = identifier(k[1])
		}
		if i := strings.LastIndex(inner, "."); i >= 0 {
			inner = inner[i+1:]
		}
		if !identRe.MatchString(inner) {
			return ":param"
		}
		return ":" + inner
	})
}

// handlers lists the HTTP methods a route.ts exports, in the order Trilha
// writes them.
func handlers(src string) []string {
	found := map[string]bool{}
	for _, re := range []*regexp.Regexp{exportRe, exportVarRe} {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			found[m[1]] = true
		}
	}
	var out []string
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		if found[m] {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = []string{"GET"}
	}
	return out
}

// rewrites reads the source/destination pairs of a next.config, which is the
// one thing in that file with somewhere to go here.
func rewrites(src string) []string {
	var out []string
	for _, m := range rewriteRe.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1]+" → "+m[2])
	}
	return out
}

// describeHooks is the hook count as one clause, for the generated comment.
func describeHooks(a Analysis) string {
	var parts []string
	if a.Client {
		parts = append(parts, "'use client'")
	} else {
		parts = append(parts, "server component")
	}
	names := make([]string, 0, len(a.Hooks))
	for k := range a.Hooks {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%d %s", a.Hooks[n], n))
	}
	return strings.Join(parts, ", ")
}

// endpointStrings renders the endpoints for a comment or a table cell.
func endpointStrings(es []Endpoint) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Method+" "+e.Path)
	}
	return out
}
