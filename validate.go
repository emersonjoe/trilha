package trilha

import (
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Validator is a type that checks itself. Bind calls it on a field, right
// after the value was converted, and on the whole struct at the end — the
// second one only when every field passed, because a check that reads two
// fields needs both of them to be there.
//
//	type CPF string
//
//	func (c CPF) Validate() error {
//		if !cpfValido(string(c)) {
//			return errors.New("CPF inválido")
//		}
//		return nil
//	}
//
// The error message goes to FieldErrors as it is, so write it in the language
// of the app. A struct's Validate may return FieldErrors to say which field is
// at fault; any other error is returned by Bind untouched.
type Validator interface{ Validate() error }

// Field is what a validation rule sees. Value is the converted value (string,
// bool, int64, float64, time.Time, []string, or nil when the field was not
// sent), and Text is the same thing as text — which is all most rules need.
type Field struct {
	Name  string // the form name, prefix included ("cob_cep")
	Param string // what came after "=" in the tag ("3" in min=3)
	Text  string // the value as text
	Value any    // the converted value

	sent bool
	all  map[string]string
}

// Other returns another field's value as text, for a rule that compares two
// fields. Unknown names give "".
func (f Field) Other(name string) string { return f.all[name] }

// rule reports whether the value passes and, when it does not, which message
// answers it — one rule can have more than one message ("min" on a number is
// not the same sentence as "min" on a text).
type rule func(Field) (bool, string)

var (
	rulesMu sync.RWMutex
	rules   = map[string]rule{
		"required": ruleRequired,
		"min":      func(f Field) (bool, string) { return minmax(f, true) },
		"max":      func(f Field) (bool, string) { return minmax(f, false) },
		"len":      ruleLen,
		"email":    ruleEmail,
		"url":      ruleURL,
		"oneof":    ruleOneOf,
		"eqfield":  ruleEqField,
	}
)

// AddRule registers a rule for the validate tag, usually in Setup. It panics
// on a name that already exists: two meanings for one word in a tag is a bug
// nobody would find later.
//
//	trilha.AddRule("cep", func(f trilha.Field) bool { return cepValido(f.Text) })
//	trilha.ValidationMessages["cep"] = "CEP inválido"
func AddRule(name string, fn func(Field) bool) {
	rulesMu.Lock()
	defer rulesMu.Unlock()
	if _, ok := rules[name]; ok {
		panic("trilha: validation rule " + name + " is already registered")
	}
	rules[name] = func(f Field) (bool, string) { return fn(f), name }
}

func lookupRule(name string) rule {
	rulesMu.RLock()
	r, ok := rules[name]
	rulesMu.RUnlock()
	if !ok {
		// A tag nobody registered would otherwise pass silently, and the form
		// would accept anything in production.
		panic("trilha: unknown validation rule " + name)
	}
	return r
}

// ValidationMessages is the message of each rule, in English. Change an entry,
// swap the map, or call UseValidationPTBR; "{param}" is replaced by what came
// after the "=" in the tag.
var ValidationMessages = map[string]string{
	"required": "required",
	"min":      "must be {param} or more",
	"max":      "must be {param} or less",
	"minlen":   "must have at least {param} characters",
	"maxlen":   "must have at most {param} characters",
	"minitems": "choose at least {param}",
	"maxitems": "choose at most {param}",
	"mindate":  "must not be before {param}",
	"maxdate":  "must not be after {param}",
	"len":      "must have exactly {param} characters",
	"lenitems": "choose exactly {param}",
	"email":    "invalid e-mail",
	"url":      "invalid URL",
	"oneof":    "invalid option",
	"eqfield":  "does not match",
}

// UseValidationPTBR switches the validation messages, BindInvalid included, to
// Brazilian Portuguese. Call it in Setup. These messages are read by the
// person filling the form, not by the developer, which is why they are the one
// piece of the runtime that comes translated.
func UseValidationPTBR() {
	ValidationMessages = map[string]string{
		"required": "obrigatório",
		"min":      "precisa ser {param} ou mais",
		"max":      "precisa ser {param} ou menos",
		"minlen":   "precisa ter ao menos {param} caracteres",
		"maxlen":   "precisa ter no máximo {param} caracteres",
		"minitems": "escolha ao menos {param}",
		"maxitems": "escolha no máximo {param}",
		"mindate":  "não pode ser antes de {param}",
		"maxdate":  "não pode ser depois de {param}",
		"len":      "precisa ter exatamente {param} caracteres",
		"lenitems": "escolha exatamente {param}",
		"email":    "e-mail inválido",
		"url":      "URL inválida",
		"oneof":    "opção inválida",
		"eqfield":  "não confere",
	}
	BindInvalid = "valor inválido"
}

func message(key, param string) string {
	m, ok := ValidationMessages[key]
	if !ok {
		m = key
	}
	return strings.ReplaceAll(m, "{param}", param)
}

// validation collects what Bind walked, so the rules run over the whole struct
// after every field was converted — eqfield has to see a field that comes
// later in the declaration.
type validation struct {
	fields []boundField
	bad    map[string]bool
}

type boundField struct {
	name  string
	tag   string
	value any
	sent  bool  // a non-nil pointer: the field came, whatever it holds
	err   error // from the field type's own Validator
}

func (v *validation) markBad(name string) {
	if v.bad == nil {
		v.bad = map[string]bool{}
	}
	v.bad[name] = true
}

func (v *validation) run(errs FieldErrors) {
	text := make(map[string]string, len(v.fields))
	for _, f := range v.fields {
		text[f.name] = textOf(f.value)
	}
	for _, f := range v.fields {
		if v.bad[f.name] || errs.Has(f.name) {
			continue // it did not even convert: one message per field is enough
		}
		fl := Field{Name: f.name, Text: text[f.name], Value: f.value, sent: f.sent, all: text}
		empty := emptyValue(f.value)
		failed := false
		for _, part := range strings.Split(f.tag, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, param, _ := strings.Cut(part, "=")
			if name != "required" && empty {
				continue // an optional field only answers for what was filled in
			}
			fl.Param = param
			if ok, key := lookupRule(name)(fl); !ok {
				errs.Add(f.name, message(key, param))
				failed = true
				break
			}
		}
		if !failed && !empty && f.err != nil {
			errs.Add(f.name, f.err.Error())
		}
	}
}

// validateWhole runs the struct's own check, once the fields are all good.
func validateWhole(v any, errs FieldErrors) error {
	if errs.Any() {
		return nil
	}
	sv, ok := v.(Validator)
	if !ok {
		return nil
	}
	err := sv.Validate()
	if err == nil {
		return nil
	}
	if fe, ok := err.(FieldErrors); ok {
		for k, m := range fe {
			errs.Add(k, m)
		}
		return nil
	}
	return err
}

// A pointer field that arrived holds an answer even when the answer is zero:
// that is the whole reason to declare it as a pointer.
func ruleRequired(f Field) (bool, string) { return f.sent || !emptyValue(f.Value), "required" }

func minmax(f Field, isMin bool) (bool, string) {
	switch v := f.Value.(type) {
	case string:
		n := intParam(f)
		if l := utf8.RuneCountInString(v); isMin {
			return l >= n, "minlen"
		} else {
			return l <= n, "maxlen"
		}
	case []string:
		n := intParam(f)
		if isMin {
			return len(v) >= n, "minitems"
		}
		return len(v) <= n, "maxitems"
	case int64:
		n := int64(intParam(f))
		if isMin {
			return v >= n, "min"
		}
		return v <= n, "max"
	case float64:
		n := floatParam(f)
		if isMin {
			return v >= n, "min"
		}
		return v <= n, "max"
	case time.Time:
		d := dateParam(f)
		if isMin {
			return !v.Before(d), "mindate"
		}
		return !v.After(d), "maxdate"
	}
	return true, ""
}

func ruleLen(f Field) (bool, string) {
	switch v := f.Value.(type) {
	case string:
		return utf8.RuneCountInString(v) == intParam(f), "len"
	case []string:
		return len(v) == intParam(f), "lenitems"
	}
	return true, ""
}

func ruleEmail(f Field) (bool, string) {
	s := f.Text
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 || strings.ContainsAny(s, " \t\r\n,;") {
		return false, "email"
	}
	domain := s[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1 && !strings.Contains(domain, ".."), "email"
}

func ruleURL(f Field) (bool, string) {
	u, err := url.Parse(f.Text)
	if err != nil {
		return false, "url"
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != "", "url"
}

func ruleOneOf(f Field) (bool, string) {
	for _, opt := range strings.Fields(f.Param) {
		if opt == f.Text {
			return true, "oneof"
		}
	}
	return false, "oneof"
}

func ruleEqField(f Field) (bool, string) { return f.Other(f.Param) == f.Text, "eqfield" }

// A parameter that is not a number is a typo in the tag, not something a user
// did: it fails on the first request, in development, like the tag itself.
func intParam(f Field) int {
	n, err := strconv.Atoi(f.Param)
	if err != nil {
		panic("trilha: validate tag on " + f.Name + ": bad number in " + f.Param)
	}
	return n
}

func floatParam(f Field) float64 {
	n, err := strconv.ParseFloat(f.Param, 64)
	if err != nil {
		panic("trilha: validate tag on " + f.Name + ": bad number in " + f.Param)
	}
	return n
}

func dateParam(f Field) time.Time {
	t, err := time.Parse("2006-01-02", f.Param)
	if err != nil {
		panic("trilha: validate tag on " + f.Name + ": bad date in " + f.Param)
	}
	return t
}

// emptyValue is the zero of the type — which is what required asks about. A
// number that may legitimately be 0 travels as a pointer, the same way Bind
// already tells "not sent" from "sent".
func emptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case int64:
		return t == 0
	case float64:
		return t == 0
	case time.Time:
		return t.IsZero()
	case []string:
		return len(t) == 0
	}
	return false
}

func textOf(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case time.Time:
		return t.Format("2006-01-02")
	case []string:
		return strings.Join(t, ",")
	}
	return ""
}
