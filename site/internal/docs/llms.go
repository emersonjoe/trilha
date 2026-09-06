package docs

import (
	"fmt"
	"strings"
)

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
