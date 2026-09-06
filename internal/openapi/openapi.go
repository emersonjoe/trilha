// Package openapi turns a scanned app into an OpenAPI 3.1 document. What the
// scanner knows (path, method, path parameter) it takes; what the handler
// shows (the struct it binds, the status it writes) it reads from the source;
// what neither says is declared in an openapi: line of the doc comment.
package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/emersonjoe/trilha/internal/scan"
)

// FileName is where trilha openapi writes by default.
const FileName = "openapi.json"

// Options is what the document cannot know by itself.
type Options struct {
	Title   string
	Version string
	Server  string
}

type document struct {
	OpenAPI    string               `json:"openapi"`
	Info       info                 `json:"info"`
	Servers    []server             `json:"servers,omitempty"`
	Paths      map[string]*pathItem `json:"paths"`
	Components components           `json:"components"`
}

type info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type server struct {
	URL string `json:"url"`
}

type pathItem struct {
	Parameters []*parameter `json:"parameters,omitempty"`
	Get        *operation   `json:"get,omitempty"`
	Post       *operation   `json:"post,omitempty"`
	Put        *operation   `json:"put,omitempty"`
	Patch      *operation   `json:"patch,omitempty"`
	Delete     *operation   `json:"delete,omitempty"`
}

type operation struct {
	Tags        []string             `json:"tags,omitempty"`
	Summary     string               `json:"summary,omitempty"`
	Description string               `json:"description,omitempty"`
	OperationID string               `json:"operationId"`
	Parameters  []*parameter         `json:"parameters,omitempty"`
	RequestBody *requestBody         `json:"requestBody,omitempty"`
	Responses   map[string]*response `json:"responses"`
}

type parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *schema `json:"schema"`
}

type requestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]mediaType `json:"content"`
}

type response struct {
	Description string               `json:"description"`
	Content     map[string]mediaType `json:"content,omitempty"`
}

type mediaType struct {
	Schema *schema `json:"schema,omitempty"`
}

type components struct {
	Schemas map[string]*schema `json:"schemas,omitempty"`
}

// ProblemMediaType is what the runtime answers on an API error since 0.21.0.
const ProblemMediaType = "application/problem+json"

const jsonMediaType = "application/json"

type generator struct {
	ix      *index
	schemas map[string]*schema
	keys    map[string]string // package path + type name -> component key
	taken   map[string]bool
	busy    map[string]bool
}

// Generate reads the project at root and returns the document, indented, with
// a trailing newline. Same input, same bytes.
func Generate(root string, res *scan.Result, o Options) ([]byte, error) {
	ix, err := newIndex(root, res.Module)
	if err != nil {
		return nil, err
	}
	g := &generator{
		ix:      ix,
		schemas: map[string]*schema{"Problem": problemSchema()},
		keys:    map[string]string{},
		taken:   map[string]bool{"Problem": true},
		busy:    map[string]bool{},
	}
	doc := document{
		OpenAPI: "3.1.0",
		Info: info{
			Title:   or(o.Title, titleOf(res.Module)),
			Version: or(o.Version, "0.0.0"),
		},
		Paths: map[string]*pathItem{},
	}
	if o.Server != "" {
		doc.Servers = []server{{URL: o.Server}}
	}
	for _, r := range res.Routes {
		if r.Kind != "api" {
			continue
		}
		item, err := g.route(root, r)
		if err != nil {
			return nil, err
		}
		doc.Paths[openAPIPath(r.Pattern)] = item
	}
	doc.Components = components{Schemas: g.schemas}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func (g *generator) route(root string, r scan.Route) (*pathItem, error) {
	dir := filepath.Join(root, filepath.FromSlash(r.Dir))
	funcs, sc, err := routeFuncs(dir, r.ImportPath)
	if err != nil {
		return nil, err
	}
	item := &pathItem{Parameters: pathParams(r.Pattern)}
	for _, m := range r.Methods {
		if m == "OPTIONS" {
			continue // preflight is CORS mechanics, not an operation of the API
		}
		fn, ok := funcs[m]
		if !ok || fn.Body == nil {
			continue
		}
		h, err := g.read(fn, sc, filepath.ToSlash(filepath.Join(r.Dir, "route.go")))
		if err != nil {
			return nil, err
		}
		op := g.operation(r, m, h)
		switch m {
		case "GET":
			item.Get = op
		case "POST":
			item.Post = op
		case "PUT":
			item.Put = op
		case "PATCH":
			item.Patch = op
		case "DELETE":
			item.Delete = op
		}
	}
	return item, nil
}

func (g *generator) operation(r scan.Route, method string, h *handler) *operation {
	op := &operation{
		Summary:     h.summary,
		Description: h.description,
		OperationID: operationID(method, r.Pattern),
		Parameters:  h.queries,
		Responses:   map[string]*response{},
	}
	if tag := or(h.tag, defaultTag(r.Pattern)); tag != "" {
		op.Tags = []string{tag}
	}
	if h.body != nil {
		op.RequestBody = &requestBody{
			Required: true,
			Content:  map[string]mediaType{jsonMediaType: {Schema: h.body}},
		}
	}
	media := or(h.media, jsonMediaType)
	for code, s := range h.ok {
		resp := &response{Description: statusText(code)}
		switch {
		case s != nil:
			resp.Content = map[string]mediaType{media: {Schema: s}}
		case h.media != "" && code != http.StatusNoContent:
			// The handler said what it writes even though it is not a Go
			// value: a CSV is text, and saying text is better than silence.
			resp.Content = map[string]mediaType{h.media: {Schema: &schema{Type: "string"}}}
		}
		op.Responses[strconv.Itoa(code)] = resp
	}
	for code := range h.fail {
		if _, seen := op.Responses[strconv.Itoa(code)]; seen {
			continue
		}
		op.Responses[strconv.Itoa(code)] = problemResponse(statusText(code))
	}
	if len(op.Responses) == 0 {
		op.Responses["200"] = &response{Description: statusText(200)}
	}
	// Every API route answers an error the same way since 0.21.0, so the
	// client knows the shape without anyone writing it down.
	op.Responses["default"] = problemResponse("Unexpected error")
	return op
}

func problemResponse(desc string) *response {
	return &response{
		Description: desc,
		Content:     map[string]mediaType{ProblemMediaType: {Schema: &schema{Ref: "#/components/schemas/Problem"}}},
	}
}

func problemSchema() *schema {
	str := func(format string) *schema { return &schema{Type: "string", Format: format} }
	return &schema{
		Type:        "object",
		Description: "RFC 9457 problem details, the body of every API error.",
		Properties: map[string]*schema{
			"type":       str("uri"),
			"title":      {Type: "string"},
			"status":     {Type: "integer"},
			"detail":     {Type: "string"},
			"instance":   {Type: "string"},
			"request_id": {Type: "string"},
			"fields": {
				Type:                 "object",
				Description:          "One message per field, when the body failed validation.",
				AdditionalProperties: &schema{Type: "string"},
			},
		},
		Required: []string{"status", "title"},
	}
}

// openAPIPath turns the net/http pattern into the OpenAPI one: only the
// catch-all differs, and it loses the dots it does not need.
func openAPIPath(pattern string) string {
	return strings.ReplaceAll(pattern, "...}", "}")
}

func pathParams(pattern string) []*parameter {
	var ps []*parameter
	for _, seg := range strings.Split(pattern, "/") {
		if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
		p := &parameter{In: "path", Required: true, Schema: &schema{Type: "string"}}
		if strings.HasSuffix(name, "...") {
			p.Name = strings.TrimSuffix(name, "...")
			p.Description = "Catch-all: everything below this point, slashes included."
		} else {
			p.Name = name
		}
		ps = append(ps, p)
	}
	return ps
}

// defaultTag groups the operations by the last fixed segment of the path, so
// /api/posts and /api/posts/{id} land together.
func defaultTag(pattern string) string {
	segs := strings.Split(pattern, "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] != "" && !strings.HasPrefix(segs[i], "{") {
			return segs[i]
		}
	}
	return ""
}

func operationID(method, pattern string) string {
	var sb strings.Builder
	sb.WriteString(strings.ToLower(method))
	for _, seg := range strings.Split(pattern, "/") {
		seg = strings.Trim(seg, "{}")
		seg = strings.TrimSuffix(seg, "...")
		var word strings.Builder
		upper := true
		for _, r := range seg {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				upper = true
				continue
			}
			if upper {
				word.WriteRune(unicode.ToUpper(r))
				upper = false
				continue
			}
			word.WriteRune(r)
		}
		sb.WriteString(word.String())
	}
	return sb.String()
}

func statusText(code int) string {
	if t := http.StatusText(code); t != "" {
		return t
	}
	return fmt.Sprintf("Status %d", code)
}

func titleOf(module string) string {
	parts := strings.Split(module, "/")
	return parts[len(parts)-1]
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
