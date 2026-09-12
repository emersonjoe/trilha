package docs

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The public surface grows one release at a time and the site does not follow
// by itself. These two tests are the alarm: every symbol promised in
// api/current.txt shows up as code on some English page and on some Portuguese
// page, and every component of the ui kit is in the catalogue of
// reference/ui.md. They fail listing what is missing, the way
// TestNoExternalDeps lists the dependency that got in.

// apiFile is the written promise, from this package's directory.
const apiFile = "../../../api/current.txt"

// skippedPkgs are packages whose symbols are not checked one by one.
var skippedPkgs = map[string]string{
	"h": "the elements are generated from a table (h/elements.go); reference/h.md documents the DSL, not one page per tag",
}

// documentedElsewhere lists the symbols no page writes as code on purpose, with
// the reason. Keep it short: an entry here is a promise the site does not
// spell out, so the reason has to say why the reader never needs to write it.
var documentedElsewhere = map[string]string{
	"trilha.CompileErrorPage": "the compile-error page of trilha dev, rendered by internal/dev; " +
		"it is exported for the dev server and never called by an application",
}

// symbol is one declaration of api/current.txt.
type symbol struct {
	Pkg  string // "trilha", "ui", "ai/mcp"...
	Name string // "New", "App.RunShutdown", "ErrNotFound"
	Kind string // "const", "var", "func", "method", "type"
}

// ID is how the symbol is named in the exception list and in the failure.
func (s symbol) ID() string { return s.Pkg + "." + s.Name }

// terms are the strings a page can write to name the symbol. A method is
// named by a call on a variable or on its type (`.RunShutdown`,
// `Problem.MarshalJSON`) or by a signature in a table (`Issue(c, name,
// scopes, ttl)`) — but not by the bare word, which would let one page's
// `MarshalJSON` cover every MarshalJSON in the framework.
func (s symbol) terms() []string {
	if _, name, found := strings.Cut(s.Name, "."); found {
		return []string{"." + name, name + "("}
	}
	return []string{s.Name}
}

// covers reports whether the code of a locale names the symbol.
func covers(code string, s symbol) bool {
	for _, term := range s.terms() {
		if mentions(code, term) {
			return true
		}
	}
	return false
}

// readAPI parses api/current.txt into the symbols the site has to cover.
func readAPI(t *testing.T) []symbol {
	t.Helper()
	raw, err := os.ReadFile(apiFile)
	if err != nil {
		t.Fatal(err)
	}
	var out []symbol
	for _, line := range strings.Split(string(raw), "\n") {
		rest, ok := strings.CutPrefix(line, "pkg ")
		if !ok {
			continue
		}
		pkg, decl, ok := strings.Cut(rest, ", ")
		if !ok {
			continue
		}
		kind, body, ok := strings.Cut(decl, " ")
		if !ok {
			continue
		}
		var name string
		switch kind {
		case "const", "var", "type":
			// A struct field or an interface method is another line of its
			// type ("type Config struct, Addr string"): the type carries it.
			if strings.Contains(body, ", ") {
				continue
			}
			name, _, _ = strings.Cut(body, " ")
		case "func":
			name, _, _ = strings.Cut(body, "(")
		case "method":
			recv, m, found := strings.Cut(body, ") ")
			if !found {
				continue
			}
			m, _, _ = strings.Cut(m, "(")
			name = strings.TrimLeft(strings.TrimPrefix(recv, "("), "*") + "." + m
		default:
			continue
		}
		// The type parameters of a generic are not part of the name:
		// "DataTable[T any]" is written "DataTable".
		name, _, _ = strings.Cut(name, "[")
		if name == "" {
			continue
		}
		out = append(out, symbol{Pkg: pkg, Name: name, Kind: kind})
	}
	if len(out) < 500 {
		t.Fatalf("api/current.txt gave only %d symbols: its format changed", len(out))
	}
	return out
}

// codeOf keeps the code of a Markdown body — fenced blocks and inline spans —
// and drops the prose. A symbol counts as documented when the page writes it
// as code, which is how every page names one; the same word in a sentence
// ("pending", "inbox") is not something the reader can call.
func codeOf(body string) string {
	var sb strings.Builder
	fenced := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}
		for {
			_, after, found := strings.Cut(line, "`")
			if !found {
				break
			}
			span, tail, closed := strings.Cut(after, "`")
			if !closed {
				break
			}
			sb.WriteString(span)
			sb.WriteByte('\n')
			line = tail
		}
	}
	return sb.String()
}

// codeOfLocale is the code of every page of one locale.
func codeOfLocale(locale string) string {
	var sb strings.Builder
	for _, p := range Pages(locale) {
		sb.WriteString(codeOf(p.Body))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// mentions reports whether code names the term whole, and not as a piece of a
// longer identifier: "Limiter" does not cover "NewLimiter". Only the ends that
// are identifier characters are guarded, so the term "Emit(" matches
// "Emit(c, event, payload)".
func mentions(code, term string) bool {
	head, tail := isIdent(term[0]), isIdent(term[len(term)-1])
	for i := 0; i <= len(code)-len(term); {
		j := strings.Index(code[i:], term)
		if j < 0 {
			return false
		}
		j += i
		i = j + len(term)
		if head && j > 0 && isIdent(code[j-1]) {
			continue
		}
		if tail && i < len(code) && isIdent(code[i]) {
			continue
		}
		return true
	}
	return false
}

func isIdent(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// TestReferenceCoversAPI is the proof that the reference did not fall behind
// the code: a symbol of api/current.txt with no page is either a line someone
// owes the reader or an entry in documentedElsewhere saying why they do not.
func TestReferenceCoversAPI(t *testing.T) {
	code := map[string]string{}
	for _, l := range Locales {
		code[l.Code] = codeOfLocale(l.Code)
	}
	used := map[string]bool{}
	var missing []string
	for _, s := range readAPI(t) {
		if _, skip := skippedPkgs[s.Pkg]; skip {
			continue
		}
		if reason, ok := documentedElsewhere[s.ID()]; ok {
			if reason == "" {
				t.Errorf("documentedElsewhere[%q] has no reason", s.ID())
			}
			used[s.ID()] = true
			continue
		}
		for _, l := range Locales {
			if !covers(code[l.Code], s) {
				missing = append(missing, s.ID()+" ("+s.Kind+", no "+l.Code+" page)")
			}
		}
	}
	for id := range documentedElsewhere {
		if !used[id] {
			t.Errorf("documentedElsewhere[%q] is not a symbol of api/current.txt anymore", id)
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	t.Fatal(report("the reference fell behind the code", missing,
		"Document each one on the page of its package — one line in a table that is\n"+
			"already there is usually enough — in English and in Portuguese in the same\n"+
			"commit. A symbol the reader never writes goes into documentedElsewhere\n"+
			"with the reason."))
}

// TestUIKitCatalogue keeps reference/ui.md the place to look: every component
// of the kit is listed there, in both locales, even the ones that have a
// chapter of their own.
func TestUIKitCatalogue(t *testing.T) {
	code := map[string]string{}
	for _, l := range Locales {
		// Sections[1] is the reference of the locale ("reference",
		// "referencia"); the slug of the kit page is "ui" in both.
		p, ok := Get(l.Code, l.Sections[1].Key, "ui")
		if !ok {
			t.Fatalf("%s: no ui reference page", l.Code)
		}
		code[l.Code] = codeOf(p.Body)
	}
	var missing []string
	for _, s := range readAPI(t) {
		if s.Pkg != "ui" || s.Kind != "func" {
			continue
		}
		if _, ok := documentedElsewhere[s.ID()]; ok {
			continue
		}
		for _, l := range Locales {
			if !mentions(code[l.Code], s.Name) {
				missing = append(missing, s.ID()+" (no "+l.Code+" catalogue entry)")
			}
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	t.Fatal(report("the kit catalogue fell behind the code", missing,
		"Add one line to the catalogue — the signature and what it renders, with a\n"+
			"link to the chapter when it has one — in English and in Portuguese."))
}

// report is the failure both tests print: the headline, the list of what is
// missing and what to do about it.
func report(headline string, missing []string, what string) string {
	var sb strings.Builder
	sb.WriteString(headline + ": " + strconv.Itoa(len(missing)))
	if len(missing) == 1 {
		sb.WriteString(" symbol.\n")
	} else {
		sb.WriteString(" symbols.\n")
	}
	for _, m := range missing {
		sb.WriteString("- " + m + "\n")
	}
	sb.WriteString("\n" + what)
	return sb.String()
}
