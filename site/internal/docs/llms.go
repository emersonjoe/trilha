package docs

import (
	"fmt"
	"regexp"
	"strings"
)

// linkRe is an internal link of a page body: ](/learn/x), ](/pt/referencia/y).
var linkRe = regexp.MustCompile(`\]\((/[a-z0-9/-]+)\)`)

// llmsIntro is the header of llms.txt per locale: the name, the one-line
// summary and the paragraph that says what the file is.
var llmsIntro = map[string][3]string{
	"en": {
		"Trilha",
		"A Next.js-style web framework for Go: a folder under app/ is a route, HTML is written in Go, and nothing outside the standard library is imported.",
		"This is the documentation index in plain text. Every entry below is one page; the whole documentation concatenated is at %s/llms-full.txt.",
	},
	"pt": {
		"Trilha",
		"Framework web para Go no estilo Next.js: uma pasta em app/ é uma rota, o HTML é escrito em Go e nada fora da biblioteca padrão é importado.",
		"Este é o índice da documentação em texto puro. Cada item abaixo é uma página; a documentação inteira, concatenada, está em %s/llms-full.txt.",
	},
}

// LLMs returns the llms.txt of the locale: the site in one paragraph and one
// line per page, grouped by section. base is the URL prefix the site is served
// under ("" or "/trilha").
func LLMs(locale, base string) string {
	l := LocaleOf(locale)
	in := llmsIntro[l.Code]
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n> %s\n\n", in[0], in[1])
	fmt.Fprintf(&sb, in[2]+"\n", base+l.Prefix)
	for _, s := range l.Sections {
		fmt.Fprintf(&sb, "\n## %s\n\n", s.Title)
		for _, slug := range s.Slugs {
			p, ok := Get(l.Code, s.Key, slug)
			if !ok {
				continue
			}
			fmt.Fprintf(&sb, "- [%s](%s): %s\n", p.Title, base+p.Path(), p.Description)
		}
	}
	return sb.String()
}

// LLMsFull returns every page of the locale as one Markdown document, bodies
// verbatim so the code blocks arrive whole.
func LLMsFull(locale, base string) string {
	l := LocaleOf(locale)
	in := llmsIntro[l.Code]
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n> %s\n", in[0], in[1])
	for _, s := range l.Sections {
		for _, slug := range s.Slugs {
			p, ok := Get(l.Code, s.Key, slug)
			if !ok {
				continue
			}
			fmt.Fprintf(&sb, "\n\n---\n\n# %s\n\nSource: %s\n\n%s\n\n%s\n",
				p.Title, base+p.Path(), p.Description, withBase(strings.TrimSpace(p.Body), base))
		}
	}
	return sb.String()
}

// withBase prefixes the site-rooted Markdown links of a body, the same thing
// the HTML renderer does for a page.
func withBase(body, base string) string {
	if base == "" {
		return body
	}
	return strings.ReplaceAll(body, "](/", "]("+base+"/")
}

// packPages is the slice of documentation one recipe touches: its own page
// and the pages that page links to, at most twelve. It is what the recipe's
// llms.txt lists and what the page's cost badge answers for.
func packPages(locale, slug string) ([]Page, bool) {
	section := "cookbook"
	if locale == "pt" {
		section = "receitas"
	}
	first, ok := Get(locale, section, slug)
	if !ok || first.Slug == "" {
		return nil, false
	}
	byPath := map[string]Page{}
	for _, p := range All() {
		byPath[p.Path()] = p
	}
	out := []Page{first}
	seen := map[string]bool{first.Path(): true}
	for _, m := range linkRe.FindAllStringSubmatch(first.Body, -1) {
		if len(out) >= 12 {
			break
		}
		p, ok := byPath[m[1]]
		if !ok || seen[p.Path()] {
			continue
		}
		seen[p.Path()] = true
		out = append(out, p)
	}
	return out, true
}

// PackCost prices a recipe's pack: the estimated tokens of every page it
// touches, at the shared rate of four characters per token. Every caller
// labels it est. — it is a budget, not a bill.
func PackCost(locale, slug string) (int, bool) {
	pages, ok := packPages(locale, slug)
	if !ok {
		return 0, false
	}
	total := 0
	for _, p := range pages {
		total += (len(p.Body) + 3) / 4
	}
	return total, true
}

// LLMsRecipe returns one recipe's llms.txt: the priced list of the pack —
// the pages the recipe touches, each with its estimated cost — and the
// recipe's own page verbatim, so an agent reads the recipe and knows what
// reading the rest of the pack would cost. ok is false when the slug names
// no cookbook page.
func LLMsRecipe(locale, base, slug string) (string, bool) {
	pages, ok := packPages(locale, slug)
	if !ok {
		return "", false
	}
	whole := LocaleOf(locale)
	in := llmsIntro[whole.Code]
	var sb strings.Builder
	title := "Recipe: " + pages[0].Title
	if locale == "pt" {
		title = "Receita: " + pages[0].Title
	}
	fmt.Fprintf(&sb, "# %s\n\n> %s\n\n", title, in[1])
	fmt.Fprintf(&sb, "> %s\n\n", base+whole.Prefix+"/llms/recipes/"+slug+".txt")
	total := 0
	for _, p := range pages {
		cost := (len(p.Body) + 3) / 4
		total += cost
		fmt.Fprintf(&sb, "- [%s](%s): %s · %d tokens est.\n", p.Title, base+p.Path(), p.Description, cost)
	}
	fmt.Fprintf(&sb, "\n%d page(s) · %d tokens est. in all\n", len(pages), total)
	fmt.Fprintf(&sb, "\n---\n\n%s\n", withBase(strings.TrimSpace(pages[0].Body), base))
	return sb.String(), true
}

// RecipeSlugs lists a locale's cookbook slugs in page order, without the
// section index — the list the export needs for the per-recipe llms.txt.
func RecipeSlugs(locale string) []string {
	section := "cookbook"
	if locale == "pt" {
		section = "receitas"
	}
	var out []string
	for _, s := range LocaleOf(locale).Sections {
		if s.Key != section {
			continue
		}
		for _, slug := range s.Slugs {
			if slug != "" {
				out = append(out, slug)
			}
		}
	}
	return out
}
