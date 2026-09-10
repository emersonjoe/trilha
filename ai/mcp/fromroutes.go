package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
)

// FromRoutesOpts says which routes become tools and what to call them.
type FromRoutesOpts struct {
	// Name and Version are what the server says about itself on initialize.
	Name, Version string
	// Include is the routes to expose, by pattern; a pattern ending in "*"
	// takes the subtree ("/api/v1/*"). Empty is "/api/*". Only API routes
	// enter — Kind API, or a route.go that said nothing — so a page under
	// the prefix never becomes a tool. Exclude takes away from Include.
	Include, Exclude []string
	// OpenAPI is the document `trilha openapi` writes, embedded by the
	// application. It gives the tools their names (operationId), their
	// descriptions (summary) and their input schemas (path, query and body,
	// flattened). Without it the tools still exist, named the same way,
	// described by method and path, and with only the path parameters in
	// the schema. A document that does not parse is a panic at wiring time:
	// the file is in the repository, and a server publishing tools with no
	// contract is not what anyone embedded it for.
	OpenAPI []byte
}

// FromRoutes exposes the API routes of app as MCP tools, each call carrying
// the caller's key. The call never touches the network: the tool builds a
// request in process — the Authorization header, the Host and the address of
// whoever called the MCP server go on it — and hands it to app.Handler(), so
// the route's whole chain runs as it would for any client: Keys.Require,
// RequirePolicy, the rate limit, the audit trail. What the trail records is
// Via "mcp" with the key's subject.
//
// tools/list is per caller: before listing, each route is probed with the
// same header (App.Probe), and a route the chain refuses is not listed and
// cannot be called by name. A 2xx becomes the tool's text; a 4xx or 5xx
// becomes isError with the body — the Problem — as the text.
//
//	// app/setup.go
//	//go:embed mcp/openapi.json
//	var openAPI []byte
//
//	trilha.Provide(a, mcp.FromRoutes(a, mcp.FromRoutesOpts{
//		Name: "acervo", Version: "1.0", Include: []string{"/api/v1/*"}, OpenAPI: openAPI,
//	}))
//
//	// app/mcp/route.go
//	func POST(c *trilha.Ctx) error { return trilha.Use[*mcp.Server](c).ServeHTTP(c) }
//
// A route whose body is multipart/form-data stays out — the bridge sends
// JSON — and the app's log says which one and why.
func FromRoutes(app *trilha.App, o FromRoutesOpts) *Server {
	doc, err := parseOpenAPI(o.OpenAPI)
	if err != nil {
		panic("mcp.FromRoutes: " + err.Error())
	}
	b := &bridge{app: app}
	byTool := map[*ai.Tool]routeTool{}
	s := NewServer(o.Name, o.Version)
	// The table is built on the first message, not here: Setup, where this
	// is called, runs before trilha_gen.go registers a single route.
	s.build = func() []*ai.Tool {
		routes := map[string][]string{}
		for pattern := range app.Routes() {
			r, ok := app.Route(pattern)
			if !ok || !isAPI(r) {
				continue
			}
			var ms []string
			for m := range r.Methods {
				ms = append(ms, m)
			}
			routes[pattern] = ms
		}
		plan, warnings := plan(doc, routes, o)
		for _, w := range warnings {
			app.Logger().Warn("mcp: route left out", "reason", w)
		}
		tools := make([]*ai.Tool, 0, len(plan))
		for _, rt := range plan {
			rt := rt
			t := ai.NewTool(rt.name, rt.description, rt.schema, func(ctx context.Context, args json.RawMessage) (string, error) {
				return b.call(ctx, rt, args)
			})
			tools = append(tools, t)
			byTool[t] = rt
		}
		return tools
	}
	s.visible = func(ctx context.Context, t *ai.Tool) bool {
		return b.allowed(ctx, byTool[t])
	}
	return s
}

// Preview is the list FromRoutes would publish, derived from the document
// and the routes alone — what `trilha mcp --from-routes` prints for a person
// to check before the server exists. routes maps each API pattern to its
// methods. The warnings are the operations left out, with the reason.
func Preview(openAPI []byte, routes map[string][]string, o FromRoutesOpts) ([]ToolInfo, []string, error) {
	doc, err := parseOpenAPI(openAPI)
	if err != nil {
		return nil, nil, err
	}
	plan, warnings := plan(doc, routes, o)
	infos := make([]ToolInfo, 0, len(plan))
	for _, rt := range plan {
		infos = append(infos, ToolInfo{Name: rt.name, Description: rt.description, InputSchema: rt.schema})
	}
	return infos, warnings, nil
}

func isAPI(r trilha.Route) bool {
	switch r.Kind {
	case trilha.KindAPI:
		return true
	case trilha.KindPage:
		return false
	}
	return r.Page == nil
}

// routeTool is one (route, method) as a tool: what to call it and where each
// argument goes when the request is built.
type routeTool struct {
	name, description string
	method, pattern   string
	schema            json.RawMessage
	path              []string        // the path parameters, in order
	query             map[string]bool // arguments sent as query string
	body              string          // the argument that is the whole body, when it is not an object
	// Everything else goes to the query on GET and DELETE, and forms the
	// JSON body on the other methods.
}

var skipMethods = map[string]bool{"OPTIONS": true, "HEAD": true}

// plan decides the tools: the routes that match, one per method, described
// by the document when there is one.
func plan(doc *openAPIDoc, routes map[string][]string, o FromRoutesOpts) ([]routeTool, []string) {
	var out []routeTool
	var warnings []string
	patterns := make([]string, 0, len(routes))
	for p := range routes {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)
	for _, pattern := range patterns {
		if !matchAny(o.Include, pattern) || (len(o.Exclude) > 0 && matchAny(o.Exclude, pattern)) {
			continue
		}
		methods := append([]string(nil), routes[pattern]...)
		sort.Strings(methods)
		var item *docPath
		if doc != nil {
			item = doc.Paths[strings.ReplaceAll(pattern, "...}", "}")]
		}
		for _, m := range methods {
			if skipMethods[m] {
				continue
			}
			rt := routeTool{name: operationID(m, pattern), method: m, pattern: pattern, path: pathParams(pattern), query: map[string]bool{}}
			var op *docOp
			if item != nil {
				op = item.op(m)
			}
			if op == nil {
				rt.description = m + " " + pattern
				rt.schema = bareSchema(rt.path)
				out = append(out, rt)
				continue
			}
			if op.OperationID != "" {
				rt.name = op.OperationID
			}
			rt.description = strings.TrimSpace(op.Summary)
			if d := strings.TrimSpace(op.Description); d != "" {
				if rt.description != "" {
					rt.description += "\n\n"
				}
				rt.description += d
			}
			if rt.description == "" {
				rt.description = m + " " + pattern
			}
			if media := op.bodyMedia(); media != "" && media != "application/json" {
				warnings = append(warnings, fmt.Sprintf("%s (%s %s): %s body; the MCP bridge sends JSON", rt.name, m, pattern, media))
				continue
			}
			schema, query, body := docSchema(doc, item, op, rt.path)
			rt.schema, rt.query, rt.body = schema, query, body
			out = append(out, rt)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, warnings
}

// matchAny is Include/Exclude: exact, or the subtree under a trailing "*".
func matchAny(pats []string, path string) bool {
	if len(pats) == 0 {
		pats = []string{"/api/*"}
	}
	for _, p := range pats {
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(strings.TrimSuffix(p, "*"), "/")
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
			continue
		}
		if path == p {
			return true
		}
	}
	return false
}

func pathParams(pattern string) []string {
	var ps []string
	for _, seg := range strings.Split(pattern, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			ps = append(ps, strings.TrimSuffix(strings.Trim(seg, "{}"), "..."))
		}
	}
	return ps
}

// operationID is the same rule trilha openapi uses, so a tool has the name
// the document gives the operation whether or not the document is there.
func operationID(method, pattern string) string {
	var sb strings.Builder
	sb.WriteString(strings.ToLower(method))
	for _, seg := range strings.Split(pattern, "/") {
		seg = strings.TrimSuffix(strings.Trim(seg, "{}"), "...")
		upper := true
		for _, r := range seg {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				upper = true
				continue
			}
			if upper {
				sb.WriteRune(unicode.ToUpper(r))
				upper = false
				continue
			}
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// bareSchema is what is known without a document: the path parameters, and
// room for whatever else the route reads.
func bareSchema(path []string) json.RawMessage {
	props := map[string]any{}
	for _, p := range path {
		props[p] = map[string]any{"type": "string"}
	}
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": true}
	if len(path) > 0 {
		s["required"] = path
	}
	return mustJSON(s)
}

// --- the document ----------------------------------------------------------

type openAPIDoc struct {
	Paths      map[string]*docPath `json:"paths"`
	Components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	} `json:"components"`
}

type docPath struct {
	Parameters []docParam `json:"parameters"`
	Get        *docOp     `json:"get"`
	Post       *docOp     `json:"post"`
	Put        *docOp     `json:"put"`
	Patch      *docOp     `json:"patch"`
	Delete     *docOp     `json:"delete"`
}

func (p *docPath) op(method string) *docOp {
	switch method {
	case "GET":
		return p.Get
	case "POST":
		return p.Post
	case "PUT":
		return p.Put
	case "PATCH":
		return p.Patch
	case "DELETE":
		return p.Delete
	}
	return nil
}

type docOp struct {
	Summary     string     `json:"summary"`
	Description string     `json:"description"`
	OperationID string     `json:"operationId"`
	Parameters  []docParam `json:"parameters"`
	RequestBody *struct {
		Required bool `json:"required"`
		Content  map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

// bodyMedia is the one media type of the request body, JSON preferred.
func (op *docOp) bodyMedia() string {
	if op.RequestBody == nil || len(op.RequestBody.Content) == 0 {
		return ""
	}
	if _, ok := op.RequestBody.Content["application/json"]; ok {
		return "application/json"
	}
	medias := make([]string, 0, len(op.RequestBody.Content))
	for m := range op.RequestBody.Content {
		medias = append(medias, m)
	}
	sort.Strings(medias)
	return medias[0]
}

type docParam struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Description string          `json:"description"`
	Required    bool            `json:"required"`
	Schema      json.RawMessage `json:"schema"`
}

func parseOpenAPI(b []byte) (*openAPIDoc, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	var doc openAPIDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("openapi document: %w", err)
	}
	return &doc, nil
}

const componentsPrefix = "#/components/schemas/"

// docSchema flattens one operation into a single object schema: the path
// parameters, the query parameters and the properties of a JSON object body
// side by side, each required when the document says so. A body that is not
// an object becomes the "body" argument. Components the schema references
// travel inside it as $defs, because the tool's schema stands alone.
func docSchema(doc *openAPIDoc, item *docPath, op *docOp, path []string) (schema json.RawMessage, query map[string]bool, body string) {
	props := map[string]any{}
	required := map[string]bool{}
	query = map[string]bool{}
	params := append(append([]docParam(nil), item.Parameters...), op.Parameters...)
	seen := map[string]bool{}
	for _, p := range params {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		s := asMap(p.Schema)
		if s == nil {
			s = map[string]any{"type": "string"}
		}
		if p.Description != "" {
			s["description"] = p.Description
		}
		switch p.In {
		case "path":
			props[p.Name] = s
			required[p.Name] = true
		case "query":
			props[p.Name] = s
			query[p.Name] = true
			if p.Required {
				required[p.Name] = true
			}
		}
	}
	for _, p := range path {
		if _, ok := props[p]; !ok {
			props[p] = map[string]any{"type": "string"}
			required[p] = true
		}
	}
	if op.RequestBody != nil {
		if c, ok := op.RequestBody.Content["application/json"]; ok {
			bs := resolve(doc, asMap(c.Schema))
			if bs != nil && bs["type"] == "object" {
				if ps, ok := bs["properties"].(map[string]any); ok {
					for k, v := range ps {
						if _, taken := props[k]; !taken {
							props[k] = v
						}
					}
				}
				if rs, ok := bs["required"].([]any); ok {
					for _, r := range rs {
						if name, ok := r.(string); ok {
							required[name] = true
						}
					}
				}
			} else if bs != nil {
				body = "body"
				props["body"] = bs
				if op.RequestBody.Required {
					required["body"] = true
				}
			}
		}
	}
	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		names := make([]string, 0, len(required))
		for n := range required {
			names = append(names, n)
		}
		sort.Strings(names)
		out["required"] = names
	}
	defs := map[string]any{}
	rewriteRefs(doc, props, defs)
	if len(defs) > 0 {
		out["$defs"] = defs
	}
	return mustJSON(out), query, body
}

func asMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// resolve follows a top-level $ref to its component, so a body declared as
// the struct's component is still flattened.
func resolve(doc *openAPIDoc, s map[string]any) map[string]any {
	for i := 0; i < 8 && s != nil; i++ {
		ref, ok := s["$ref"].(string)
		if !ok || !strings.HasPrefix(ref, componentsPrefix) {
			return s
		}
		s = asMap(doc.Components.Schemas[strings.TrimPrefix(ref, componentsPrefix)])
	}
	return s
}

// rewriteRefs turns every "#/components/schemas/X" into "#/$defs/X" and
// collects X — and what X references — into defs.
func rewriteRefs(doc *openAPIDoc, v any, defs map[string]any) {
	switch x := v.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok && strings.HasPrefix(ref, componentsPrefix) {
			name := strings.TrimPrefix(ref, componentsPrefix)
			x["$ref"] = "#/$defs/" + name
			if _, done := defs[name]; !done {
				defs[name] = nil // reserve before descending: a recursive type ends here
				comp := asMap(doc.Components.Schemas[name])
				if comp == nil {
					comp = map[string]any{}
				}
				rewriteRefs(doc, comp, defs)
				defs[name] = comp
			}
		}
		for _, child := range x {
			rewriteRefs(doc, child, defs)
		}
	case []any:
		for _, child := range x {
			rewriteRefs(doc, child, defs)
		}
	}
}

// --- the call --------------------------------------------------------------

// callerKey carries the HTTP request that reached the MCP server, so a tool
// can build its own request in the caller's name.
type callerKey struct{}

func callerOf(ctx context.Context) *http.Request {
	r, _ := ctx.Value(callerKey{}).(*http.Request)
	return r
}

type bridge struct {
	app *trilha.App
}

// build is the in-process request for one call: the arguments placed where
// the route reads them, the caller's identity copied over.
func (b *bridge) build(ctx context.Context, rt routeTool, args json.RawMessage) (*http.Request, error) {
	in := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(args)) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, fmt.Errorf("arguments must be a JSON object: %w", err)
		}
	}
	path := rt.pattern
	for _, name := range rt.path {
		raw, ok := in[name]
		if !ok {
			return nil, errors.New("missing argument: " + name)
		}
		delete(in, name)
		val := scalar(raw)
		catchAll := strings.Contains(rt.pattern, "{"+name+"...}")
		if catchAll {
			segs := strings.Split(val, "/")
			for i, s := range segs {
				segs[i] = url.PathEscape(s)
			}
			path = strings.Replace(path, "{"+name+"...}", strings.Join(segs, "/"), 1)
		} else {
			path = strings.Replace(path, "{"+name+"}", url.PathEscape(val), 1)
		}
	}
	q := url.Values{}
	bodyObj := map[string]json.RawMessage{}
	var bodyRaw json.RawMessage
	noBody := rt.method == "GET" || rt.method == "DELETE"
	for name, raw := range in {
		switch {
		case rt.query[name] || (noBody && rt.body == ""):
			q.Set(name, scalar(raw))
		case rt.body != "" && name == rt.body:
			bodyRaw = raw
		default:
			bodyObj[name] = raw
		}
	}
	var body *bytes.Reader
	switch {
	case bodyRaw != nil:
		body = bytes.NewReader(bodyRaw)
	case len(bodyObj) > 0 || (!noBody && rt.body == ""):
		body = bytes.NewReader(mustJSON(bodyObj))
	}
	target := path
	if enc := q.Encode(); enc != "" {
		target += "?" + enc
	}
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, rt.method, target, body)
	} else {
		req, err = http.NewRequestWithContext(ctx, rt.method, target, nil)
	}
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if caller := callerOf(ctx); caller != nil {
		for _, h := range []string{"Authorization", "X-Forwarded-For", "X-Request-ID", "Accept-Language"} {
			if v := caller.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}
		req.Host = caller.Host
		req.RemoteAddr = caller.RemoteAddr
	}
	return trilha.WithVia(req, "mcp"), nil
}

// scalar is the argument as text: a JSON string unquoted, anything else as
// written.
func scalar(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(bytes.TrimSpace(raw))
}

func (b *bridge) call(ctx context.Context, rt routeTool, args json.RawMessage) (string, error) {
	req, err := b.build(ctx, rt, args)
	if err != nil {
		return "", err
	}
	rec := &recorder{h: http.Header{}, status: http.StatusOK}
	b.app.Handler().ServeHTTP(rec, req)
	text := strings.TrimSpace(rec.body.String())
	if rec.status >= 400 {
		if text == "" {
			text = "HTTP " + strconv.Itoa(rec.status) + " " + http.StatusText(rec.status)
		}
		return "", errors.New(text)
	}
	return text, nil
}

// allowed asks the route's chain whether this caller may reach it, with a
// placeholder in every path parameter: the chain decides on the key and the
// policy, not on the id.
func (b *bridge) allowed(ctx context.Context, rt routeTool) bool {
	if rt.name == "" {
		return false
	}
	path := rt.pattern
	for _, name := range rt.path {
		path = strings.Replace(path, "{"+name+"...}", "_", 1)
		path = strings.Replace(path, "{"+name+"}", "_", 1)
	}
	req, err := http.NewRequestWithContext(ctx, rt.method, path, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/json")
	if caller := callerOf(ctx); caller != nil {
		for _, h := range []string{"Authorization", "X-Forwarded-For"} {
			if v := caller.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}
		req.Host = caller.Host
		req.RemoteAddr = caller.RemoteAddr
	}
	return b.app.Probe(trilha.WithVia(req, "mcp"))
}

// recorder is the response of an in-process call.
type recorder struct {
	h      http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header         { return r.h }
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *recorder) WriteHeader(code int)        { r.status = code }
