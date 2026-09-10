package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/h"
)

// The document trilha openapi would write for the routes below, cut to what
// the bridge reads. The upload operation is declared multipart on purpose.
const routesDoc = `{
  "openapi": "3.1.0",
  "paths": {
    "/api/v1/items": {
      "get": {"summary": "GET lists the items.", "operationId": "getApiV1Items",
        "parameters": [{"name": "q", "in": "query", "description": "filter by name", "schema": {"type": "string"}}]},
      "post": {"summary": "POST creates an item.", "description": "The body is the form's struct.", "operationId": "postApiV1Items",
        "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/store.NewItem"}}}}}
    },
    "/api/v1/items/{id}": {
      "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}],
      "get": {"summary": "GET returns one item.", "operationId": "getApiV1ItemsId"}
    },
    "/api/v1/upload": {
      "post": {"summary": "POST receives a file.", "operationId": "postApiV1Upload",
        "requestBody": {"content": {"multipart/form-data": {"schema": {"type": "object"}}}}}
    }
  },
  "components": {"schemas": {
    "store.NewItem": {"type": "object", "properties": {"name": {"type": "string", "maxLength": 40}, "kind": {"$ref": "#/components/schemas/store.Kind"}}, "required": ["name"]},
    "store.Kind": {"type": "string", "enum": ["book", "tool"]}
  }}
}`

type auditSink struct {
	mu   sync.Mutex
	recs []trilha.AuditRecord
}

func (s *auditSink) Write(r trilha.AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, r)
	return nil
}

type routesFixture struct {
	app    *trilha.App
	keys   *auth.Keys
	audit  *auditSink
	logs   *bytes.Buffer
	full   string // a key with both scopes
	reader string // a key with docs:read only
	posted []string
}

func routesApp(t *testing.T, doc []byte) *routesFixture {
	t.Helper()
	f := &routesFixture{audit: &auditSink{}, logs: &bytes.Buffer{}}
	f.app = trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte(strings.Repeat("k", 40)),
		Audit: f.audit, Logger: slog.New(slog.NewTextHandler(f.logs, nil))})
	f.keys = auth.APIKeys(auth.KeyOptions{Scopes: []string{"docs:read", "docs:write"}})
	_, f.full, _ = f.keys.Issue(nil, "full", []string{"docs:read", "docs:write"}, 0)
	_, f.reader, _ = f.keys.Issue(nil, "reader", []string{"docs:read"}, 0)
	// Built before any route exists, which is the order of a real app: Setup
	// runs first and trilha_gen.go registers the routes after it.
	srv := FromRoutes(f.app, FromRoutesOpts{Name: "loja", Version: "1.0", Include: []string{"/api/v1/*"}, OpenAPI: doc})
	f.app.Register(trilha.Route{Pattern: "/mcp", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{"POST": srv.ServeHTTP}})
	items := map[string]string{"1": "Livro", "2": "Chave"}
	f.app.Register(trilha.Route{Pattern: "/api/v1/items", Kind: trilha.KindAPI,
		Middlewares:         []trilha.MiddlewareFunc{f.keys.Require("docs:read")},
		MiddlewaresByMethod: map[string][]trilha.MiddlewareFunc{"POST": {f.keys.Require("docs:write")}},
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				if q := c.Query("q"); q != "" {
					return c.JSON(200, map[string]any{"q": q, "items": []string{}})
				}
				return c.JSON(200, map[string]any{"items": []string{"1", "2"}})
			},
			"POST": func(c *trilha.Ctx) error {
				var in struct {
					Name string `json:"name" validate:"required"`
					Kind string `json:"kind"`
				}
				if err := c.BindJSON(&in); err != nil {
					return err
				}
				f.posted = append(f.posted, in.Name)
				c.Audit("item.created", in.Name)
				return c.JSON(201, map[string]string{"id": "3", "name": in.Name, "kind": in.Kind})
			},
		}})
	f.app.Register(trilha.Route{Pattern: "/api/v1/items/{id}", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{f.keys.Require("docs:read")},
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			name, ok := items[c.Param("id")]
			if !ok {
				return trilha.ErrNotFound
			}
			return c.JSON(200, map[string]string{"id": c.Param("id"), "name": name})
		}}})
	f.app.Register(trilha.Route{Pattern: "/api/v1/upload", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{f.keys.Require("docs:write")},
		Methods:     map[string]trilha.HandlerFunc{"POST": func(c *trilha.Ctx) error { return c.Text(200, "no") }}})
	// A page under the prefix is not an API and never becomes a tool.
	f.app.Register(trilha.Route{Pattern: "/api/v1/help", Page: func(c *trilha.Ctx) (h.Node, error) { return h.Text("help"), nil },
		Methods: map[string]trilha.HandlerFunc{"POST": func(c *trilha.Ctx) error { return nil }}})
	return f
}

func (f *routesFixture) dial(t *testing.T, key string) *Client {
	t.Helper()
	hs := httptest.NewServer(f.app.Handler())
	t.Cleanup(hs.Close)
	headers := map[string]string{}
	if key != "" {
		headers["Authorization"] = "Bearer " + key
	}
	c, err := Dial(context.Background(), HTTP(hs.URL+"/mcp", headers))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func (f *routesFixture) direct(t *testing.T, key, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	f.app.Handler().ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func names(tools []ToolInfo) []string {
	var out []string
	for _, t := range tools {
		out = append(out, t.Name)
	}
	return out
}

// SC-001: three API routes behind keys are three tools, and a tool answers
// exactly what the route answers over HTTP — same handler, same JSON.
func TestFromRoutesExposesTheAPIAsTools(t *testing.T) {
	f := routesApp(t, []byte(routesDoc))
	c := f.dial(t, f.full)
	ctx := context.Background()
	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(tools), ","); got != "getApiV1Items,getApiV1ItemsId,postApiV1Items" {
		t.Fatalf("tools = %s", got)
	}
	byName := map[string]ToolInfo{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}
	if d := byName["postApiV1Items"].Description; !strings.HasPrefix(d, "POST creates an item.") || !strings.Contains(d, "form's struct") {
		t.Errorf("description = %q", d)
	}
	// The schema is the document's, flattened: path, query and body at the
	// top level, and the component the body referenced carried along.
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		Defs       map[string]json.RawMessage `json:"$defs"`
	}
	_ = json.Unmarshal(byName["postApiV1Items"].InputSchema, &schema)
	if _, ok := schema.Properties["name"]; !ok || strings.Join(schema.Required, ",") != "name" {
		t.Errorf("post schema = %s", byName["postApiV1Items"].InputSchema)
	}
	if !strings.Contains(string(schema.Properties["kind"]), `"#/$defs/store.Kind"`) || !strings.Contains(string(schema.Defs["store.Kind"]), "book") {
		t.Errorf("the referenced component must travel inside the schema: %s", byName["postApiV1Items"].InputSchema)
	}
	_ = json.Unmarshal(byName["getApiV1ItemsId"].InputSchema, &schema)
	if _, ok := schema.Properties["id"]; !ok || strings.Join(schema.Required, ",") != "id" {
		t.Errorf("get one schema = %s", byName["getApiV1ItemsId"].InputSchema)
	}
	_ = json.Unmarshal(byName["getApiV1Items"].InputSchema, &schema)
	if s := string(byName["getApiV1Items"].InputSchema); !strings.Contains(s, `"q"`) || !strings.Contains(s, "filter by name") {
		t.Errorf("list schema = %s", s)
	}

	// Same JSON as the curl.
	res, err := c.CallTool(ctx, "getApiV1ItemsId", json.RawMessage(`{"id":"1"}`))
	if err != nil || res.IsError {
		t.Fatalf("%v %+v", err, res)
	}
	_, want := f.direct(t, f.full, "GET", "/api/v1/items/1", "")
	if res.Text() != strings.TrimSpace(want) {
		t.Fatalf("tool = %q, curl = %q", res.Text(), want)
	}
	// A query parameter goes to the query string.
	res, _ = c.CallTool(ctx, "getApiV1Items", json.RawMessage(`{"q":"liv"}`))
	if res.IsError || !strings.Contains(res.Text(), `"q":"liv"`) {
		t.Fatalf("%+v", res)
	}
	// The rest goes to the body, and 201 is still a success.
	res, _ = c.CallTool(ctx, "postApiV1Items", json.RawMessage(`{"name":"Caneta","kind":"tool"}`))
	if res.IsError || !strings.Contains(res.Text(), `"name":"Caneta"`) || len(f.posted) != 1 {
		t.Fatalf("%+v posted=%v", res, f.posted)
	}
	// A 4xx of the route is a tool error carrying the problem.
	res, _ = c.CallTool(ctx, "getApiV1ItemsId", json.RawMessage(`{"id":"9"}`))
	if !res.IsError || !strings.Contains(res.Text(), `"status":404`) {
		t.Fatalf("%+v", res)
	}
	res, _ = c.CallTool(ctx, "postApiV1Items", json.RawMessage(`{"kind":"tool"}`))
	if !res.IsError || !strings.Contains(res.Text(), `"status":422`) || len(f.posted) != 1 {
		t.Fatalf("%+v", res)
	}
	// SC-005: the multipart operation stayed out, and the log says so.
	if !strings.Contains(f.logs.String(), "postApiV1Upload") || !strings.Contains(f.logs.String(), "multipart") {
		t.Fatalf("no warning about the upload:\n%s", f.logs.String())
	}
}

// SC-004: the trail says the key acted through the MCP, not through the API.
func TestFromRoutesAuditsTheKeyViaMCP(t *testing.T) {
	f := routesApp(t, []byte(routesDoc))
	c := f.dial(t, f.full)
	if _, err := c.CallTool(context.Background(), "postApiV1Items", json.RawMessage(`{"name":"X"}`)); err != nil {
		t.Fatal(err)
	}
	var rec *trilha.AuditRecord
	for i := range f.audit.recs {
		if f.audit.recs[i].Action == "item.created" {
			rec = &f.audit.recs[i]
		}
	}
	if rec == nil {
		t.Fatalf("no audit record: %+v", f.audit.recs)
	}
	if rec.Actor.Via != "mcp" || !strings.HasPrefix(rec.Actor.Subject, "key:") || rec.Actor.Name != "full" || rec.Route != "/api/v1/items" {
		t.Fatalf("actor = %+v route=%s", rec.Actor, rec.Route)
	}
}

// SC-002 and SC-003: the key decides, before and during. A wrong key gets the
// route's 401 and runs nothing; a key without the scope does not even see the
// tool — and asking for it by name is the same as asking for one that does
// not exist.
func TestFromRoutesFiltersByTheCallersKey(t *testing.T) {
	f := routesApp(t, []byte(routesDoc))
	ctx := context.Background()

	bad := f.dial(t, "ak_nope_nope")
	tools, err := bad.ListTools(ctx)
	if err != nil || len(tools) != 0 {
		t.Fatalf("a wrong key sees %v (%v)", names(tools), err)
	}
	res, err := bad.CallTool(ctx, "postApiV1Items", json.RawMessage(`{"name":"X"}`))
	if err == nil || !strings.Contains(err.Error(), "unknown tool") || len(f.posted) != 0 {
		t.Fatalf("%v %+v posted=%v", err, res, f.posted)
	}

	reader := f.dial(t, f.reader)
	tools, _ = reader.ListTools(ctx)
	if got := strings.Join(names(tools), ","); got != "getApiV1Items,getApiV1ItemsId" {
		t.Fatalf("reader sees %s", got)
	}
	if _, err := reader.CallTool(ctx, "postApiV1Items", json.RawMessage(`{"name":"X"}`)); err == nil || len(f.posted) != 0 {
		t.Fatal("the reader wrote:", err, f.posted)
	}
	res, err = reader.CallTool(ctx, "getApiV1ItemsId", json.RawMessage(`{"id":"2"}`))
	if err != nil || res.IsError || !strings.Contains(res.Text(), "Chave") {
		t.Fatalf("%v %+v", err, res)
	}
	// Listing costs the key nothing: the usage counter is untouched by probes
	// (proved in auth); here, the direct call still answers after the lists.
	if code, _ := f.direct(t, f.reader, "GET", "/api/v1/items", ""); code != 200 {
		t.Fatal(code)
	}
}

// Without the document the tools still exist: same names, the method and
// path as description, and the path parameters as the schema.
func TestFromRoutesWithoutTheDocument(t *testing.T) {
	f := routesApp(t, nil)
	c := f.dial(t, f.full)
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(tools), ","); got != "getApiV1Items,getApiV1ItemsId,postApiV1Items,postApiV1Upload" {
		t.Fatalf("tools = %s", got)
	}
	for _, tl := range tools {
		if tl.Name == "getApiV1ItemsId" {
			if tl.Description != "GET /api/v1/items/{id}" || !strings.Contains(string(tl.InputSchema), `"id"`) {
				t.Fatalf("%+v", tl)
			}
		}
	}
	res, _ := c.CallTool(context.Background(), "getApiV1ItemsId", json.RawMessage(`{"id":"1"}`))
	if res.IsError || !strings.Contains(res.Text(), "Livro") {
		t.Fatalf("%+v", res)
	}
}

// The list a person checks before publishing: the same derivation, from the
// document alone, with what stayed out and why.
func TestPreviewListsWhatWouldBeExposed(t *testing.T) {
	tools, warnings, err := Preview([]byte(routesDoc), map[string][]string{
		"/api/v1/items": {"GET", "POST"}, "/api/v1/items/{id}": {"GET"}, "/api/v1/upload": {"POST"}, "/internal/x": {"GET"},
	}, FromRoutesOpts{Include: []string{"/api/v1/*"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(tools), ","); got != "getApiV1Items,getApiV1ItemsId,postApiV1Items" {
		t.Fatalf("tools = %s", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "postApiV1Upload") {
		t.Fatalf("warnings = %v", warnings)
	}
	if _, _, err := Preview([]byte("{"), nil, FromRoutesOpts{}); err == nil {
		t.Fatal("a broken document must be an error")
	}
}

func TestIncludeMatchesPrefixesAndExactPaths(t *testing.T) {
	cases := []struct {
		pats []string
		path string
		want bool
	}{
		{nil, "/api/x", true}, {nil, "/x", false},
		{[]string{"/api/v1/*"}, "/api/v1/items", true}, {[]string{"/api/v1/*"}, "/api/v1", true}, {[]string{"/api/v1/*"}, "/api/v10", false},
		{[]string{"/api/items"}, "/api/items", true}, {[]string{"/api/items"}, "/api/items/{id}", false},
	}
	for _, tc := range cases {
		if got := matchAny(tc.pats, tc.path); got != tc.want {
			t.Errorf("match(%v, %s) = %v", tc.pats, tc.path, got)
		}
	}
}

var _ http.Handler = (*Server)(nil).Handler()
