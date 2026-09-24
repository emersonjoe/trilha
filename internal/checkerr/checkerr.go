// Package checkerr is the single source of the stable error codes: what each
// code means, the sentence that resolves it, and a small example. The gate
// attaches these to every failure it prints, the docs pages are generated
// from here, and the MCP server answers get_error with the same table — one
// catalog, so a repair shown anywhere cannot drift from the repair shown
// everywhere else.
package checkerr

import (
	"sort"
	"strings"

	"github.com/emersonjoe/trilha"
)

// Doc is one entry of the catalog. Example is a small, compilable snippet —
// a line or two, enough to show the shape of the fix.
type Doc struct {
	Code    string // "E_DUPLICATE_ROUTE"
	Title   string // one line, as a page heading
	Cause   string // why the gate failed, in one sentence
	Fix     string // the sentence that resolves it
	Example string // Go, showing the shape of the fix
	Doc     string // the site page: "/docs/errors/<code>"
}

// Docs returns the whole catalog in code order.
func Docs() []Doc {
	out := append([]Doc(nil), catalog...)
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// ByCode finds one entry. Codes are case-insensitive because they are copied
// from logs into a search box; a family code (E_VULN_GO-2026-0001) falls back
// to its prefix's entry, which is where the upgrade rule lives.
func ByCode(code string) (Doc, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if d, ok := exact[code]; ok {
		return d, true
	}
	for prefix, d := range families {
		if strings.HasPrefix(code, prefix) {
			return d, true
		}
	}
	return Doc{}, false
}

// Families lists the prefix codes whose entries answer every code that starts
// with them, longest prefix first.
func Families() []string {
	out := make([]string, 0, len(families))
	for f := range families {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

var exact map[string]Doc
var families map[string]Doc

func init() {
	exact = map[string]Doc{}
	families = map[string]Doc{}
	for _, d := range catalog {
		if strings.HasSuffix(d.Code, "_") {
			families[d.Code] = d
		} else {
			exact[d.Code] = d
		}
	}
	// The runtime hints already have a catalog of their own; it is the same
	// table, so it joins this one instead of living beside it.
	for _, g := range trilha.ErrorGuides() {
		exact[g.Code] = Doc{
			Code:    g.Code,
			Title:   g.Title,
			Cause:   g.Description,
			Fix:     g.Repair,
			Example: "",
			Doc:     "/docs/errors/" + g.Code,
		}
	}
}

// catalog is the table, in reading order: the gates first (what check emits),
// then the scanner's conventions, then the runtime's.
var catalog = []Doc{
	// ---- the gates ----
	{"E_GEN_STALE", "trilha_gen.go is out of date", "The routes in app/ moved and the generated file was not regenerated, so a route can 404 with no one explaining why.", "Run `trilha gen` (or `trilha check --fix`) and commit the file — it is generated and committed, and `go build` must work without the CLI.", "trilha gen", "/docs/errors/E_GEN_STALE"},
	{"E_GOFMT", "File is not gofmt'd", "gofmt found formatting that differs from the standard.", "Run `gofmt -w .` (or `trilha check --fix`) and commit the formatting with the change that needed it.", "gofmt -w app/blog/page.go", "/docs/errors/E_GOFMT"},
	{"E_VET", "go vet reported a problem", "go vet found a suspicious construct — a lock copied by value, an unreachable branch, a format string's cousin.", "Fix what go vet reports at the file and line it names; the message is the diagnosis.", "", "/docs/errors/E_VET"},
	{"E_VET_PRINTF", "Printf format and arguments disagree", "The printf analyzer found a format verb that does not match its argument — a %d handed a string, or a value with no verb at all.", "Match the verb to the argument, or drop the argument from the format string.", "fmt.Sprintf(\"id=%s\", id) // %s for strings, %d for numbers", "/docs/errors/E_VET_PRINTF"},
	{"E_TEST", "A test failed", "The suite ran and at least one test did not pass — the file and line in the failure are where it said so.", "Run `go test ./...`, read the failing test's message, and fix the code the test was holding in place — or, when the test is wrong, fix the test and say why in the commit.", "", "/docs/errors/E_TEST"},
	{"E_AUDIT", "A critical audit rule failed", "trilha audit found a configuration that is unsafe to ship — a secret in the source, a cleartext provider URL.", "Follow the audit hint for this finding; the rule exists because the configuration is the door.", "", "/docs/errors/E_AUDIT"},
	{"E_VULN_", "A dependency has a known vulnerability", "govulncheck found a vulnerability that this project's code actually reaches.", "Upgrade the module the advisory names to at least the fixed version: `go get module@version && go mod tidy`.", "go get github.com/some/module@v1.2.3", "/docs/errors/E_VULN_"},
	{"E_API_SURFACE", "The public API surface changed", "api/current.txt does not match the exported symbols the guarded packages declare — something was added, changed or removed without the file following.", "Run `make api` and commit api/current.txt with the change. Additions are welcome; removals and renames need a deprecation cycle.", "make api", "/docs/errors/E_API_SURFACE"},
	{"E_OPENAPI", "The OpenAPI document is stale", "The openapi.json the app serves no longer matches the routes the scanner found.", "Run `trilha openapi` (with `-o` pointing at the document the app embeds) and commit it.", "trilha openapi -o app/mcp/openapi.json", "/docs/errors/E_OPENAPI"},
	{"E_I18N", "A translation is missing keys", "A locale's catalog lacks keys the default locale uses, so its readers fall back to another language.", "Run `trilha i18n missing <locale>` and add the keys — or accept the fallback on purpose and say so.", "trilha i18n missing pt-BR", "/docs/errors/E_I18N"},
	{"E_CTX_BUDGET", "The context pack exceeded its budget", "trilha ctx had to cut sections to fit the requested token budget.", "Raise --budget to keep the sections, or drop --strict to accept the cut (the answer names what went).", "trilha ctx --pack app --budget 4000", "/docs/errors/E_CTX_BUDGET"},

	// ---- the scanner's conventions ----
	{"E_PAGE_AND_ROUTE", "A folder has page.go and route.go", "The folder answers both as a page and as an API, and the router cannot serve two answers for one URL.", "Split the folder: the page in one, the API under an api/ branch — or move the write into the page's POST handler.", "", "/docs/errors/E_PAGE_AND_ROUTE"},
	{"E_NO_PAGE_FUNC", "page.go has no Page function", "The scanner found page.go but no `func Page(c *trilha.Ctx) (h.Node, error)` in it, so the route has nothing to answer with.", "Name the handler Page, or rename the file if it is not a page.", "func Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }", "/docs/errors/E_NO_PAGE_FUNC"},
	{"E_NO_METHOD", "route.go has no exported HTTP method", "route.go declares no GET, POST, PUT, PATCH or DELETE, so the folder is a URL that answers nothing.", "Export at least one method handler with the exact name of the method.", "func GET(c *trilha.Ctx) error { return c.JSON(200, data) }", "/docs/errors/E_NO_METHOD"},
	{"E_NO_LAYOUT_FUNC", "layout.go has no Layout function", "The scanner found layout.go but no Layout function, so the tree's wrapper is missing.", "Name the handler Layout with the (c *trilha.Ctx, children h.Node) signature.", "func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) { return h.Html(children), nil }", "/docs/errors/E_NO_LAYOUT_FUNC"},
	{"E_NO_MIDDLEWARE_FUNC", "middleware.go has no Middleware function", "The scanner found middleware.go but no Middleware function, so nothing runs before the subtree.", "Name the handler Middleware and call next.", "func Middleware(c *trilha.Ctx, next trilha.Next) error { return next() }", "/docs/errors/E_NO_MIDDLEWARE_FUNC"},
	{"E_NO_NOT_FOUND_FUNC", "not_found.go has no NotFound function", "The custom 404 file declares no NotFound handler.", "Name the handler NotFound.", "func NotFound(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }", "/docs/errors/E_NO_NOT_FOUND_FUNC"},
	{"E_NO_ERROR_FUNC", "error.go has no Error function", "The custom error page declares no Error handler.", "Name the handler Error with the (c *trilha.Ctx, err error) signature.", "func Error(c *trilha.Ctx, err error) (h.Node, error) { return h.Div(), nil }", "/docs/errors/E_NO_ERROR_FUNC"},
	{"E_NO_SETUP_FUNC", "setup.go has no Setup function", "The file named setup.go declares no Setup, so the bootstrapping it holds never runs.", "Export Setup(a *trilha.App) error — or move the code where it belongs.", "", "/docs/errors/E_NO_SETUP_FUNC"},
	{"E_UNUSED_METHOD_MIDDLEWARE", "Middleware<Method> guards no method", "The folder declares a per-method middleware for a method the routes below never export.", "Delete the middleware, or add the method it was written to guard.", "", "/docs/errors/E_UNUSED_METHOD_MIDDLEWARE"},
	{"E_AMBIGUOUS_SEGMENT", "Two dynamic names for the same segment", "One level of the URL is named slug_ in one route and id_ in another, and the router cannot be both.", "Use the same parameter name for the same segment, or split the routes.", "", "/docs/errors/E_AMBIGUOUS_SEGMENT"},
	{"E_CATCHALL_NOT_LEAF", "A catch-all is not the last segment", "A `name__` folder has children, but a catch-all swallows everything below it — children would be unreachable.", "Move the catch-all to a leaf, or give the children their own branch beside it.", "", "/docs/errors/E_CATCHALL_NOT_LEAF"},
	{"E_BAD_SEGMENT", "The folder name is not a valid segment", "A folder under app/ uses a name Go cannot import — brackets, spaces, or a parameter suffix in the wrong place.", "Rename the folder: lowercase, hyphens, `name_` for a parameter, `name__` for a catch-all, `name-` for a group.", "", "/docs/errors/E_BAD_SEGMENT"},
	{"E_DUPLICATE_ROUTE", "Two files serve the same pattern", "Two routes answer the same method and path, and only one of them can win — the second is a bug waiting for traffic.", "Rename one of the directories, or drop the route group so the two URLs differ.", "", "/docs/errors/E_DUPLICATE_ROUTE"},
	{"E_DUPLICATE_PARAM", "The same parameter twice in one pattern", "A pattern uses a parameter name two times, so the router cannot tell which segment a value belongs to.", "Give each parameter in the pattern its own name.", "", "/docs/errors/E_DUPLICATE_PARAM"},
	{"E_HIDDEN_ROUTE", "A route is hidden by an earlier pattern", "A more general pattern registered first answers what this one was written for, so this route never runs.", "Move the more specific route deeper, or make the general one narrower.", "", "/docs/errors/E_HIDDEN_ROUTE"},
	{"E_UNROUTABLE_METHOD", "The method name is not routable", "HEAD, TRACE and CONNECT are not taken from files — HEAD is answered by GET, and the others are not for apps.", "Write the GET handler; it answers HEAD too.", "", "/docs/errors/E_UNROUTABLE_METHOD"},
	{"E_CORS_ON_PAGE", "CORS on a page", "A page declares CORS, but CORS is for APIs: a browser never fetches an HTML page from another origin.", "Move the handler to a route.go, or drop the policy if the page is same-origin.", "", "/docs/errors/E_CORS_ON_PAGE"},
	{"E_OFFLINE_ON_API", "Offline handling on an API", "An API route declares the offline fallback, which answers with the cached page — an API must answer with its status.", "Drop `var Offline = true` from the route, or move the route into a page folder.", "", "/docs/errors/E_OFFLINE_ON_API"},
	{"E_PARSE", "The file could not be parsed", "A Go file under app/ has a syntax error, so the scanner stopped.", "Fix the syntax error at the file and line given.", "", "/docs/errors/E_PARSE"},
	{"E_NO_APP", "No app/ directory", "The command ran where there is no project: no app/ folder, no routes to read.", "Run at the project root (the folder with go.mod and app/), or create the project with `trilha new`.", "trilha new agenda && cd agenda", "/docs/errors/E_NO_APP"},
}
