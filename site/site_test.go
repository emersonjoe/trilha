package main

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/site/internal/demos"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

func get(t *testing.T, path string) (int, string) {
	t.Helper()
	rec := request(t, path)
	return rec.Code, rec.Body.String()
}

func request(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	newApp().Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// homes lists the home page of every locale.
func homes() []string {
	var out []string
	for _, l := range docs.Locales {
		out = append(out, l.Home())
	}
	return out
}

func pagePaths() []string {
	var out []string
	for _, p := range docs.All() {
		out = append(out, p.Path())
	}
	return out
}

func allPaths() []string { return append(homes(), pagePaths()...) }

func TestEveryPageResponds(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	for _, l := range docs.Locales {
		paths := []string{l.Home()}
		for _, p := range docs.Pages(l.Code) {
			paths = append(paths, p.Path())
		}
		for _, p := range paths {
			code, body := get(t, p)
			if code != 200 {
				t.Errorf("%s → %d", p, code)
			}
			if !strings.Contains(body, "<title>") || !strings.Contains(body, `href="/site.css?v=`) {
				t.Errorf("%s: missing shell", p)
			}
			if !strings.Contains(body, `<html lang="`+l.Lang+`">`) {
				t.Errorf("%s: <html lang> must be %q", p, l.Lang)
			}
		}
	}
	for _, p := range []string{"/learn/nao-existe", "/pt/aprender/pages-and-routes", "/aprender/pages-and-routes/x"} {
		if code, _ := get(t, p); code != 404 {
			t.Errorf("%s → %d, want 404", p, code)
		}
	}
}

// Spec 015: the pre-i18n Portuguese paths keep working through a permanent
// redirect that carries the base path.
func TestLegacyPathsRedirect(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	for old, want := range map[string]string{"/aprender": "/pt/aprender", "/referencia": "/pt/referencia", "/referencia/ctx": "/pt/referencia/ctx", "/aprender/formularios": "/pt/aprender/formularios"} {
		rec := request(t, old)
		if rec.Code != 301 || rec.Header().Get("Location") != want {
			t.Errorf("%s → %d %s, want 301 %s", old, rec.Code, rec.Header().Get("Location"), want)
		}
	}
	t.Setenv("TRILHA_BASE_PATH", "/trilha")
	if rec := request(t, "/referencia/ctx"); rec.Header().Get("Location") != "/trilha/pt/referencia/ctx" {
		t.Errorf("base path missing in redirect: %s", rec.Header().Get("Location"))
	}
}

// Spec 015: every page declares its translations and links to them.
func TestAlternatesAndSwitcher(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	t.Setenv("SITE_ORIGIN", "")
	cases := map[string]string{"/learn/forms": "/pt/aprender/formularios", "/reference/ctx": "/pt/referencia/ctx", "/learn": "/pt/aprender", "/": "/pt"}
	for en, pt := range cases {
		_, body := get(t, en)
		for _, want := range []string{
			`<link rel="alternate" hreflang="en" href="` + en + `">`,
			`<link rel="alternate" hreflang="pt-BR" href="` + pt + `">`,
			`<link rel="alternate" hreflang="x-default" href="` + en + `">`,
			`<a class="idioma" href="` + pt + `" hreflang="pt-BR" lang="pt-BR">Português</a>`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s misses %s", en, want)
			}
		}
		_, body = get(t, pt)
		for _, want := range []string{
			`<link rel="alternate" hreflang="en" href="` + en + `">`,
			`<link rel="alternate" hreflang="x-default" href="` + en + `">`,
			`<a class="idioma" href="` + en + `" hreflang="en" lang="en">English</a>`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s misses %s", pt, want)
			}
		}
	}
	t.Setenv("SITE_ORIGIN", "https://example.org")
	t.Setenv("TRILHA_BASE_PATH", "/trilha")
	if _, body := get(t, "/learn"); !strings.Contains(body, `hreflang="pt-BR" href="https://example.org/trilha/pt/aprender"`) {
		t.Error("hreflang must be absolute when SITE_ORIGIN is set")
	}
}

// Spec 015: locales are parallel. A page, a demo or a section without its
// counterpart is a bug (the constitution says the translation ships in the
// same commit).
func TestLocalesInSync(t *testing.T) {
	ref := docs.Locales[0]
	demoRe := regexp.MustCompile(`(?m)^@demo\s+(\S+)`)
	for _, l := range docs.Locales[1:] {
		if len(l.Sections) != len(ref.Sections) {
			t.Fatalf("%s has %d sections, %s has %d", l.Code, len(l.Sections), ref.Code, len(ref.Sections))
		}
		for i, s := range ref.Sections {
			if got := len(l.Sections[i].Slugs); got != len(s.Slugs) {
				t.Errorf("section %s: %s has %d pages, %s has %d", s.Key, l.Code, got, ref.Code, len(s.Slugs))
			}
		}
	}
	for _, p := range docs.All() {
		for _, l := range docs.Locales {
			tr, ok := docs.Translation(p, l.Code)
			if !ok {
				t.Errorf("%s has no translation in %s", p.Path(), l.Code)
				continue
			}
			mine, theirs := demoRe.FindAllStringSubmatch(p.Body, -1), demoRe.FindAllStringSubmatch(tr.Body, -1)
			if len(mine) != len(theirs) {
				t.Errorf("%s has %d demos, %s has %d", p.Path(), len(mine), tr.Path(), len(theirs))
				continue
			}
			for i := range mine {
				if mine[i][1] != theirs[i][1] {
					t.Errorf("%s: demo %q vs %q in %s", p.Path(), mine[i][1], theirs[i][1], tr.Path())
				}
			}
		}
		for _, m := range demoRe.FindAllStringSubmatch(p.Body, -1) {
			if _, ok := demos.Get(p.Locale, m[1]); !ok {
				t.Errorf("%s uses unknown demo %q (locale %s)", p.Path(), m[1], p.Locale)
			}
		}
	}
	for _, l := range docs.Locales {
		if got, want := demos.Names(l.Code), demos.Names(ref.Code); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("demos of %s (%v) differ from %s (%v)", l.Code, got, ref.Code, want)
		}
	}
}

func TestChaptersHaveChallengeAndSolution(t *testing.T) {
	ids := map[string]string{"en": `id="challenge"`, "pt": `id="desafio"`}
	for _, l := range docs.Locales {
		learn := l.Sections[0]
		last := learn.Slugs[len(learn.Slugs)-1] // troubleshooting has no challenge
		for _, p := range docs.Pages(l.Code) {
			if p.Section != learn.Key || p.Slug == last {
				continue
			}
			_, body := get(t, p.Path())
			if !strings.Contains(body, ids[l.Code]) || !strings.Contains(body, `<details class="solucao">`) {
				t.Errorf("%s: chapter without challenge + solution", p.Path())
			}
		}
	}
}

func TestInternalLinksResolve(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	known := map[string]bool{}
	for _, p := range allPaths() {
		known[p] = true
	}
	re := regexp.MustCompile(`href="(/[^"#?]*)`)
	for _, p := range allPaths() {
		_, body := get(t, p)
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			href := m[1]
			if strings.HasSuffix(href, ".css") || strings.HasSuffix(href, ".js") || strings.HasSuffix(href, ".svg") {
				continue
			}
			if !known[href] {
				t.Errorf("%s links to unknown %s", p, href)
			}
		}
	}
}

// Spec 015: a page in one language never links to content in the other
// (except through the switcher), otherwise the reader silently changes
// language mid-trail.
func TestNoCrossLocaleLinks(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	re := regexp.MustCompile(`<a (?:class="[^"]*" )?href="(/[^"#?]*)"`)
	for _, p := range docs.All() {
		_, body := get(t, p.Path())
		body = body[strings.Index(body, "<main"):]
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			isPT := m[1] == "/pt" || strings.HasPrefix(m[1], "/pt/")
			if isPT != (p.Locale == "pt") {
				t.Errorf("%s (%s) links to %s", p.Path(), p.Locale, m[1])
			}
		}
	}
}

func TestBasePathPrefixesLinks(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "/trilha")
	_, body := get(t, "/learn")
	if !strings.Contains(body, `href="/trilha/learn/pages-and-routes"`) || !strings.Contains(body, `href="/trilha/site.css?v=`) || !strings.Contains(body, `href="/trilha/pt/aprender"`) {
		t.Fatal("links must carry the base path")
	}
	if strings.Contains(body, `href="/learn/`) || strings.Contains(body, `href="/pt/`) {
		t.Fatal("unprefixed link found")
	}
}

func TestExportPathsCoverEveryPage(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	a := newApp()
	got := " " + strings.Join(a.ExportPaths(), " ") + " "
	want := allPaths()
	for _, p := range docs.Pages("pt") {
		want = append(want, strings.TrimPrefix(p.Path(), "/pt")) // legacy redirect stubs
	}
	for _, p := range want {
		if !strings.Contains(got, " "+p+" ") {
			t.Errorf("export misses %s", p)
		}
	}
}

func TestNoDevScriptInProd(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	if _, body := get(t, "/"); strings.Contains(body, "_trilha/events") {
		t.Fatal("dev script leaked")
	}
}

func TestAnalyticsOnlyWhenConfigured(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("SITE_ANALYTICS", "")
	if _, body := get(t, "/"); strings.Contains(body, "goatcounter") {
		t.Fatal("analytics must be off by default")
	}
	t.Setenv("SITE_ANALYTICS", "goatcounter:trilha")
	rec := request(t, "/")
	body := rec.Body.String()
	if !strings.Contains(body, `<script data-goatcounter="https://trilha.goatcounter.com/count" async src="https://gc.zgo.to/count.js">`) || !strings.Contains(body, "without cookies") {
		t.Fatal(body)
	}
	if _, body := get(t, "/pt"); !strings.Contains(body, "sem cookies") {
		t.Fatal("analytics note must follow the locale")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'nonce-") || !strings.Contains(csp, "https://gc.zgo.to") || !strings.Contains(csp, "connect-src 'self' https://trilha.goatcounter.com") {
		t.Fatal(csp)
	}
	for _, bad := range []string{"google:UA-1", "goatcounter:", "goatcounter:Bad Code", "x"} {
		if _, err := ui.ParseAnalytics(bad); err == nil {
			t.Fatal("must reject", bad)
		}
	}
}

// Spec 013: the default CSP (script-src without unsafe-inline) blocks inline
// handlers, and a site that teaches this cannot use them.
func TestNoInlineEventHandlers(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	inline := regexp.MustCompile(`\son(click|change|submit|load|input|error|mouseover)\s*=`)
	for _, p := range allPaths() {
		_, body := get(t, p)
		if m := inline.FindString(body); m != "" {
			t.Errorf("%s: inline handler %q (use an external .js file)", p, strings.TrimSpace(m))
		}
	}
}

// Spec 013: the form demo answers the submit without an inline handler, in
// both languages.
func TestFormDemoIsInteractive(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	for path, note := range map[string]string{"/": "simulated in the browser", "/learn/forms": "simulated in the browser", "/pt": "simulado no navegador", "/pt/aprender/formularios": "simulado no navegador"} {
		_, body := get(t, path)
		if !strings.Contains(body, `<form method="get" action="#" data-demo="form"`) {
			t.Errorf("%s: form demo without data-demo", path)
		}
		if !strings.Contains(body, `data-demo-saida`) || !strings.Contains(body, note) {
			t.Errorf("%s: demo without output area or note", path)
		}
	}
}

// cookbookSection is the position of the Cookbook in every locale's list of
// sections; the keys differ per language, the position does not.
const cookbookSection = 2

// Spec 038: every Go block of the cookbook is a declaration copied from a
// file that compiles with the rest of the repository. A block that stops
// matching its source is a recipe that stopped being true, and a recipe
// nobody can run is worse than no recipe.
func TestCookbookSnippetsAreReal(t *testing.T) {
	sources := repoGoSources(t)
	fence := regexp.MustCompile("(?s)```go\n(.*?)\n```")
	for _, p := range docs.All() {
		if p.Section != docs.LocaleOf(p.Locale).Sections[cookbookSection].Key {
			continue
		}
		for _, b := range fence.FindAllStringSubmatch(p.Body, -1) {
			if !strings.Contains(sources, b[1]) {
				t.Errorf("%s: block is in no .go file of the repository:\n%s", p.Path(), b[1])
			}
		}
	}
}

// repoGoSources concatenates every .go file of the repository, minus the
// synthetic trees under testdata, which exist to be broken.
func repoGoSources(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && (d.Name() == ".git" || d.Name() == "testdata"):
			return fs.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go"):
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.Write(raw)
		b.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
