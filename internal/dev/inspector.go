package dev

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/scan"
)

// InspectorPath is where trilha dev serves the route inspector. It lives in
// the supervisor, not in the app: a page that does not exist in the binary
// cannot be turned on in production by mistake.
const InspectorPath = "/_trilha/routes"

// inspectorRoute is one line of the table.
type inspectorRoute struct {
	Pattern     string
	Kind        string
	Methods     string
	Source      string
	Layouts     []string
	Middlewares []string
}

type paramValue struct{ Name, Value string }

// match is the answer to "who answers this path".
type match struct {
	Path    string
	Pattern string
	Params  []paramValue
}

type inspectorData struct {
	Module   string
	Routes   []inspectorRoute
	Asked    string
	Match    *match
	Layout   string
	NotFound string
	Error    string
	Setup    string
}

// renderInspector scans the project and renders the page. The scan happens per
// request: in dev the tree changes under the browser, and a cached table would
// be answering about the app of a minute ago.
func renderInspector(root, module, path string) ([]byte, error) {
	res, err := scan.Scan(root, module)
	if err != nil {
		return nil, err
	}
	data := inspectorData{
		Module:   res.Module,
		Asked:    path,
		Layout:   refFile(res.Module, res.RootLayout, "layout.go"),
		NotFound: refFile(res.Module, res.NotFound, "not_found.go"),
		Error:    refFile(res.Module, res.ErrorPage, "error.go"),
		Setup:    refFile(res.Module, res.Setup, "setup.go"),
	}
	for _, r := range sortByPrecedence(res.Routes) {
		file := "route.go"
		if r.Kind == "page" {
			file = "page.go"
		}
		methods := r.Methods
		if r.HasPage {
			methods = append([]string{"GET"}, methods...)
		}
		line := inspectorRoute{
			Pattern: r.Pattern,
			Kind:    r.Kind,
			Methods: strings.Join(methods, ", "),
			Source:  r.Dir + "/" + file,
		}
		// Outermost first is how a request meets them, which is the order the
		// question is usually asked in.
		for i := len(r.Layouts) - 1; i >= 0; i-- {
			line.Layouts = append(line.Layouts, refFile(res.Module, &r.Layouts[i], "layout.go"))
		}
		for i := range r.Middlewares {
			line.Middlewares = append(line.Middlewares, refFile(res.Module, &r.Middlewares[i], "middleware.go"))
		}
		// A chain that guards a single method is listed under that method, so
		// the panel answers "what runs on POST?" without reading the files.
		for _, m := range methods {
			chain := r.MiddlewaresByMethod[m]
			for i := range chain {
				line.Middlewares = append(line.Middlewares, m+": "+refFile(res.Module, &chain[i], "middleware.go"))
			}
		}
		data.Routes = append(data.Routes, line)
	}
	if path != "" {
		m := matchPath(res.Routes, path)
		m.Path = path
		data.Match = &m
	}
	var buf bytes.Buffer
	if err := inspectorTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// refFile turns a scanned reference into the file a person can open.
func refFile(module string, r *scan.Ref, file string) string {
	if r == nil {
		return ""
	}
	dir := strings.TrimPrefix(strings.TrimPrefix(r.ImportPath, module), "/")
	if dir == "" {
		return file
	}
	return dir + "/" + file
}

// matchPath answers with the pattern that would serve path, resolved by the
// same http.ServeMux the app uses — a second implementation of the precedence
// rules would be a second set of bugs.
func matchPath(routes []scan.Route, path string) match {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	mux := http.NewServeMux()
	var m match
	for _, r := range routes {
		pattern, names := r.Pattern, paramNames(r.Pattern)
		if pattern == "/" {
			pattern = "/{$}"
		}
		mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, req *http.Request) {
			m.Pattern = r.Pattern
			for _, n := range names {
				m.Params = append(m.Params, paramValue{Name: n, Value: req.PathValue(n)})
			}
		})
	}
	req := &http.Request{Method: "GET", Host: "localhost", URL: &url.URL{Path: path}}
	mux.ServeHTTP(nullWriter{}, req)
	return m
}

// nullWriter lets the mux resolve a pattern without answering anybody: the
// handlers registered above only record which one won.
type nullWriter struct{}

func (nullWriter) Header() http.Header         { return http.Header{} }
func (nullWriter) Write(b []byte) (int, error) { return len(b), nil }
func (nullWriter) WriteHeader(int)             {}

func paramNames(pattern string) []string {
	var names []string
	for _, seg := range strings.Split(pattern, "/") {
		if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
			continue
		}
		names = append(names, strings.TrimSuffix(strings.Trim(seg, "{}"), "..."))
	}
	return names
}

// sortByPrecedence puts the pattern that wins above the one it shadows, so the
// table reads like the router decides: literal, then parameter, then catch-all.
func sortByPrecedence(routes []scan.Route) []scan.Route {
	out := append([]scan.Route(nil), routes...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.Split(out[i].Pattern, "/"), strings.Split(out[j].Pattern, "/")
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] == b[k] {
				continue
			}
			if sa, sb := specificity(a[k]), specificity(b[k]); sa != sb {
				return sa > sb
			}
			return a[k] < b[k]
		}
		return len(a) < len(b)
	})
	return out
}

func specificity(seg string) int {
	switch {
	case strings.HasSuffix(seg, "...}"):
		return 0
	case strings.HasPrefix(seg, "{"):
		return 1
	default:
		return 2
	}
}

// serveInspector is the handler trilha dev mounts on InspectorPath.
func (s *Server) serveInspector(w http.ResponseWriter, r *http.Request) {
	body, err := renderInspector(s.Root, s.Module, r.URL.Query().Get("path"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(inspectorError(err)))
		return
	}
	_, _ = w.Write(body)
}

func inspectorError(err error) string {
	var buf bytes.Buffer
	_ = template.Must(template.New("e").Parse(
		`<!doctype html><meta charset=utf-8><title>trilha routes</title><h1>scan failed</h1><pre>{{.}}</pre>`,
	)).Execute(&buf, err.Error())
	return buf.String()
}

var inspectorTemplate = template.Must(template.New("inspector").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>trilha routes</title>
<style>
:root{color-scheme:light dark}
body{margin:0;padding:2rem;font:14px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}
h1{font-size:1.1rem;margin:0 0 .25rem}
p.mod{margin:0 0 1.5rem;opacity:.6}
form{margin:0 0 1.5rem;display:flex;gap:.5rem;flex-wrap:wrap}
input{flex:1;min-width:14rem;padding:.4rem .6rem;font:inherit;border:1px solid;border-radius:.3rem;background:transparent;color:inherit}
button{padding:.4rem .9rem;font:inherit;border:1px solid;border-radius:.3rem;background:transparent;color:inherit;cursor:pointer}
.answer{margin:0 0 1.5rem;padding:.75rem 1rem;border-left:3px solid;opacity:.95}
table{border-collapse:collapse;width:100%}
th,td{text-align:left;vertical-align:top;padding:.4rem .8rem .4rem 0;border-bottom:1px solid rgba(128,128,128,.3)}
th{font-weight:600;opacity:.6;font-size:.85em;text-transform:uppercase;letter-spacing:.04em}
td.dim{opacity:.6}
ul{margin:0;padding-left:1rem}
footer{margin-top:2rem;opacity:.6}
</style></head><body>
<h1>Routes</h1>
<p class="mod">{{.Module}} · this page is served by <code>trilha dev</code> and does not exist in the built binary</p>

<form method="get" action="` + InspectorPath + `">
  <input name="path" value="{{.Asked}}" placeholder="/blog/hello" aria-label="path">
  <button type="submit">Who answers this?</button>
</form>
{{with .Match}}
<p class="answer">
{{if .Pattern}}<strong>{{.Path}}</strong> → <code>{{.Pattern}}</code>
{{range .Params}}<br>{{.Name}} = <code>{{.Value}}</code>{{end}}
{{else}}<strong>{{.Path}}</strong> → nothing matches; the app answers 404.{{end}}
</p>
{{end}}

<table>
<tr><th>Pattern</th><th>Kind</th><th>Methods</th><th>Source</th><th>Layouts</th><th>Middlewares</th></tr>
{{range .Routes}}<tr>
<td><code>{{.Pattern}}</code></td>
<td class="dim">{{.Kind}}</td>
<td class="dim">{{.Methods}}</td>
<td class="dim">{{.Source}}</td>
<td class="dim">{{if .Layouts}}<ul>{{range .Layouts}}<li>{{.}}</li>{{end}}</ul>{{else}}—{{end}}</td>
<td class="dim">{{if .Middlewares}}<ul>{{range .Middlewares}}<li>{{.}}</li>{{end}}</ul>{{else}}—{{end}}</td>
</tr>{{end}}
</table>

<footer>
{{if .Layout}}root layout: {{.Layout}}<br>{{end}}
{{if .NotFound}}404: {{.NotFound}}<br>{{end}}
{{if .Error}}error page: {{.Error}}<br>{{end}}
{{if .Setup}}setup: {{.Setup}}{{end}}
</footer>
</body></html>
`))
