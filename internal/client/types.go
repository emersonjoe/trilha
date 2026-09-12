package client

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Field is one field of a generated struct.
type Field struct {
	Name string // Go name
	Type string
	Tag  string
	Doc  string
}

// Struct is a generated type.
type Struct struct {
	Name   string
	Doc    string
	Fields []Field
}

// Enum is a string schema with a closed list of values: a defined type plus its
// constants, so the compiler catches the value that does not exist.
type Enum struct {
	Name   string
	Doc    string
	Values []EnumValue
}

// EnumValue is one constant of an Enum.
type EnumValue struct {
	Name  string
	Value string
}

// Note is a line of the report: something the document says that the generated
// client carries as json.RawMessage, or a name the generator had to invent.
type Note struct {
	Where string
	What  string
}

// builder turns schemas into Go types. It keeps the order it created things in
// so the generated file is the same on every run, and a stack of the structs it
// is inside, which is how a $ref that closes a cycle becomes a pointer.
type builder struct {
	doc    *Doc
	byName map[string]*Struct
	order  []string
	enums  map[string]*Enum
	eOrder []string
	notes  []Note
	// untyped are the operations whose answer stayed json.RawMessage, in the
	// order the document declares them.
	untyped []string
	stack   map[string]bool
	// alias maps a component schema that is not a struct (a bare string, an
	// array) to the Go type it expands to, so a $ref to it reads naturally.
	alias map[string]string
	// reserved are the names of types the emission writes without recording
	// them as schemas: the struct of a group, the struct of a form.
	reserved map[string]bool
}

func newBuilder(d *Doc) *builder {
	return &builder{
		doc:    d,
		byName: map[string]*Struct{},
		enums:  map[string]*Enum{},
		stack:  map[string]bool{},
		alias:  map[string]string{},

		reserved: map[string]bool{},
	}
}

// taken says whether a name is already the name of a type in the file. An alias
// is not one: it expands where it is used and declares nothing.
func (b *builder) taken(name string) bool {
	if _, ok := b.byName[name]; ok {
		return true
	}
	if _, ok := b.enums[name]; ok {
		return true
	}
	return b.reserved[name]
}

// freeName is a name the generator invents — a query struct, a form — moved out
// of the way of a name that came from the document.
func (b *builder) freeName(name string) string {
	out := name
	for i := 2; b.taken(out); i++ {
		out = name + strconv.Itoa(i)
	}
	return out
}

// components turns every named schema into a type, in name order.
func (b *builder) components() {
	names := make([]string, 0, len(b.doc.Components.Schemas))
	for n := range b.doc.Components.Schemas {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		b.named(n, b.doc.Components.Schemas[n])
	}
}

// named builds the type of one component schema and returns the Go type that
// refers to it.
func (b *builder) named(name string, s *Schema) string {
	goName := exportName(name)
	if _, ok := b.byName[goName]; ok {
		return goName
	}
	if _, ok := b.enums[goName]; ok {
		return goName
	}
	if t, ok := b.alias[goName]; ok {
		return t
	}
	flat := b.flatten(s)
	switch {
	case len(flat.Properties) > 0 || (flat.TypeName() == "object" && flat.AdditionalProperties == nil):
		b.object(goName, flat, "#/components/schemas/"+name)
		return goName
	case flat.TypeName() == "string" && len(flat.Enum) > 0:
		b.enum(goName, flat)
		return goName
	default:
		// A component that is not an object and not an enum (a bare string, an
		// array of something) does not earn a defined type: it would only make
		// every assignment need a conversion. It expands where it is used.
		t := b.goType(flat, goName, "#/components/schemas/"+name)
		b.alias[goName] = t
		return t
	}
}

// object records a struct and its fields, marking itself as being built so a
// field that points back at it becomes a pointer.
func (b *builder) object(name string, s *Schema, where string) {
	st := &Struct{Name: name, Doc: doc(s)}
	b.byName[name] = st
	b.order = append(b.order, name)
	b.stack[name] = true
	defer delete(b.stack, name)

	props := make([]string, 0, len(s.Properties))
	for p := range s.Properties {
		props = append(props, p)
	}
	sort.Strings(props)
	for _, p := range props {
		ps := s.Properties[p]
		req := s.IsRequired(p)
		t := b.goType(ps, name+exportName(p), where+"."+p)
		// A struct that contains itself does not compile; a pointer to it does.
		if b.stack[t] {
			t = "*" + t
		}
		f := Field{Name: exportName(p), Type: t, Doc: doc(ps)}
		tag := "json:\"" + p
		if !req {
			tag += ",omitempty"
		}
		tag += "\""
		if v := validateTag(ps, req); v != "" {
			tag += " validate:\"" + v + "\""
		}
		f.Tag = tag
		st.Fields = append(st.Fields, f)
	}
	if s.AdditionalProperties != nil && len(s.Properties) == 0 {
		st.Fields = append(st.Fields, Field{Name: "Extra", Type: "map[string]json.RawMessage", Tag: "json:\"-\""})
	}
}

// enum records a defined string type and its constants.
func (b *builder) enum(name string, s *Schema) {
	e := &Enum{Name: name, Doc: doc(s)}
	for _, v := range s.Enum {
		str, ok := v.(string)
		if !ok {
			continue
		}
		e.Values = append(e.Values, EnumValue{Name: name + exportName(str), Value: str})
	}
	b.enums[name] = e
	b.eOrder = append(b.eOrder, name)
}

// goType is the Go type of a schema. hint names the type it has to invent when
// the schema is inline; where is what the report prints.
func (b *builder) goType(s *Schema, hint, where string) string {
	if s == nil {
		return "json.RawMessage"
	}
	if s.Ref != "" {
		n := refName(s.Ref)
		if n == "" {
			b.note(where, "external $ref "+s.Ref+" — the document does not carry it")
			return "json.RawMessage"
		}
		target := b.doc.Components.Schemas[n]
		if target == nil {
			b.note(where, "$ref to "+n+", which the document does not define")
			return "json.RawMessage"
		}
		return b.named(n, target)
	}
	if len(s.OneOf) > 0 || len(s.AnyOf) > 0 {
		// "T or null" is not a union, and in a FastAPI document it is the most
		// common shape there is: Pydantic writes every Optional[T] this way. A
		// client that hands those back as json.RawMessage makes the caller
		// marshal by hand exactly where it was supposed to help.
		if inner := nullableOf(s); inner != nil {
			return "*" + b.goType(inner, hint, where)
		}
		b.note(where, "oneOf/anyOf — carried as json.RawMessage")
		return "json.RawMessage"
	}
	s = b.flatten(s)
	switch s.TypeName() {
	case "string":
		if len(s.Enum) > 0 {
			b.enum(hint, s)
			return hint
		}
		return "string"
	case "integer":
		if s.Format == "int32" {
			return "int32"
		}
		return "int64"
	case "number":
		if s.Format == "float" {
			return "float32"
		}
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		return "[]" + b.goType(s.Items, singular(hint)+"Item", where+"[]")
	case "object", "":
		if len(s.Properties) > 0 {
			if _, ok := b.byName[hint]; !ok {
				b.object(hint, s, where)
			}
			return hint
		}
		if s.AdditionalProperties != nil {
			var inner Schema
			if err := json.Unmarshal(s.AdditionalProperties, &inner); err == nil && (inner.Ref != "" || inner.Type != nil) {
				return "map[string]" + b.goType(&inner, hint+"Value", where+"{}")
			}
			return "map[string]json.RawMessage"
		}
		if s.TypeName() == "" {
			b.note(where, "no type in the schema — carried as json.RawMessage")
		}
		return "json.RawMessage"
	}
	b.note(where, "type "+s.TypeName()+" is not one this generator knows")
	return "json.RawMessage"
}

// flatten merges an allOf into one schema. Almost every API describes
// inheritance this way and the result is a struct; translating it to embedding
// would produce the same JSON with more surprise.
func (b *builder) flatten(s *Schema) *Schema {
	if s == nil || len(s.AllOf) == 0 {
		return s
	}
	out := *s
	out.AllOf = nil
	out.Properties = map[string]*Schema{}
	for k, v := range s.Properties {
		out.Properties[k] = v
	}
	for _, part := range s.AllOf {
		p, _ := b.doc.Resolve(part)
		if p == nil {
			continue
		}
		p = b.flatten(p)
		for k, v := range p.Properties {
			if _, ok := out.Properties[k]; !ok {
				out.Properties[k] = v
			}
		}
		out.Required = append(out.Required, p.Required...)
		if out.Type == nil {
			out.Type = p.Type
		}
		if out.Description == "" {
			out.Description = p.Description
		}
	}
	sort.Strings(out.Required)
	return &out
}

func (b *builder) note(where, what string) {
	b.notes = append(b.notes, Note{Where: where, What: what})
}

// dropNotes removes the lines added since from about exactly that place. It is
// how an operation counted as untyped stops also being a note: the count says
// the same thing, for all of them at once.
func (b *builder) dropNotes(since int, where string) {
	kept := b.notes[:since]
	for _, n := range b.notes[since:] {
		if n.Where != where {
			kept = append(kept, n)
		}
	}
	b.notes = kept
}

// validateTag turns what the schema promises into the tags of spec 027, so the
// same type can be the answer of the API and the Bind of a form.
func validateTag(s *Schema, required bool) string {
	if s == nil {
		return ""
	}
	var rules []string
	if required {
		rules = append(rules, "required")
	}
	switch s.Format {
	case "email":
		rules = append(rules, "email")
	case "uri", "url":
		rules = append(rules, "url")
	}
	if s.MinLength != nil {
		rules = append(rules, "min="+strconv.Itoa(*s.MinLength))
	} else if s.Minimum != nil {
		rules = append(rules, "min="+trimFloat(*s.Minimum))
	}
	if s.MaxLength != nil {
		rules = append(rules, "max="+strconv.Itoa(*s.MaxLength))
	} else if s.Maximum != nil {
		rules = append(rules, "max="+trimFloat(*s.Maximum))
	}
	if len(s.Enum) > 0 && s.TypeName() == "string" {
		var vals []string
		for _, v := range s.Enum {
			if str, ok := v.(string); ok && !strings.ContainsAny(str, " ,") {
				vals = append(vals, str)
			}
		}
		if len(vals) == len(s.Enum) && len(vals) > 0 {
			rules = append(rules, "oneof="+strings.Join(vals, " "))
		}
	}
	return strings.Join(rules, ",")
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// doc is the one-line comment of a schema: the description, or the title when
// there is no description.
func doc(s *Schema) string {
	if s == nil {
		return ""
	}
	d := s.Description
	if d == "" {
		d = s.Title
	}
	return clip(strings.TrimSpace(strings.ReplaceAll(d, "\n", " ")), 110)
}

// clip shortens a comment to max bytes. The cut walks back to the start of a
// character: a document written in Portuguese puts an accent at byte 107 sooner
// or later, and cutting inside a UTF-8 sequence gave a file the Go parser
// refuses — the whole command failed and wrote nothing.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max - 3
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

// exportName turns anything a document can call a thing into a Go identifier:
// user-id, user_id, "user id", 2fa → UserID, UserID, UserID, N2fa. The result is
// always ASCII — see asciiFold.
func exportName(s string) string {
	var parts []string
	cur := strings.Builder{}
	var prev rune
	for _, raw := range s {
		for _, r := range asciiFold(raw) {
			switch {
			case r == '_' || r == '-' || r == ' ' || r == '.' || r == '/' || r == '[' || r == ']' || r == '{' || r == '}':
				if cur.Len() > 0 {
					parts = append(parts, cur.String())
					cur.Reset()
				}
			case unicode.IsUpper(r) && unicode.IsLower(prev):
				if cur.Len() > 0 {
					parts = append(parts, cur.String())
					cur.Reset()
				}
				cur.WriteRune(r)
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				cur.WriteRune(r)
			}
			prev = r
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	out := strings.Builder{}
	for _, p := range parts {
		if up, ok := initialisms[strings.ToLower(p)]; ok {
			out.WriteString(up)
			continue
		}
		out.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	name := out.String()
	if name == "" {
		return "X"
	}
	if unicode.IsDigit(rune(name[0])) {
		return "N" + name
	}
	return name
}

// asciiFold is the ASCII spelling of a rune: the accented letter falls to the
// letter underneath it, and anything with no Latin base is dropped, the way
// punctuation already was.
//
// A tag is the label of a page — `verificação de assinaturas` in every FastAPI
// written in Portuguese — and Go accepts a Unicode letter in an identifier, so
// the name went through whole and compiled. What it cost came later: every call
// site typing `c.VerificaçãoDeAssinaturas()` on a keyboard with no dead key, a
// grep that depends on NFC and NFD being the same bytes, and a `ç` for a `c`
// that only fails in the caller's build. The document's label stays in the
// comment, which is where it is read instead of typed.
func asciiFold(r rune) string {
	if r < utf8.RuneSelf {
		return string(r)
	}
	for _, f := range latinFolds {
		if strings.ContainsRune(f.from, r) {
			return f.to
		}
	}
	return ""
}

// latinFolds is the accented Latin alphabet — Latin-1 Supplement and Latin
// Extended-A, which is Portuguese, Spanish, French, German, Italian, Polish and
// Czech — with the letter each one falls to. The table is written by hand
// because the decomposition that would build it lives outside the standard
// library, and the constitution says the CLI has no dependencies.
var latinFolds = []struct{ to, from string }{
	{"A", "ÀÁÂÃÄÅĀĂĄ"}, {"a", "àáâãäåāăą"},
	{"AE", "Æ"}, {"ae", "æ"},
	{"C", "ÇĆĈĊČ"}, {"c", "çćĉċč"},
	{"D", "ĎĐÐ"}, {"d", "ďđð"},
	{"E", "ÈÉÊËĒĔĖĘĚ"}, {"e", "èéêëēĕėęě"},
	{"G", "ĜĞĠĢ"}, {"g", "ĝğġģ"},
	{"H", "ĤĦ"}, {"h", "ĥħ"},
	{"I", "ÌÍÎÏĨĪĬĮİ"}, {"i", "ìíîïĩīĭįı"},
	{"IJ", "Ĳ"}, {"ij", "ĳ"},
	{"J", "Ĵ"}, {"j", "ĵ"},
	{"K", "Ķ"}, {"k", "ķĸ"},
	{"L", "ĹĻĽĿŁ"}, {"l", "ĺļľŀł"},
	{"N", "ÑŃŅŇŊ"}, {"n", "ñńņňŉŋ"},
	{"O", "ÒÓÔÕÖØŌŎŐ"}, {"o", "òóôõöøōŏő"},
	{"OE", "Œ"}, {"oe", "œ"},
	{"R", "ŔŖŘ"}, {"r", "ŕŗř"},
	{"S", "ŚŜŞŠ"}, {"s", "śŝşš"}, {"ss", "ß"},
	{"T", "ŢŤŦ"}, {"t", "ţťŧ"},
	{"TH", "Þ"}, {"th", "þ"},
	{"U", "ÙÚÛÜŨŪŬŮŰŲ"}, {"u", "ùúûüũūŭůűų"},
	{"W", "Ŵ"}, {"w", "ŵ"},
	{"Y", "ÝŶŸ"}, {"y", "ýÿŷ"},
	{"Z", "ŹŻŽ"}, {"z", "źżž"},
}

// isASCII says whether a string is already the alphabet the generated file uses.
func isASCII(s string) bool {
	for _, r := range s {
		if r >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// initialisms are the words Go writes in caps. The list is short on purpose:
// every entry is a name the generator will spell differently from the document,
// so it earns its place only if the other spelling would look wrong in review.
var initialisms = map[string]string{
	"id": "ID", "url": "URL", "uri": "URI", "api": "API", "http": "HTTP",
	"https": "HTTPS", "html": "HTML", "json": "JSON", "xml": "XML", "sql": "SQL",
	"uuid": "UUID", "ip": "IP", "db": "DB", "cpf": "CPF", "cnpj": "CNPJ",
	"pdf": "PDF", "ok": "OK", "csv": "CSV",
}

// singular drops a trailing s so []Item of Documents is DocumentItem, not
// DocumentsItem. It is a cosmetic rule and only touches the invented name.
func singular(s string) string {
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// sortedNotes returns the report lines in a stable order.
func (b *builder) sortedNotes() []Note {
	out := append([]Note(nil), b.notes...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Where != out[j].Where {
			return out[i].Where < out[j].Where
		}
		return out[i].What < out[j].What
	})
	// The same schema reached twice is the same line twice.
	uniq := out[:0]
	seen := map[string]bool{}
	for _, n := range out {
		k := n.Where + "\x00" + n.What
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, n)
	}
	return uniq
}

// fmtErr keeps the error text of this package in one shape.
func fmtErr(format string, args ...any) error { return fmt.Errorf("client: "+format, args...) }

// nullableOf recognises the one shape a two-member union is allowed to have:
// exactly two members, one of them the null type. That is Optional[T] as
// Pydantic writes it, and it says "this field may be absent", not "this field
// may be one of two things". Anything else — three members, two real types — is
// a union the generator cannot name, and stays json.RawMessage.
func nullableOf(s *Schema) *Schema {
	members := s.AnyOf
	if len(members) == 0 {
		members = s.OneOf
	}
	if len(members) != 2 {
		return nil
	}
	var inner *Schema
	nulls := 0
	for _, m := range members {
		if m == nil {
			return nil
		}
		if m.Ref == "" && m.TypeName() == "null" {
			nulls++
			continue
		}
		inner = m
	}
	if nulls != 1 || inner == nil {
		return nil
	}
	return inner
}
