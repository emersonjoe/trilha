package trilha

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// BindInvalid is the FieldErrors message Bind records when a value cannot be
// converted to the field's type. Change it to localise.
var BindInvalid = "invalid value"

// Bind fills a struct from the request: JSON when the Content-Type is
// application/json (see BindJSON), otherwise form fields and query string.
// Fields are matched by the `form:"name"` tag (or the field name). Supported
// types: string, []string, bool (checkbox: on/true/1), int, int64, float64,
// time.Time (2006-01-02 or 2006-01-02T15:04) and pointers to them (nil when
// absent). A nested struct is flattened: its fields are read with the
// struct's tag as prefix (`Cobranca Endereco `+"`"+`form:"cob_"`+"`"+` reads cob_cep...),
// or with no prefix when it has no tag. Values that fail to convert are
// reported as FieldErrors, after every field was tried, so all messages
// reach the user at once.
//
//	var in struct {
//		Nome  string  `form:"nome"`
//		Idade int     `form:"idade"`
//		Ativo bool    `form:"ativo"`
//	}
//	if err := c.Bind(&in); err != nil { return err }
func (c *Ctx) Bind(v any) error {
	if strings.HasPrefix(c.r.Header.Get("Content-Type"), "application/json") {
		return c.BindJSON(v)
	}
	if err := c.parseForm(); err != nil {
		return err
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("trilha: Bind needs a pointer to a struct, got %T", v)
	}
	return validated(v, rv, c.r.Form)
}

// validated walks the struct, applies the validate tags and returns what the
// handler sees: FieldErrors, or nil. A nil form means the values are already
// in place (JSON), so the walk only collects them.
func validated(v any, rv reflect.Value, form map[string][]string) error {
	errs := FieldErrors{}
	vn := &validation{}
	bindStruct(rv.Elem(), "", form, errs, vn)
	vn.run(errs)
	if err := validateWhole(v, errs); err != nil {
		return err
	}
	return errs.OrNil()
}

func bindStruct(sv reflect.Value, prefix string, form map[string][]string, errs FieldErrors, vn *validation) {
	st := sv.Type()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := sv.Field(i)
		name := f.Tag.Get("form")
		if form == nil {
			// JSON: the field is named the way the client sent it, so the
			// message comes back under a key the caller recognises.
			if j, _, _ := strings.Cut(f.Tag.Get("json"), ","); j != "" {
				name = j
			}
		}
		if name == "-" {
			continue
		}
		if fv.Kind() == reflect.Struct && fv.Type() != reflect.TypeOf(time.Time{}) {
			inner := prefix + name
			if form == nil && name != "" {
				inner += "."
			}
			bindStruct(fv, inner, form, errs, vn)
			continue
		}
		if name == "" {
			name = f.Name
		}
		name = prefix + name
		if vals, ok := form[name]; ok {
			if err := setField(fv, vals); err != nil {
				errs.Add(name, BindInvalid)
				vn.markBad(name)
			}
		}
		// Every field is collected, sent or not: required has to fire on what
		// nobody typed, and eqfield has to find the field it points at.
		vn.fields = append(vn.fields, boundField{
			name:  name,
			tag:   f.Tag.Get("validate"),
			value: fieldValue(fv),
			sent:  fv.Kind() == reflect.Pointer && !fv.IsNil(),
			err:   callValidator(fv),
		})
	}
}

// fieldValue is the converted value a rule sees, with the pointer opened and
// the named type behind it ("type CPF string" is a string here).
func fieldValue(fv reflect.Value) any {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return nil
		}
		fv = fv.Elem()
	}
	switch v := fv.Interface().(type) {
	case time.Time:
		return v
	case []string:
		return v
	}
	switch fv.Kind() {
	case reflect.String:
		return fv.String()
	case reflect.Bool:
		return fv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fv.Int()
	case reflect.Float32, reflect.Float64:
		return fv.Float()
	}
	return nil
}

// callValidator asks the field's own type whether the value serves. The
// address is tried too, because a Validate with a pointer receiver is just as
// common as one with a value receiver.
func callValidator(fv reflect.Value) error {
	if fv.Kind() == reflect.Pointer && fv.IsNil() {
		return nil
	}
	if v, ok := fv.Interface().(Validator); ok {
		return v.Validate()
	}
	if fv.Kind() != reflect.Pointer && fv.CanAddr() {
		if v, ok := fv.Addr().Interface().(Validator); ok {
			return v.Validate()
		}
	}
	return nil
}

func setField(fv reflect.Value, vals []string) error {
	if fv.Kind() == reflect.Pointer {
		if len(vals) == 0 || (len(vals) == 1 && vals[0] == "") {
			return nil
		}
		p := reflect.New(fv.Type().Elem())
		if err := setField(p.Elem(), vals); err != nil {
			return err
		}
		fv.Set(p)
		return nil
	}
	s := ""
	if len(vals) > 0 {
		s = strings.TrimSpace(vals[0])
	}
	switch fv.Interface().(type) {
	case time.Time:
		if s == "" {
			return nil
		}
		for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339} {
			if t, err := time.Parse(layout, s); err == nil {
				fv.Set(reflect.ValueOf(t))
				return nil
			}
		}
		return fmt.Errorf("bad time")
	case []string:
		fv.Set(reflect.ValueOf(append([]string(nil), vals...)))
		return nil
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(s)
	case reflect.Bool:
		switch strings.ToLower(s) {
		case "", "0", "false", "off", "no", "nao", "não":
			fv.SetBool(false)
		case "1", "true", "on", "yes", "sim":
			fv.SetBool(true)
		default:
			return fmt.Errorf("bad bool")
		}
	case reflect.Int, reflect.Int64, reflect.Int32:
		if s == "" {
			fv.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Float64, reflect.Float32:
		if s == "" {
			fv.SetFloat(0)
			return nil
		}
		n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	default:
		return fmt.Errorf("unsupported type %s", fv.Type())
	}
	return nil
}
