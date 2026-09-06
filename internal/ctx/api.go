package ctx

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha/internal/openapi"
	"github.com/emersonjoe/trilha/internal/scan"
)

// The contract of an API handler is not read twice: the openapi generator
// already infers what the handler binds and what it answers, so ctx asks it
// for the document and reads it back. It costs one serialization and buys the
// guarantee that trilha ctx and trilha openapi can never disagree.

type doc struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]*docSchema `json:"schemas"`
	} `json:"components"`
}

type docOp struct {
	Summary     string `json:"summary"`
	Parameters  []docParam
	RequestBody *docBody            `json:"requestBody"`
	Responses   map[string]*docBody `json:"responses"`
}

type docParam struct {
	Name string `json:"name"`
	In   string `json:"in"`
}

type docBody struct {
	Content map[string]struct {
		Schema *docSchema `json:"schema"`
	} `json:"content"`
}

type docSchema struct {
	Ref        string                `json:"$ref"`
	Type       string                `json:"type"`
	Format     string                `json:"format"`
	Enum       []any                 `json:"enum"`
	MinLength  *int                  `json:"minLength"`
	MaxLength  *int                  `json:"maxLength"`
	Minimum    *float64              `json:"minimum"`
	Maximum    *float64              `json:"maximum"`
	Items      *docSchema            `json:"items"`
	Properties map[string]*docSchema `json:"properties"`
	Required   []string              `json:"required"`
}

// document generates the OpenAPI document and decodes it. Title and version
// are fixed: nothing about them belongs in a map of the project.
func document(root string, res *scan.Result) (*doc, error) {
	b, err := openapi.Generate(root, res, openapi.Options{})
	if err != nil {
		return nil, err
	}
	var d doc
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func operations(r scan.Route, d *doc, used map[string]bool) []Operation {
	item := d.Paths[r.Pattern]
	if item == nil {
		return nil
	}
	var out []Operation
	for _, m := range r.Methods {
		raw, ok := item[strings.ToLower(m)]
		if !ok {
			continue
		}
		var op docOp
		if err := json.Unmarshal(raw, &op); err != nil {
			continue
		}
		o := Operation{Method: m, Summary: op.Summary}
		for _, p := range op.Parameters {
			if p.In == "query" {
				o.Query = append(o.Query, p.Name)
			}
		}
		if op.RequestBody != nil {
			media, s := body(op.RequestBody)
			o.Request = name(s, media, d, used)
		}
		var codes []string
		for code := range op.Responses {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			status, err := strconv.Atoi(code)
			if err != nil {
				continue
			}
			media, s := body(op.Responses[code])
			resp := Response{Status: status, Type: name(s, "", d, used)}
			if media != "" && media != "application/json" {
				resp.Media = media
			}
			o.Responses = append(o.Responses, resp)
		}
		out = append(out, o)
	}
	return out
}

// body picks the media type of a request or response, JSON first: it is the
// one the handler usually means, and the others say so by name.
func body(b *docBody) (string, *docSchema) {
	if b == nil || len(b.Content) == 0 {
		return "", nil
	}
	if c, ok := b.Content["application/json"]; ok {
		return "application/json", c.Schema
	}
	var medias []string
	for m := range b.Content {
		medias = append(medias, m)
	}
	sort.Strings(medias)
	return medias[0], b.Content[medias[0]].Schema
}

// name renders a schema the way a Go reader wants to see it — posts.Post,
// posts.Post[], string — and records every component it walked through, so
// the types section lists what the routes actually name.
func name(s *docSchema, media string, d *doc, used map[string]bool) string {
	if s == nil {
		if media != "" && media != "application/json" {
			return media
		}
		return ""
	}
	if s.Ref != "" {
		key := strings.TrimPrefix(s.Ref, "#/components/schemas/")
		mark(key, d, used)
		return key
	}
	if s.Type == "array" && s.Items != nil {
		return name(s.Items, "", d, used) + "[]"
	}
	if s.Type == "object" && len(s.Properties) > 0 {
		var fs []string
		for f := range s.Properties {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		return "object{" + strings.Join(fs, ", ") + "}"
	}
	if s.Type == "" {
		return ""
	}
	return s.Type
}

// mark records a component and everything it points at.
func mark(key string, d *doc, used map[string]bool) {
	if used[key] {
		return
	}
	used[key] = true
	s := d.Components.Schemas[key]
	if s == nil {
		return
	}
	var fields []string
	for f := range s.Properties {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	for _, f := range fields {
		walk(s.Properties[f], d, used)
	}
}

func walk(s *docSchema, d *doc, used map[string]bool) {
	if s == nil {
		return
	}
	if s.Ref != "" {
		mark(strings.TrimPrefix(s.Ref, "#/components/schemas/"), d, used)
		return
	}
	walk(s.Items, d, used)
}

// types lists the components the routes named, with the rules the validate
// tag declared.
func types(d *doc, used map[string]bool) []Type {
	var names []string
	for k := range d.Components.Schemas {
		if used[k] {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	var out []Type
	for _, n := range names {
		s := d.Components.Schemas[n]
		t := Type{Name: n}
		req := map[string]bool{}
		for _, r := range s.Required {
			req[r] = true
		}
		var fields []string
		for f := range s.Properties {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		for _, f := range fields {
			p := s.Properties[f]
			t.Fields = append(t.Fields, Field{
				Name:     f,
				Type:     name(p, "", d, used),
				Required: req[f],
				Rules:    rules(p),
			})
		}
		out = append(out, t)
	}
	return out
}

// rules turns the schema constraints back into the validate tag they came
// from, in one line.
func rules(s *docSchema) string {
	if s == nil {
		return ""
	}
	var out []string
	if s.Format != "" {
		out = append(out, s.Format)
	}
	if s.MinLength != nil {
		out = append(out, fmt.Sprintf("min %d", *s.MinLength))
	}
	if s.MaxLength != nil {
		out = append(out, fmt.Sprintf("max %d", *s.MaxLength))
	}
	if s.Minimum != nil {
		out = append(out, fmt.Sprintf(">= %s", num(*s.Minimum)))
	}
	if s.Maximum != nil {
		out = append(out, fmt.Sprintf("<= %s", num(*s.Maximum)))
	}
	if len(s.Enum) > 0 {
		var vs []string
		for _, e := range s.Enum {
			vs = append(vs, fmt.Sprint(e))
		}
		out = append(out, "one of "+strings.Join(vs, "|"))
	}
	return strings.Join(out, ", ")
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
