package trilha

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// The configuration an administrator changes without a deploy is a pattern
// every application writes by hand: a settings table, a GET that answers JSON,
// a PUT, a screen with one form per section — and validation in none of them.
// Here the struct is the configuration, and the screen comes from it.

// SettingsStore is where a section is kept. Two methods, because the decision
// an application makes is "which table", not "which shape": the value is the
// JSON of the struct, and the key is the section's name.
//
// Nil — the default — keeps the section in memory, which is what a test wants
// and what a first version can ship with; the log says so once, because
// configuration that quietly forgets on restart is a bad surprise to have in
// production.
type SettingsStore interface {
	Load(key string) ([]byte, bool)
	Save(key string, data []byte) error
}

// Settings is one section of configuration: a struct with defaults, kept
// wherever the app says, validated by the same rules a form uses.
//
//	type Pipeline struct {
//		OCR      string `json:"ocr"      form:"ocr"      validate:"required,oneof=auto always never" label:"OCR"`
//		Parallel int    `json:"parallel" form:"parallel" validate:"min=1,max=8" label:"Documents in parallel"`
//	}
//
//	var Cfg = trilha.NewSettings("pipeline", Pipeline{OCR: "auto", Parallel: 2})
//
//	func Setup(a *trilha.App) error { return Cfg.Bind(a, store) }
//
//	p := Cfg.Get()          // a copy, safe from any goroutine
//	return Cfg.Update(c)    // bind, validate, save, audit, redirect
type Settings[T any] struct {
	key string
	def T

	mu     sync.RWMutex
	cur    T
	loaded bool
	store  SettingsStore
	app    *App
	memory map[string][]byte
}

// NewSettings declares a section and its defaults. It is a package-level var
// in the application, not something built per request: the defaults are the
// answer before anybody has saved anything.
func NewSettings[T any](key string, def T) *Settings[T] {
	return &Settings[T]{key: key, def: def, cur: def}
}

// Bind attaches the section to the app and reads what was saved. Call it in
// Setup. A nil store keeps the section in memory and says so once — a
// configuration screen that forgets on restart is worth one line in the log.
func (s *Settings[T]) Bind(a *App, store SettingsStore) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app, s.store = a, store
	if store == nil {
		s.memory = map[string][]byte{}
		if a != nil {
			a.infoOnce("settings:"+s.key, "trilha: settings kept in memory; they are lost on restart",
				"section", s.key)
		}
	}
	raw, ok := s.load()
	if !ok {
		s.cur, s.loaded = s.def, true
		return nil
	}
	// A section written by an older version of the struct is not a failure:
	// the field was renamed between deploys. What is missing keeps its
	// default, which is what json.Unmarshal already does over a copy of it.
	v := s.def
	if err := json.Unmarshal(raw, &v); err != nil {
		if s.app != nil {
			s.app.warnOnce("settings:bad:"+s.key, "trilha: saved settings could not be read; using the defaults",
				"section", s.key, "err", err)
		}
		s.cur, s.loaded = s.def, true
		return nil
	}
	s.cur, s.loaded = v, true
	return nil
}

// Get is the current configuration, copied. It is safe from any goroutine, and
// what comes back is a value: changing it changes nothing until Set or Update.
func (s *Settings[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.loaded {
		return s.def
	}
	return s.cur
}

// Set saves a section the application built itself — a migration, a command,
// a test. Update is what a screen calls.
func (s *Settings[T]) Set(v T) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("trilha: settings %s: %w", s.key, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.save(raw); err != nil {
		return fmt.Errorf("trilha: settings %s: %w", s.key, err)
	}
	s.cur, s.loaded = v, true
	return nil
}

// Update is the whole POST of a settings screen: read the form, validate it
// with the struct's own rules, save it, write who changed what to the audit
// trail, and answer.
//
//	func POST(c *trilha.Ctx) error { return config.Cfg.Update(c) }
//
// A 422 comes back as FieldErrors, so the screen redraws with the messages
// beside the fields and **nothing is saved**. What passes is saved and the
// answer is a redirect to the same address, which is what keeps a reload from
// posting the form again.
//
// The audit line carries the names of the fields that changed and not their
// values: a settings page is where a token or a password lives, and a trail
// that copies them is a second place to leak them from.
func (s *Settings[T]) Update(c *Ctx) error {
	v := s.Get()
	if err := c.Bind(&v); err != nil {
		return err
	}
	before := s.Get()
	if err := s.Set(v); err != nil {
		return err
	}
	if fields := changedFields(before, v); len(fields) > 0 {
		c.Audit("settings."+s.key, s.key, Fields{"fields": strings.Join(fields, ",")})
	}
	c.Flash("success", "Saved.")
	return c.Redirect(c.Request().URL.Path)
}

// Schema is the form of this section, derived from the struct.
func (s *Settings[T]) Schema() Schema { return SchemaOf[T]() }

// Key is the section's name, as stored.
func (s *Settings[T]) Key() string { return s.key }

// Values is what the form shows: the current configuration, as the strings a
// form field holds.
func (s *Settings[T]) Values() map[string]string { return schemaValues(s.Get()) }

func (s *Settings[T]) load() ([]byte, bool) {
	if s.store != nil {
		return s.store.Load(s.key)
	}
	b, ok := s.memory[s.key]
	return b, ok
}

func (s *Settings[T]) save(raw []byte) error {
	if s.store != nil {
		return s.store.Save(s.key, raw)
	}
	if s.memory == nil {
		s.memory = map[string][]byte{}
	}
	s.memory[s.key] = raw
	return nil
}

// changedFields names what is different between two versions, and never says
// what the new value is.
func changedFields(a, b any) []string {
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	if av.Kind() != reflect.Struct {
		return nil
	}
	var out []string
	for i := 0; i < av.NumField(); i++ {
		if !av.Type().Field(i).IsExported() {
			continue
		}
		if !reflect.DeepEqual(av.Field(i).Interface(), bv.Field(i).Interface()) {
			out = append(out, fieldName(av.Type().Field(i)))
		}
	}
	return out
}

// SchemaOf builds the form of a struct: one field per exported field, with the
// label, the help and the rules it already declares.
//
//	trilha.SchemaOf[Pipeline]()
//
// The input's name is the form tag, or the field's name — the same name Bind
// reads, because the form this describes is the form Bind fills. The json tag
// names what is stored and does not have to agree with it.
//
// What the tags become: label and help are the label and the line under it;
// "required" marks the field; "oneof" makes it a select; min and max become
// the length of a text or the range of a number; "pattern" travels as it is.
// A bool is a checkbox, a number is a number, everything else is text.
//
// A field of a kind no form can hold — a map, a slice, a nested struct — is a
// programming error and panics, because the alternative is a screen that
// silently cannot edit part of its own configuration.
func SchemaOf[T any]() Schema {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil || t.Kind() != reflect.Struct {
		panic("trilha: SchemaOf needs a struct")
	}
	var out Schema
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Tag.Get("form") == "-" {
			continue
		}
		out = append(out, schemaFieldOf(t, f))
	}
	return out
}

func schemaFieldOf(t reflect.Type, f reflect.StructField) SchemaField {
	sf := SchemaField{
		Name:  fieldName(f),
		Label: f.Tag.Get("label"),
		Help:  f.Tag.Get("help"),
		Type:  "text",
	}
	if sf.Label == "" {
		sf.Label = f.Name
	}
	switch f.Type.Kind() {
	case reflect.Bool:
		sf.Type = "checkbox"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		sf.Type = "number"
	case reflect.String:
	default:
		panic(fmt.Sprintf("trilha: SchemaOf cannot make a field of %s.%s (%s); a settings struct holds strings, numbers and booleans",
			t.Name(), f.Name, f.Type))
	}
	for _, rule := range strings.Split(f.Tag.Get("validate"), ",") {
		name, param, _ := strings.Cut(strings.TrimSpace(rule), "=")
		switch name {
		case "required":
			sf.Required = true
		case "min":
			sf.Min = param
		case "max":
			sf.Max = param
		case "pattern":
			sf.Pattern = param
		case "oneof":
			sf.Type = "select"
			for _, v := range strings.Fields(param) {
				sf.Options = append(sf.Options, SchemaOption{Value: v, Label: v})
			}
		}
	}
	return sf
}

// fieldName is the name the form uses, which is the name Bind reads.
func fieldName(f reflect.StructField) string {
	if n := f.Tag.Get("form"); n != "" && n != "-" {
		return n
	}
	return f.Name
}

// schemaValues turns the configuration into what the form shows.
func schemaValues(v any) map[string]string {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct {
		return nil
	}
	out := map[string]string{}
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		fv := rv.Field(i)
		switch fv.Kind() {
		case reflect.Bool:
			if fv.Bool() {
				out[fieldName(f)] = "on"
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			out[fieldName(f)] = strconv.FormatInt(fv.Int(), 10)
		case reflect.Float32, reflect.Float64:
			out[fieldName(f)] = strconv.FormatFloat(fv.Float(), 'f', -1, 64)
		case reflect.String:
			out[fieldName(f)] = fv.String()
		}
	}
	return out
}
