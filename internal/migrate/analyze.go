package migrate

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
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
	Deps      []string       // the files it imports that were read too
	DepLines  int            // and how many lines they add to the job
	// Reaches are the global files this screen reaches — the shell and what
	// the shell carries. They are not Deps: a screen wrapped by a frame is not
	// made of it, and counting them classified twenty listings as the chat
	// that sits beside them.
	Reaches []string
	// Actions are the Server Actions this screen imports: what it writes. They
	// are not a signal and do not change the class — an action is the opposite
	// of JavaScript in the browser — but a screen with four of them is not the
	// "nothing to do" that an A on its own reads like.
	Actions []Action
}

// Action is one Server Action: a function a form calls, with no URL of its own.
// Here it becomes a write handler and a form that posts to it.
type Action struct {
	Path string // the file it is declared in, relative to the project root
	Name string // the exported function
	Line int    // where the declaration starts
	// Screens are the URLs of the screens that import it. It is filled on
	// Project.Actions, where the same action of two screens is one line.
	Screens []string
}

// Endpoint is one address the screen calls, with the method it calls it with.
type Endpoint struct {
	Method string
	Path   string
}

// The classes, in the order the report explains them.
const (
	ClassForm   = "A" // form and list: no island signal, no polling
	ClassIsland = "B" // small island: polling, modal, tabs, upload, playing audio
	ClassSPA    = "C" // pointer, drawing surface, editor or capturing media
)

// hookNames are counted because the count is the size of the job: a screen
// with two useState is a form, one with fourteen is an application.
var hookNames = []string{"useState", "useEffect", "useRef", "useMemo", "useCallback", "useContext"}

// signal is a name and what has to be in the file for it to count. Some
// signals need two things at once: onDrop is a drop area when the file also
// mentions a file, and something being dragged when it does not.
type signal struct {
	name string
	re   *regexp.Regexp
	and  *regexp.Regexp // optional: must match too
}

// spaSignals are what cannot become a form: the browser is doing the work.
var spaSignals = []signal{
	// Dropping is not pointing. onDrop and onDragOver on a screen that also
	// mentions a file are a drop area — ui.Dropzone, class B — and reading
	// them as pointer sent the upload screen to the hardest class there is.
	// Dragging something that is not a file (a row being reordered) stays.
	{name: "pointer", re: regexp.MustCompile(`onPointer(Down|Move|Up)|onMouseMove|onDragStart|draggable=`)},
	{name: "drawing", re: regexp.MustCompile(`<canvas\b|getContext\(`)},
	// An <svg> is a logo until something works on it. Reading the tag alone as
	// a drawing surface classified four screens of a real migration as C — a
	// form with a 22px icon in it — and C is the class that says "write an
	// island". What counts is the ref that points at the element, the wheel
	// and the frame loop that redraw it, or a library that draws for you;
	// a pointer handler and d3 already classify on their own, above and below.
	{name: "live svg", re: svgRe, and: svgWorkRe},
	{name: "editor", re: regexp.MustCompile(`contentEditable|execCommand|Slate|ProseMirror|Tiptap`)},
	{name: "chart", re: regexp.MustCompile(`\bd3[-.]|recharts|chart\.js|Chart\(`)},
	// Recording voice is the purest case of the browser doing the work there
	// is: a device permission, a stream, an object with a life of its own and a
	// Blob at the end. There is no PRG that replaces it, and a screen like that
	// came back A — a form — because none of the signals above mentions a
	// microphone. Capturing is C; playing is B, below: the difference is that
	// one needs permission and a live object, and the other is an element the
	// server can draw.
	{name: "media capture", re: regexp.MustCompile(`getUserMedia\(|new MediaRecorder\(|\bSpeechRecognition\b|\bwebkitSpeechRecognition\b|new AudioContext\(`)},
}

// islandSignals are what a form cannot do alone, but the kit can.
var islandSignals = []signal{
	{name: "polling", re: regexp.MustCompile(`setInterval\(|usePoll\(|refetchInterval|EventSource\(`)},
	{name: "modal", re: regexp.MustCompile(`<Dialog\b|<Modal\b|showModal\(|<AlertDialog\b`)},
	{name: "tabs", re: regexp.MustCompile(`<Tabs\b|role="tab"|<TabsTrigger\b`)},
	{name: "upload", re: regexp.MustCompile(`type="file"|new FormData\(|<Dropzone\b`)},
	{name: "storage", re: regexp.MustCompile(`localStorage|sessionStorage`)},
	{name: "drop area", re: dropRe, and: fileRe},
	{name: "raw html", re: regexp.MustCompile(`dangerouslySetInnerHTML`)},
	// Playing is not capturing: the element is one the server draws, and the
	// position and the controls around it are what the kit covers.
	{name: "media playback", re: regexp.MustCompile(`new Audio\(|<audio\b|<video\b|HTMLMediaElement`)},
}

var (
	useClientRe = regexp.MustCompile(`(?m)^\s*['"]use client['"]`)
	// The three quote characters are three branches because Go's regexp has
	// no backreference: a template literal may hold a quote, and
	// `/api/users/${params["user-id"]}` is exactly the call worth reading.
	// The generic is optional and it is not rare: apiGet<DocsResponse>("/x")
	// is how a typed helper is called, and without this the Calls column of a
	// whole screen comes back empty — which is the column that says which
	// endpoint of the generated client to use.
	callRe      = regexp.MustCompile("\\b(apiGet|apiPost|apiPut|apiPatch|apiDelete|fetch)\\s*(?:<[^>]*>)?\\s*\\(\\s*(?:\"([^\"]*)\"|'([^']*)'|`([^`]*)`)")
	methodRe    = regexp.MustCompile(`method\s*:\s*["'` + "`" + `](\w+)`)
	exportRe    = regexp.MustCompile(`(?m)^export\s+(?:async\s+)?function\s+([A-Z]+)\s*\(`)
	exportVarRe = regexp.MustCompile(`(?m)^export\s+const\s+([A-Z]+)\s*=`)
	rewriteRe   = regexp.MustCompile(`source\s*:\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `][^}]*?destination\s*:\s*["'` + "`" + `]([^"'` + "`" + `]+)`)
	interpRe    = regexp.MustCompile(`\$\{([^}]*)\}`)
	identRe     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	keyRe       = regexp.MustCompile(`\[\s*["']([^"']+)["']\s*\]\s*$`)
	// Dropping a file is an upload; dragging something that is not a file
	// is a pointer. The two together are what tell them apart.
	dropRe = regexp.MustCompile(`onDrop|onDragOver|onDragEnter|onDragLeave`)
	fileRe = regexp.MustCompile(`dataTransfer\.files|type="file"`)
	// An <svg> and what would be working on it. The ref is looked for inside
	// the opening tag, because a ref somewhere else in the file belongs to
	// something else — an <aside>, an input — and the whole point here is not
	// to call a logo a drawing.
	svgRe     = regexp.MustCompile(`<svg\b`)
	svgWorkRe = regexp.MustCompile(`<svg[^>]*\bref=|onPointer(Down|Move|Up)|onMouse(Down|Move|Up)|onWheel|requestAnimationFrame\(|\bkonva|\bpixi|\bfabric\.|\bthree\b`)
)

// analyze reads one source file and the ones it imports. deps is keyed by the
// path each source came from; it may be nil, and then only src is read.
//
// The imports are read because the page is thin and the component beside it is
// not: a fifty-line page.tsx that imports three hundred lines of pointer
// handling is the hardest screen in the app, and reading the page alone calls
// it the easiest.
//
// global is what the layouts reach, and it is read the other way round: a file
// in it is the frame around the screen, not the screen. Its signals belong to
// the application once — the report says so in its own section — and letting
// them decide a class is how every screen of an app came back C.
//
// self is the path of src, used where a finding has to name a file. Inside the
// classification the page is still the file with no name: a signal it carries
// itself prints without one, because the reason only names a file when the
// reason is somewhere else.
func analyze(self, src string, deps map[string]dep, global map[string]bool) Analysis {
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

	// The page is looked at first and named "", so a signal it carries itself
	// is printed without a file: the reason only names a file when the reason
	// is somewhere else.
	from := []sourceFile{{src: src}}
	for _, p := range sortedKeys(deps) {
		if global[p] {
			// The frame around the screen is not the screen. Its signals are
			// the application's work, once, and the report says so in a
			// section of its own.
			a.Reaches = append(a.Reaches, p)
			continue
		}
		from = append(from, sourceFile{path: p, src: deps[p].src, whole: deps[p].whole})
		a.Deps = append(a.Deps, p)
		// The size is the file, not the part of it that counts for the class:
		// it is the file somebody is going to open. Reading a module by name is
		// about whose work the behaviour is, not about how long the module is.
		a.DepLines += strings.Count(deps[p].src, "\n") + 1
	}
	a.Actions = actionsOf(self, from)
	spa, island := scan(spaSignals, from), scan(islandSignals, from)

	a.Signals = append(append([]string{}, names(spa)...), names(island)...)
	switch {
	case len(spa) > 0:
		a.Class, a.Why = ClassSPA, why(spa)
	case len(island) > 0:
		a.Class, a.Why = ClassIsland, why(island)
	default:
		a.Class, a.Why = ClassForm, "no island signal"
		if len(a.Reaches) > 0 {
			// Saying "of its own" is the whole finding: the screen has a chat
			// around it, and the chat is not this screen's work.
			a.Why = "no island signal of its own"
		}
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

// sourceFile is one file the classification reads: where it came from — "" for
// the page itself — its text as the page uses it, and whether that text is the
// whole module or only the declarations the page asked for.
type sourceFile struct {
	path  string
	src   string
	whole bool
}

// actionStrings renders the Server Actions of one screen for the generated
// comment: the name, and the file it is declared in.
func actionStrings(as []Action) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name+" ("+a.Path+":"+strconv.Itoa(a.Line)+")")
	}
	return out
}

// hit is one signal that matched: what it was, which file it was in — "" when
// the page itself carried it — and the line, so the reason in the report can be
// thrown out without opening the file. A decorative <svg> read as a drawing
// surface is the case that made the line worth printing.
type hit struct {
	name, path string
	line       int
	whole      bool // the module was read whole: nobody said which names came in
}

// scan looks for each signal in the page and then in what it imports, keeping
// the first place it found each one: a signal the page carries is the page's,
// and one it does not is named after the file that does.
func scan(signals []signal, from []sourceFile) []hit {
	var out []hit
	for _, s := range signals {
		for _, f := range from {
			loc := s.re.FindStringIndex(f.src)
			if loc == nil || (s.and != nil && !s.and.MatchString(f.src)) {
				continue
			}
			out = append(out, hit{
				name:  s.name,
				path:  f.path,
				line:  1 + strings.Count(f.src[:loc[0]], "\n"),
				whole: f.path != "" && f.whole,
			})
			break
		}
	}
	return out
}

// useServerRe is the directive that makes a function a Server Action: at the
// top of a file it says every export of it is one, and in the body of a
// function it says that one is.
var useServerRe = regexp.MustCompile(`(?m)^\s*['"]use server['"]`)

// actionsOf collects the Server Actions of the page and of what it imports, in
// the order the files were read. It is a section of its own in the report
// because a screen with no island signal and four actions is not the "nothing
// to do" that an A on its own reads like: those four are a write handler, a
// contract and a form each, and nothing in the report used to say so.
//
// The directive decides the scope, the way Next defines it: at the top of the
// file every exported function is an action; inside a function body, only that
// one. Reading each module by the names the page imported (#167) is what keeps
// this from listing the fifty-three actions of a module the screen took two
// functions from.
func actionsOf(self string, from []sourceFile) []Action {
	var out []Action
	for _, f := range from {
		path := f.path
		if path == "" {
			path = self
		}
		ds := declsOf(f.src)
		loc := useServerRe.FindStringIndex(f.src)
		if loc == nil {
			continue
		}
		// The file-level directive is the first statement of the module, so it
		// sits before every declaration in it.
		file := len(ds) == 0 || loc[0] < ds[0].start
		for _, d := range ds {
			switch {
			case file && !d.exported:
				continue
			case !file && !useServerRe.MatchString(f.src[d.start:d.end]):
				continue
			}
			out = append(out, Action{
				Path: path,
				Name: d.name,
				Line: 1 + strings.Count(f.src[:d.start], "\n"),
			})
		}
	}
	return out
}

func names(hits []hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.name)
	}
	return out
}

// why is the clause the report prints. A signal found in an imported file says
// which one, because that is where the work is and the reader is about to go
// looking for it.
func why(hits []hit) string {
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		if h.path == "" {
			parts = append(parts, h.name+" (line "+strconv.Itoa(h.line)+")")
			continue
		}
		where := path.Base(path.Dir(h.path)) + "/" + path.Base(h.path) + ":" + strconv.Itoa(h.line)
		// A module nobody named — a default or a namespace import — counts
		// whole, and the reason says so: otherwise the line printed is a line
		// of a file the screen may never call into.
		if h.whole {
			where += ", whole module"
		}
		parts = append(parts, h.name+" ("+where+")")
	}
	return strings.Join(parts, " and ")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
