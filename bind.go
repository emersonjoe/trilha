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
var BindInvalid = "valor inválido"

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
	errs := FieldErrors{}
	bindStruct(rv.Elem(), "", c.r.Form, errs)
	return errs.OrNil()
}

func bindStruct(sv reflect.Value, prefix string, form map[string][]string, errs FieldErrors) {
	st := sv.Type()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := sv.Field(i)
		name := f.Tag.Get("form")
		if name == "-" {
			continue
		}
		if fv.Kind() == reflect.Struct && fv.Type() != reflect.TypeOf(time.Time{}) {
			bindStruct(fv, prefix+name, form, errs)
			continue
		}
		if name == "" {
			name = f.Name
		}
		name = prefix + name
		vals, ok := form[name]
		if !ok {
			continue
		}
		if err := setField(fv, vals); err != nil {
			errs.Add(name, BindInvalid)
		}
	}
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
