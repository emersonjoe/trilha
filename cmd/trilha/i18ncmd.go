package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// An application that answers in five languages keeps its sentences in
// i18n/<locale>.json and asks for them with c.T("key"). Two questions come
// up from the first day: which keys does the code use, and which of them a
// translation still lacks. Both are answered by reading the code, which is
// the only place that knows.

// i18nCall finds c.T("key"), s.T(`key`) and anything else that calls .T with
// a literal first argument. A key built at runtime is invisible here, and
// that is the trade the regexp makes: no build, no types, no dependency.
var i18nCall = regexp.MustCompile("\\.T\\(\\s*(\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`)")

// i18nLocalesLine reads the default locale out of app/setup.go:
// cfg.Locales = []string{"pt-BR", "ht"} — the first one is the default.
var i18nLocalesLine = regexp.MustCompile(`Locales\s*=\s*\[\]string\{\s*"([^"]+)"`)

// i18nScanDirs are where an application's own code lives.
var i18nScanDirs = []string{"app", "internal"}

// i18nDir is where the catalog lives. embed only reaches downwards, so the
// folder sits next to the package that embeds it — app/i18n for the usual
// //go:embed i18n in app/setup.go — and a project that keeps it at the root
// works too. Whichever exists wins; when neither does, a write creates
// app/i18n, which is the one that can be embedded.
func i18nDir(root string) string {
	for _, rel := range []string{filepath.Join("app", "i18n"), "i18n"} {
		if st, err := os.Stat(filepath.Join(root, rel)); err == nil && st.IsDir() {
			return rel
		}
	}
	return filepath.Join("app", "i18n")
}

// i18nFile is the catalog file of one locale, and the path a report prints.
func i18nFile(root, locale string) (abs, rel string) {
	rel = path.Join(filepath.ToSlash(i18nDir(root)), locale+".json")
	return filepath.Join(root, filepath.FromSlash(rel)), rel
}

func cmdI18n(args []string) error {
	if len(args) == 0 {
		return errors.New(t("i18n usage"))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	switch args[0] {
	case "extract":
		write := false
		for _, a := range args[1:] {
			if a != "--write" && a != "-write" {
				return errors.New(t("i18n usage"))
			}
			write = true
		}
		return i18nExtract(p.Root, write)
	case "missing":
		if len(args) != 2 {
			return errors.New(t("i18n usage"))
		}
		return i18nMissing(p.Root, args[1])
	default:
		return errors.New(t("i18n usage"))
	}
}

// i18nExtract prints every key the code uses, and with --write adds the ones
// the default locale does not define yet — with the key itself as the text,
// so the screen says something readable while the translation is written.
func i18nExtract(root string, write bool) error {
	keys, err := i18nKeys(root)
	if err != nil {
		return err
	}
	if !write {
		for _, k := range keys {
			fmt.Println(k)
		}
		return nil
	}
	locale := i18nDefaultLocale(root)
	abs, rel := i18nFile(root, locale)
	cur, err := i18nRead(abs)
	if err != nil {
		return err
	}
	added := 0
	for _, k := range keys {
		if _, ok := cur[k]; !ok {
			cur[k] = k
			added++
		}
	}
	if added > 0 {
		if err := i18nWrite(abs, cur); err != nil {
			return err
		}
	}
	fmt.Printf(t("i18n extracted"), len(keys), added, rel)
	return nil
}

// i18nMissing prints the keys the code uses and the locale does not define,
// and fails when there is any: it is meant to be a step of a pipeline.
func i18nMissing(root, locale string) error {
	keys, err := i18nKeys(root)
	if err != nil {
		return err
	}
	abs, _ := i18nFile(root, locale)
	have, err := i18nRead(abs)
	if err != nil {
		return err
	}
	var missing []string
	for _, k := range keys {
		if _, ok := have[k]; !ok {
			missing = append(missing, k)
		}
	}
	for _, k := range missing {
		fmt.Println(k)
	}
	if len(missing) > 0 {
		return fmt.Errorf(t("i18n missing"), len(missing), locale)
	}
	fmt.Printf(t("i18n complete"), locale, len(keys))
	return nil
}

// i18nKeys is every key the project's code asks for, sorted and without
// repeats.
func i18nKeys(root string) ([]string, error) {
	seen := map[string]bool{}
	for _, dir := range i18nScanDirs {
		base := filepath.Join(root, dir)
		if st, err := os.Stat(base); err != nil || !st.IsDir() {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range i18nCall.FindAllStringSubmatch(string(src), -1) {
				if key := i18nLiteral(m[1]); key != "" {
					seen[key] = true
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// i18nLiteral unquotes the literal the regexp caught, in either of the two
// spellings Go has for a string.
func i18nLiteral(lit string) string {
	if strings.HasPrefix(lit, "`") {
		return strings.Trim(lit, "`")
	}
	s, err := strconv.Unquote(lit)
	if err != nil {
		return ""
	}
	return s
}

// i18nDefaultLocale is the first locale of Config.Locales in app/setup.go,
// and "en" when the project never said.
func i18nDefaultLocale(root string) string {
	src, err := os.ReadFile(filepath.Join(root, "app", "setup.go"))
	if err == nil {
		if m := i18nLocalesLine.FindSubmatch(src); m != nil {
			return string(m[1])
		}
	}
	return "en"
}

// i18nCatalogLocales lists the locales i18n/ carries, sorted.
func i18nCatalogLocales(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, i18nDir(root)))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(out)
	return out
}

// i18nRead loads one catalog file as key → raw value. A file that is not
// there yet is an empty catalog: the first extract --write creates it.
func i18nRead(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(path), err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// i18nWrite writes the catalog back, indented and with the keys in order so
// that a diff shows the translation and not the shuffle.
func i18nWrite(path string, msgs map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(msgs))
	for k := range msgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("{\n")
	for i, k := range keys {
		name, err := json.Marshal(k)
		if err != nil {
			return err
		}
		value, err := json.Marshal(msgs[k])
		if err != nil {
			return err
		}
		sb.Write(name)
		sb.WriteString(": ")
		sb.Write(value)
		if i < len(keys)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// checkStepI18n only speaks when the project keeps an i18n/ folder. A key the
// code asks for and the default locale does not have is a failure — it is the
// screen showing "protocolo.recebido" to somebody — and a key another locale
// has not translated yet is a line to read, not a gate: a translation always
// arrives after the code that needs it.
func checkStepI18n(p *project, _ bool) (string, []problem) {
	locales := i18nCatalogLocales(p.Root)
	if len(locales) == 0 {
		return statusSkipped, nil
	}
	keys, err := i18nKeys(p.Root)
	if err != nil {
		return statusFailed, []problem{{Tool: "i18n", Message: err.Error()}}
	}
	def := i18nDefaultLocale(p.Root)
	var problems []problem
	failed := false
	// The default first, because it is the one that fails; then the others
	// in order, each one line.
	order := []string{def}
	for _, l := range locales {
		if l != def {
			order = append(order, l)
		}
	}
	for _, locale := range order {
		abs, rel := i18nFile(p.Root, locale)
		have, err := i18nRead(abs)
		if err != nil {
			return statusFailed, []problem{{Tool: "i18n", File: rel, Message: err.Error()}}
		}
		var missing []string
		for _, k := range keys {
			if _, ok := have[k]; !ok {
				missing = append(missing, k)
			}
		}
		if len(missing) == 0 {
			continue
		}
		msg := fmt.Sprintf(t("i18n keys missing"), len(missing), strings.Join(firstKeys(missing, 3), ", "))
		if locale == def {
			failed = true
			problems = append(problems, problem{Tool: "i18n", File: rel,
				Message: msg, Fix: t("fix i18n")})
			continue
		}
		problems = append(problems, problem{Tool: "i18n", File: rel,
			Message: t("warning prefix") + msg, Fix: fmt.Sprintf(t("fix i18n locale"), locale)})
	}
	if failed {
		return statusFailed, problems
	}
	return statusOK, problems
}

// firstKeys is what fits on the line; the whole list is one command away
// (trilha i18n missing <locale>).
func firstKeys(keys []string, n int) []string {
	if len(keys) <= n {
		return keys
	}
	return append(append([]string{}, keys[:n]...), "…")
}
