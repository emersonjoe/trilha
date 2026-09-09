package trilha

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SchemaOption is one choice of a select field.
type SchemaOption struct{ Value, Label string }

// SchemaField is one field of a form the product defines at runtime — the step
// of a workflow, the public form behind a token, the settings of a tenant.
// The schema comes from data, so there is no Go struct to fill and no tag to
// read: what a tag would say is said here instead, and it ends up in the same
// validation and the same FieldErrors as any other form.
type SchemaField struct {
	Name     string // the input's name, and the key of its message
	Label    string
	Type     string // see SchemaTypes
	Required bool
	Help     string // the line under the field
	Min, Max string // length for a text, value for a number, date for a date
	Pattern  string // regular expression the text has to match
	Options  []SchemaOption
	Text     string // what a display field shows
	Rows     int    // rows of a textarea; 3 when zero
}

// Schema is the form itself, in the order it is shown.
type Schema []SchemaField

// SchemaTypes lists the field types a schema may ask for.
var SchemaTypes = []string{"text", "textarea", "number", "date", "datetime", "select", "checkbox", "file", "signature", "display", "password"}

// SchemaPattern is the FieldErrors message for a value that does not match the
// field's Pattern; it is also ValidationMessages["pattern"].
func init() { ValidationMessages["pattern"] = "invalid format" }

// Check reports what is wrong with the schema itself: an unknown type, a field
// with no name, a pattern that does not compile. It is a bug in the app, not
// something the person filling the form did, so it is a plain error and never
// a 422.
func (s Schema) Check() error {
	seen := map[string]bool{}
	for _, f := range s {
		if f.Type == "display" && f.Name == "" {
			continue // a paragraph in the middle of the form needs no name
		}
		if f.Name == "" {
			return errors.New("trilha: schema field with no name")
		}
		if seen[f.Name] {
			return errors.New("trilha: schema field " + f.Name + " appears twice")
		}
		seen[f.Name] = true
		if !contains(SchemaTypes, f.Type) {
			return errors.New("trilha: schema field " + f.Name + " has unknown type " + strconv.Quote(f.Type))
		}
		if f.Pattern != "" {
			if _, err := pattern(f.Pattern); err != nil {
				return errors.New("trilha: schema field " + f.Name + ": " + err.Error())
			}
		}
	}
	return nil
}

// BindSchema reads a form defined by data. It answers the values as text —
// the schema came from a table, so there is no Go type to convert them to, and
// converting to any would only move the conversion into the app — plus the
// FieldErrors of the same validation every other form goes through. A field
// the schema calls display is not read and never receives a message; a file is
// read by c.File, as any file is.
//
//	values, err := trilha.BindSchema(c, schema)
//	errs, ok := err.(trilha.FieldErrors)
//	if err != nil && !ok { return err }
//	if ok { return c.Render(422, ui.SchemaForm(schema, values, errs)) }
func BindSchema(c *Ctx, s Schema) (map[string]string, error) {
	if err := s.Check(); err != nil {
		return nil, err
	}
	if err := c.parseForm(); err != nil {
		return nil, err
	}
	values := make(map[string]string, len(s))
	errs := FieldErrors{}
	vn := &validation{}
	for _, f := range s {
		if f.Type == "display" || f.Type == "file" {
			continue
		}
		text := strings.TrimSpace(first(c.r.Form[f.Name]))
		if f.Type == "checkbox" {
			text = checked(text)
		}
		values[f.Name] = text
		value, err := schemaValue(f.Type, text)
		if err != nil {
			errs.Add(f.Name, BindInvalid)
			vn.markBad(f.Name)
			continue
		}
		vn.fields = append(vn.fields, boundField{name: f.Name, tag: schemaTag(f), value: value})
	}
	vn.run(errs)
	// oneof and pattern are checked here and not in the tag: an option and a
	// regular expression can both hold a comma, which a tag cannot.
	for _, f := range s {
		if errs.Has(f.Name) {
			continue
		}
		text, ok := values[f.Name]
		if !ok || text == "" {
			continue
		}
		if f.Type == "select" && len(f.Options) > 0 && !hasOption(f.Options, text) {
			errs.Add(f.Name, message("oneof", ""))
		}
		if f.Pattern != "" {
			re, err := pattern(f.Pattern)
			if err == nil && !re.MatchString(text) {
				errs.Add(f.Name, message("pattern", f.Pattern))
			}
		}
	}
	return values, errs.OrNil()
}

// schemaTag is what the field would have said in a struct tag.
func schemaTag(f SchemaField) string {
	var parts []string
	if f.Required {
		parts = append(parts, "required")
	}
	if f.Min != "" {
		parts = append(parts, "min="+f.Min)
	}
	if f.Max != "" {
		parts = append(parts, "max="+f.Max)
	}
	return strings.Join(parts, ",")
}

// schemaValue is the value the rules see: a number compares as a number and a
// date as a date, so min and max mean what the schema meant by them.
func schemaValue(kind, text string) (any, error) {
	switch kind {
	case "number":
		if text == "" {
			return float64(0), nil
		}
		return strconv.ParseFloat(strings.ReplaceAll(text, ",", "."), 64)
	case "date", "datetime":
		if text == "" {
			return time.Time{}, nil
		}
		for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339} {
			if t, err := time.Parse(layout, text); err == nil {
				return t, nil
			}
		}
		return nil, errors.New("bad time")
	case "checkbox":
		return text == "true", nil
	}
	return text, nil
}

var patterns sync.Map // string -> *regexp.Regexp

// pattern compiles once and keeps it: a schema is read on every request and
// the expression inside it does not change between them.
func pattern(expr string) (*regexp.Regexp, error) {
	if re, ok := patterns.Load(expr); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	patterns.Store(expr, re)
	return re, nil
}

func hasOption(opts []SchemaOption, v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func first(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// checked normalises what a checkbox sends into the one word a schema value
// carries: a browser sends "on", a fetch may send "1" or "true", and an
// unchecked box sends nothing at all.
func checked(s string) string {
	switch strings.ToLower(s) {
	case "", "0", "false", "off", "no", "nao", "não":
		return ""
	}
	return "true"
}
