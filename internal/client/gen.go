package client

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Options are what the CLI passes down: where the file goes and what it is
// called from the inside.
type Options struct {
	Package string // package clause of the generated file (default "api")
	Source  string // what the header says the document was (a path or a URL)
}

// Result is what Generate produced: the file, the lines of the report, and the
// count that says how much of the API the client actually types. An API written
// without response_model declares no schema for the answer, and every operation
// comes back as json.RawMessage — the file compiles, types nothing, and whoever
// is migrating used to find out by opening it.
type Result struct {
	Source  []byte
	Notes   []Note
	Ops     int      // operations in the document
	Untyped []string // the ones whose answer is json.RawMessage, as "GET /path"
}

// method is one generated method: everything the template needs, already
// resolved, so the emission below is only text.
type method struct {
	Name     string
	Doc      string
	HTTP     string
	Path     string
	PathVars []pathVar
	Query    []queryParam
	Params   string // name of the params struct, "" when there is none
	Body     string // Go type of the JSON body, "" when there is none
	Upload   string // multipart field name when the form is one file and nothing else
	Form     *form  // the multipart form when it is anything more than that
	Return   string // Go type of the answer, "" when there is none
	Binary   bool
}

// form is a multipart body read as what it is: a list of parts. One binary
// field and nothing else keeps the two arguments it always had; anything else
// — a file with a password beside it, fifty files under one name — becomes a
// struct, because the alternative is a signature nobody can read and a field
// the generator silently drops.
type form struct {
	Type  string // name of the generated struct
	Parts []formPart
}

// formPart is one field of the form, in the order the document declares it.
type formPart struct {
	Wire     string // the name on the wire
	Field    string // the Go field
	Type     string // the scalar Go type; "" when it is a file
	Many     bool   // repeated under the same name
	Required bool
	Doc      string
}

// File says whether the part carries bytes.
func (p formPart) File() bool { return p.Type == "" }

type pathVar struct {
	Name string // Go argument
	Raw  string // {name} in the path
}

type queryParam struct {
	Wire     string
	Field    string
	Type     string
	Required bool
}

// group is a tag: the methods of one part of the API, on one struct.
type group struct {
	Name    string
	Doc     string
	Methods []*method
}

// Generate reads the document and writes the client. It is deterministic: the
// same bytes in, the same bytes out, which is what lets --check mean anything.
func Generate(data []byte, opt Options) (*Result, error) {
	d, ops, err := Read(data)
	if err != nil {
		return nil, err
	}
	if opt.Package == "" {
		opt.Package = "api"
	}
	b := newBuilder(d)
	b.components()

	groups := map[string]*group{}
	var order []string
	used := map[string]bool{}
	for _, op := range ops {
		gName := "Default"
		if len(op.Tags) > 0 && strings.TrimSpace(op.Tags[0]) != "" {
			gName = exportName(op.Tags[0])
		}
		g := groups[gName]
		if g == nil {
			g = &group{Name: gName}
			groups[gName] = g
			order = append(order, gName)
		}
		m, err := b.method(op, gName, used)
		if err != nil {
			return nil, err
		}
		g.Methods = append(g.Methods, m)
	}
	sort.Strings(order)
	for _, n := range order {
		sort.SliceStable(groups[n].Methods, func(i, j int) bool {
			return groups[n].Methods[i].Name < groups[n].Methods[j].Name
		})
	}

	src := b.emit(d, opt, order, groups)
	out, err := format.Source(src)
	if err != nil {
		return nil, fmtErr("the generated file does not parse: %w", err)
	}
	return &Result{Source: out, Notes: b.sortedNotes(), Ops: len(ops), Untyped: b.untyped}, nil
}

// method resolves one operation into everything the emission needs.
func (b *builder) method(op *Operation, gName string, used map[string]bool) (*method, error) {
	where := op.Method + " " + op.Path
	m := &method{HTTP: op.Method, Path: op.Path, Doc: firstSentence(op.Summary, op.Description)}
	m.Name = uniqueName(methodName(op, gName), gName, op, used)

	// Path parameters are positional by nature, so they go in the signature.
	for _, seg := range strings.Split(op.Path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			raw := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			m.PathVars = append(m.PathVars, pathVar{Name: argName(raw), Raw: seg})
		}
	}
	// Query parameters are a struct: ten optional arguments in a signature is
	// unreadable, and the struct is also the Bind of a form.
	var qs []*Param
	for _, p := range op.Parameters {
		if p != nil && p.In == "query" {
			qs = append(qs, p)
		}
	}
	if len(qs) > 0 {
		sort.SliceStable(qs, func(i, j int) bool { return qs[i].Name < qs[j].Name })
		name := gName + m.Name + "Params"
		st := &Struct{Name: name, Doc: "Query of " + where + "."}
		for _, p := range qs {
			t := b.goType(p.Schema, name+exportName(p.Name), where+"?"+p.Name)
			tag := "json:\"" + p.Name
			if !p.Required {
				tag += ",omitempty"
			}
			tag += "\""
			if v := validateTag(p.Schema, p.Required); v != "" {
				tag += " validate:\"" + v + "\""
			}
			st.Fields = append(st.Fields, Field{Name: exportName(p.Name), Type: t, Tag: tag, Doc: doc(p.Schema)})
			m.Query = append(m.Query, queryParam{Wire: p.Name, Field: exportName(p.Name), Type: t, Required: p.Required})
		}
		b.byName[name] = st
		b.order = append(b.order, name)
		m.Params = name
	}
	// The body: JSON becomes the schema's type, multipart becomes a reader and
	// a filename, anything else is a line in the report.
	if op.RequestBody != nil {
		switch ct, mt := pickBody(op.RequestBody.Content); ct {
		case "":
		case "multipart/form-data":
			m.Upload, m.Form = b.multipart(mt, gName+m.Name+"Form", where)
		default:
			if strings.Contains(ct, "json") {
				m.Body = b.goType(mt.Schema, gName+m.Name+"Body", where+" body")
			} else {
				b.note(where, "request body is "+ct+" — send it with the raw client")
			}
		}
	}
	// The answer: the first 2xx that carries content.
	if ct, mt := pickResponse(op.Responses); ct != "" {
		switch {
		case strings.Contains(ct, "json"):
			before := len(b.notes)
			m.Return = b.goType(mt.Schema, gName+m.Name+"Response", where+" response")
			// A schema the generator could not name is an answer the caller
			// has to unmarshal by hand. One of those is a note; ninety-seven
			// of them is the shape of the API, and that is a count — so the
			// operation goes on the count and its own line comes back only
			// under --verbose, instead of the same fact twice.
			if m.Return == "json.RawMessage" {
				b.untyped = append(b.untyped, where)
				b.dropNotes(before, where+" response")
			}
		default:
			m.Binary = true
		}
	}
	if op.OperationID == "" {
		b.note(where, "no operationId — the name came from the method and the path")
	}
	return m, nil
}

// methodName is decision 6: the operationId when it exists, minus the part of
// it that only repeats the path and the method (FastAPI writes
// list_documents_api_documents_get), minus the group's own name.
func methodName(op *Operation, gName string) string {
	if op.OperationID == "" {
		return fallbackName(op)
	}
	name := exportName(op.OperationID)
	if tail := exportName(op.Path + "_" + strings.ToLower(op.Method)); tail != "" && len(name) > len(tail) {
		name = strings.TrimSuffix(name, tail)
	}
	// What is left often repeats the tag: Documents.ListDocuments reads worse
	// than Documents.List, and the singular is the spelling APIs actually use.
	for _, word := range []string{gName, singular(gName)} {
		for _, cut := range []func(string) string{
			func(s string) string { return strings.TrimSuffix(s, word) },
			func(s string) string { return trimWordPrefix(s, word) },
		} {
			if trimmed := cut(name); trimmed != "" && trimmed != name {
				name = trimmed
			}
		}
	}
	return name
}

// trimWordPrefix drops word from the front of a PascalCase name, but only when
// what is left starts a word of its own. Cutting the tag `config` off
// ConfigurarRegra leaves `urarRegra`: a method that is not exported, which the
// caller's package cannot see, from a name that is not in the document either.
// When the tag is merely the start of the first word there is no repetition to
// remove, so the name stays whole. The suffix side needs no such guard: the cut
// is case-sensitive, so a match already lands on the start of a word.
func trimWordPrefix(s, word string) string {
	rest := strings.TrimPrefix(s, word)
	if rest == s {
		return s
	}
	if r, _ := utf8.DecodeRuneInString(rest); !unicode.IsUpper(r) {
		return s
	}
	return rest
}

// verbs name an operation that has no operationId.
var verbs = map[string]string{
	"GET": "Get", "POST": "Create", "PUT": "Update", "PATCH": "Update", "DELETE": "Delete",
}

// fallbackName is method plus the last static segment: GET /api/documents is
// List, GET /api/documents/{id} is GetDocument.
func fallbackName(op *Operation) string {
	segs := strings.Split(strings.Trim(op.Path, "/"), "/")
	last, tail := "", ""
	for _, s := range segs {
		if strings.HasPrefix(s, "{") {
			continue
		}
		last = s
	}
	tail = exportName(last)
	if op.Method == "GET" && !strings.HasSuffix(op.Path, "}") {
		return "List" + tail
	}
	return verbs[op.Method] + singular(tail)
}

// uniqueName keeps two operations of one group from colliding.
func uniqueName(name, gName string, op *Operation, used map[string]bool) string {
	key := gName + "." + name
	if !used[key] {
		used[key] = true
		return name
	}
	for _, seg := range strings.Split(strings.Trim(op.Path, "/"), "/") {
		if strings.HasPrefix(seg, "{") {
			continue
		}
		cand := name + exportName(seg)
		if !used[gName+"."+cand] {
			used[gName+"."+cand] = true
			return cand
		}
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s%d", name, i)
		if !used[gName+"."+cand] {
			used[gName+"."+cand] = true
			return cand
		}
	}
}

// pickBody chooses the media type to send: JSON first, then multipart, then
// whatever came, so the report can name it.
func pickBody(content map[string]*MediaType) (string, *MediaType) {
	for _, want := range []string{"application/json", "multipart/form-data"} {
		if mt, ok := content[want]; ok {
			return want, mt
		}
	}
	keys := make([]string, 0, len(content))
	for k := range content {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.Contains(k, "json") {
			return k, content[k]
		}
	}
	if len(keys) > 0 {
		return keys[0], content[keys[0]]
	}
	return "", nil
}

// multipart reads a multipart body as what it is: a list of parts. It answers
// with the single field name when the form is one binary and nothing else —
// the shape that keeps the two arguments it always had — and with the form
// otherwise.
//
// The parts come out in the order of the property names, which is the order of
// the struct fields: the document has no order of its own to preserve, and a
// generator that answers differently on two runs is a generator nobody can
// check.
func (b *builder) multipart(mt *MediaType, name, where string) (string, *form) {
	if mt == nil || mt.Schema == nil {
		return "file", nil
	}
	names := make([]string, 0, len(mt.Schema.Properties))
	for n := range mt.Schema.Properties {
		names = append(names, n)
	}
	sort.Strings(names)

	f := &form{Type: name}
	files := 0
	for _, n := range names {
		s := mt.Schema.Properties[n]
		if s == nil {
			continue
		}
		part := formPart{Wire: n, Field: exportName(n), Required: mt.Schema.IsRequired(n), Doc: doc(s)}
		switch {
		case s.Format == "binary":
			files++
		case s.TypeName() == "array" && s.Items != nil && s.Items.Format == "binary":
			part.Many, files = true, files+1
		default:
			t := b.goType(s, name+exportName(n), where+" "+n)
			if !scalar(t) {
				// A nested object inside a form is a decision, not a
				// translation: the note says so instead of the client
				// inventing an encoding for it.
				b.note(where, "multipart field "+n+" is not a scalar — send it with the raw client")
				continue
			}
			part.Type = t
		}
		f.Parts = append(f.Parts, part)
	}
	switch {
	case len(f.Parts) == 0:
		return "file", nil
	case len(f.Parts) == 1 && files == 1 && !f.Parts[0].Many:
		// One file and nothing else: the call that was already right.
		return f.Parts[0].Wire, nil
	}
	return "", f
}

// scalar says whether a Go type can be written as one form field. An enum is a
// string with a name, and it counts.
func scalar(t string) bool {
	switch t {
	case "string", "bool", "int32", "int64", "float32", "float64":
		return true
	}
	// A named enum is a string underneath, and the generator only invents
	// names for enums out of a string schema.
	return t != "" && !strings.ContainsAny(t, "[]*.") && t == exportName(t)
}

// pickResponse is the first 2xx that carries content.
func pickResponse(resp map[string]*Response) (string, *MediaType) {
	codes := make([]string, 0, len(resp))
	for c := range resp {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		if !strings.HasPrefix(c, "2") {
			continue
		}
		r := resp[c]
		if r == nil || len(r.Content) == 0 {
			continue
		}
		if mt, ok := r.Content["application/json"]; ok {
			return "application/json", mt
		}
		types := make([]string, 0, len(r.Content))
		for t := range r.Content {
			types = append(types, t)
		}
		sort.Strings(types)
		for _, t := range types {
			if strings.Contains(t, "json") {
				return t, r.Content[t]
			}
		}
		return types[0], r.Content[types[0]]
	}
	return "", nil
}

// firstSentence is the doc comment of a method.
func firstSentence(summary, description string) string {
	s := summary
	if s == "" {
		s = description
	}
	return clip(strings.TrimSpace(strings.ReplaceAll(s, "\n", " ")), 110)
}

// argName is a path parameter as a Go argument.
func argName(raw string) string {
	n := exportName(raw)
	if n == "" {
		return "arg"
	}
	lower := strings.ToLower(n[:1]) + n[1:]
	switch lower {
	case "type", "range", "func", "map", "len", "cap", "new", "make", "select", "go", "chan", "var", "const", "package", "import", "return", "interface", "default", "case", "if", "for", "switch", "string", "int", "error", "ctx":
		return lower + "_"
	}
	return lower
}

// buf is a tiny helper so the emission below reads as text, not as calls.
type buf struct{ bytes.Buffer }

func (b *buf) p(format string, args ...any) {
	if len(args) == 0 {
		b.WriteString(format)
	} else {
		fmt.Fprintf(&b.Buffer, format, args...)
	}
	b.WriteByte('\n')
}
