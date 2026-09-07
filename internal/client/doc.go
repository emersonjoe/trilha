// Package client reads an OpenAPI document that somebody else publishes and
// writes the Go client for it: one struct per schema, one method per operation,
// no dependencies. It is the other half of Config.Upstreams — the browser talks
// to the API through the prefix, and the page, which runs on the server, talks
// to it through these types.
//
// What it reads is JSON, OpenAPI 3.x. The reading lives here rather than in
// internal/openapi because that package writes the document Trilha itself emits;
// this one has to survive everything Trilha never emits — allOf, a $ref that
// closes a cycle, an operation with no tag.
package client

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Doc is the part of an OpenAPI document this generator reads. What it leaves
// out (servers, security, callbacks, webhooks) is left out on purpose: the
// credential comes from WithHeader and the base URL from the caller.
type Doc struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Paths      map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]*Schema `json:"schemas"`
	} `json:"components"`
}

// Schema is a JSON Schema node as OpenAPI uses it. Type is `any` because 3.1
// allows a list ("type": ["string","null"]); TypeName sorts that out.
type Schema struct {
	Ref                  string             `json:"$ref"`
	Type                 any                `json:"type"`
	Format               string             `json:"format"`
	Title                string             `json:"title"`
	Description          string             `json:"description"`
	Items                *Schema            `json:"items"`
	Properties           map[string]*Schema `json:"properties"`
	Required             []string           `json:"required"`
	Enum                 []any              `json:"enum"`
	AllOf                []*Schema          `json:"allOf"`
	OneOf                []*Schema          `json:"oneOf"`
	AnyOf                []*Schema          `json:"anyOf"`
	Nullable             bool               `json:"nullable"`
	Default              any                `json:"default"`
	MinLength            *int               `json:"minLength"`
	MaxLength            *int               `json:"maxLength"`
	Minimum              *float64           `json:"minimum"`
	Maximum              *float64           `json:"maximum"`
	AdditionalProperties json.RawMessage    `json:"additionalProperties"`
}

// Operation is one method of one path.
type Operation struct {
	OperationID string               `json:"operationId"`
	Summary     string               `json:"summary"`
	Description string               `json:"description"`
	Tags        []string             `json:"tags"`
	Parameters  []*Param             `json:"parameters"`
	RequestBody *Body                `json:"requestBody"`
	Responses   map[string]*Response `json:"responses"`
	Deprecated  bool                 `json:"deprecated"`

	// Filled in by Read.
	Method, Path string
}

// Param is a path, query or header parameter.
type Param struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema"`
}

// Body is a requestBody; only the media types the generator knows how to send
// are looked at.
type Body struct {
	Required bool                  `json:"required"`
	Content  map[string]*MediaType `json:"content"`
}

// Response is one status code of one operation.
type Response struct {
	Description string                `json:"description"`
	Content     map[string]*MediaType `json:"content"`
}

// MediaType is one entry of a content map.
type MediaType struct {
	Schema *Schema `json:"schema"`
}

// pathItem is the shape of one entry of `paths`: the methods, plus parameters
// shared by all of them.
type pathItem struct {
	Parameters []*Param `json:"parameters"`
}

// methods is the set of HTTP methods read from a path item, in the order they
// come out — sorted, so the generated file does not move between runs.
var methods = []string{"delete", "get", "patch", "post", "put"}

// Read parses a document. An operation with no operationId keeps its place: the
// name is derived from the method and the path later, because operationId is
// optional and half the APIs in the world leave it out.
func Read(data []byte) (*Doc, []*Operation, error) {
	var d Doc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, nil, fmt.Errorf("openapi: %w", err)
	}
	if d.OpenAPI == "" {
		return nil, nil, fmt.Errorf("openapi: no \"openapi\" version in the document")
	}
	if strings.HasPrefix(d.OpenAPI, "2.") {
		return nil, nil, fmt.Errorf("openapi: %s is Swagger 2.0; convert it to 3.x first", d.OpenAPI)
	}
	paths := make([]string, 0, len(d.Paths))
	for p := range d.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var ops []*Operation
	for _, p := range paths {
		var item pathItem
		if err := json.Unmarshal(d.Paths[p], &item); err != nil {
			return nil, nil, fmt.Errorf("openapi: path %s: %w", p, err)
		}
		var byMethod map[string]json.RawMessage
		if err := json.Unmarshal(d.Paths[p], &byMethod); err != nil {
			return nil, nil, fmt.Errorf("openapi: path %s: %w", p, err)
		}
		for _, m := range methods {
			raw, ok := byMethod[m]
			if !ok {
				continue
			}
			op := &Operation{Method: strings.ToUpper(m), Path: p}
			if err := json.Unmarshal(raw, op); err != nil {
				return nil, nil, fmt.Errorf("openapi: %s %s: %w", m, p, err)
			}
			// A parameter declared on the path item applies to every method,
			// unless the method declares one with the same name and place.
			for _, sh := range item.Parameters {
				dup := false
				for _, own := range op.Parameters {
					if own.Name == sh.Name && own.In == sh.In {
						dup = true
						break
					}
				}
				if !dup {
					op.Parameters = append(op.Parameters, sh)
				}
			}
			ops = append(ops, op)
		}
	}
	return &d, ops, nil
}

// Resolve follows a $ref inside the document. Only local refs are followed: a
// document that points at another file is a document this generator cannot see,
// and saying so is better than fetching whatever the URL happens to serve.
func (d *Doc) Resolve(s *Schema) (*Schema, string) {
	name := ""
	for i := 0; s != nil && s.Ref != "" && i < 32; i++ {
		n := refName(s.Ref)
		if n == "" {
			return nil, ""
		}
		name = n
		s = d.Components.Schemas[n]
	}
	return s, name
}

// refName is the last segment of a local $ref, or "" for anything else.
func refName(ref string) string {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	return strings.TrimPrefix(ref, prefix)
}

// TypeName is the JSON type of a schema, with the 3.1 list form collapsed and
// "null" dropped — a nullable string is a string here, and the pointer question
// is answered by `required`, not by the type list.
func (s *Schema) TypeName() string {
	switch t := s.Type.(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if str, ok := v.(string); ok && str != "null" {
				return str
			}
		}
	}
	return ""
}

// IsRequired says whether a property is in the schema's required list.
func (s *Schema) IsRequired(prop string) bool {
	for _, r := range s.Required {
		if r == prop {
			return true
		}
	}
	return false
}
