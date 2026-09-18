package trilha

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Config.Locales decides which language a request is in; the Catalog is what
// the application says in it. The file is a flat JSON object per locale,
// embedded with embed so a build is still a single binary:
//
//	//go:embed i18n
//	var messages embed.FS
//
//	cat, err := trilha.LoadCatalog(messages, "i18n")
//	cat.Fallback = map[string]string{"ht": "fr"}
//	cfg.Catalog = cat
//
// A value is a string, or an object with "one" and "other" for a count:
//
//	{
//	  "protocolo.recebido": "Protocol %s received",
//	  "protocolo.pendentes": {"one": "%d pending protocol", "other": "%d pending protocols"}
//	}
//
// Nothing here tries to be CLDR: one and other is what a screen needs, and a
// language whose plural is a case of its own writes the whole sentence.
type Catalog struct {
	// Fallback says what to read when a locale has no entry for the key:
	// {"ht": "fr"} sends Haitian Creole through French before the default.
	// A locale not named here falls straight to the default, which is
	// Config.Locales[0].
	Fallback map[string]string

	msgs map[string]map[string]catMessage // locale -> key -> catMessage
}

// catMessage is one entry of the catalog: a sentence, or the two forms of a
// count.
type catMessage struct {
	other  string
	one    string
	plural bool
}

// LoadCatalog reads every <locale>.json under dir of fsys — "i18n/pt-BR.json"
// is the locale "pt-BR" — and returns the catalog they make up. It is meant
// to be called once, in Setup, over an embed.FS.
//
// A file that is not a JSON object, or an entry that is neither a string nor
// an object with "one"/"other", fails the load naming the file: a message
// nobody can read is a screen nobody can read, and it should stop the build
// and not the request.
func LoadCatalog(fsys fs.FS, dir string) (*Catalog, error) {
	if dir == "" {
		dir = "."
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("trilha: i18n catalog: %w", err)
	}
	k := &Catalog{msgs: map[string]map[string]catMessage{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := path.Join(dir, e.Name())
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("trilha: i18n catalog: %w", err)
		}
		msgs, err := parseCatalogFile(raw)
		if err != nil {
			return nil, fmt.Errorf("trilha: i18n catalog %s: %w", name, err)
		}
		k.msgs[strings.TrimSuffix(e.Name(), ".json")] = msgs
	}
	return k, nil
}

// parseCatalogFile turns one file into its messages.
func parseCatalogFile(raw []byte) (map[string]catMessage, error) {
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		return nil, fmt.Errorf("is not a JSON object of key to message: %w", err)
	}
	out := make(map[string]catMessage, len(flat))
	for key, value := range flat {
		var s string
		if err := json.Unmarshal(value, &s); err == nil {
			out[key] = catMessage{other: s}
			continue
		}
		var forms struct {
			One   string `json:"one"`
			Other string `json:"other"`
		}
		if err := json.Unmarshal(value, &forms); err != nil || forms.Other == "" {
			return nil, fmt.Errorf("key %q is neither a string nor {\"one\": …, \"other\": …}", key)
		}
		out[key] = catMessage{other: forms.Other, one: forms.One, plural: true}
	}
	return out, nil
}

// Locales lists the locales the catalog carries, sorted. It is what a tool
// asks — trilha i18n missing, a page that offers the language menu.
func (k *Catalog) Locales() []string {
	if k == nil {
		return nil
	}
	out := make([]string, 0, len(k.msgs))
	for l := range k.msgs {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// Missing returns the keys, in the order they were given, that the locale
// does not define — the fallback chain does not count here, because the
// point is to list what is still to be translated.
func (k *Catalog) Missing(locale string, keys []string) []string {
	var out []string
	var have map[string]catMessage
	if k != nil {
		have = k.msgs[locale]
	}
	for _, key := range keys {
		if _, ok := have[key]; !ok {
			out = append(out, key)
		}
	}
	return out
}

// chain is the locales to try for a request in locale: the locale itself,
// what Fallback sends it to, and the default at the end. A Fallback that
// loops stops at the locale it has already seen.
func (k *Catalog) chain(locale, def string) []string {
	out := []string{}
	seen := map[string]bool{}
	for l := locale; l != "" && !seen[l]; l = k.Fallback[l] {
		seen[l] = true
		out = append(out, l)
	}
	if def != "" && !seen[def] {
		out = append(out, def)
	}
	return out
}

// lookup renders the key in the first locale of the chain that defines it.
func (k *Catalog) lookup(locale, def, key string, args ...any) (string, bool) {
	if k == nil {
		return "", false
	}
	for _, l := range k.chain(locale, def) {
		if m, ok := k.msgs[l][key]; ok {
			return m.render(args), true
		}
	}
	return "", false
}

// render picks the form and fills the verbs. A count leads the arguments —
// c.T("carts.items", 3) — and 1 is the only "one" this does: a language that
// counts differently writes its own sentence.
func (m catMessage) render(args []any) string {
	s := m.other
	if m.plural && m.one != "" && isOne(args) {
		s = m.one
	}
	if len(args) > 0 && strings.Contains(s, "%") {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// isOne reports whether the first argument is the number 1.
func isOne(args []any) bool {
	if len(args) == 0 {
		return false
	}
	switch n := args[0].(type) {
	case int:
		return n == 1
	case int64:
		return n == 1
	case int32:
		return n == 1
	}
	return false
}

// T is the application's own message in the language of this request:
//
//	c.T("protocolo.recebido", numero)
//	c.T("protocolo.pendentes", n) // "1 pending protocol" / "4 pending protocols"
//
// The key is looked up in Ctx.Locale, then in what Catalog.Fallback sends it
// to, then in the default locale (Config.Locales[0]). A key no locale defines
// comes back as the key itself and is logged once — a screen showing
// "protocolo.recebido" is a bug you can see, and trilha check fails on it
// before anybody does.
//
// Without Config.Catalog, T returns the key: an application with a single
// language does not need a catalog to compile.
func (c *Ctx) T(key string, args ...any) string {
	k := c.app.cfg.Catalog
	if k == nil {
		return key
	}
	locale := c.Locale()
	if s, ok := k.lookup(locale, c.app.defaultLocale(), key, args...); ok {
		return s
	}
	c.app.warnOnce("i18n:"+key, "trilha: message key is in no catalog", "key", key, "locale", locale)
	return key
}
