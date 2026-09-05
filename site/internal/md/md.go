// Package md is a deliberately small Markdown-to-HTML converter for the
// documentation site: headings with ids, paragraphs, lists, fenced code (with
// Go highlighting), tables, blockquotes, callouts (:::nome), inline code,
// emphasis and links. No third-party dependencies.
package md

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// Heading is a heading collected while rendering (for tables of contents).
type Heading struct {
	Level int
	ID    string
	Text  string
}

// Options tune rendering.
type Options struct {
	// Base is prefixed to links that start with "/".
	Base string
	// Demo renders a "@demo name" directive; nil ignores the directive.
	Demo func(name string) string
	// Locale picks the language of generated labels (callout titles). Empty
	// means "en".
	Locale string
}

// Callouts are written as :::name. The class names stay in Portuguese (they
// are the CSS contract); English content may use the aliases below.
var calloutAlias = map[string]string{"tip": "dica", "warning": "atencao", "note": "nota", "challenge": "desafio", "solution": "solucao"}

var calloutTitles = map[string]map[string]string{
	"en": {"dica": "Tip", "atencao": "Warning", "nota": "Note", "desafio": "Challenge", "solucao": "Show solution"},
	"pt": {"dica": "Dica", "atencao": "Atenção", "nota": "Nota", "desafio": "Desafio", "solucao": "Mostrar solução"},
}

// Render converts Markdown to HTML and returns the headings found.
func Render(src string, opt Options) (string, []Heading) {
	r := &renderer{opt: opt}
	r.render(strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n"))
	return r.sb.String(), r.headings
}

type renderer struct {
	opt      Options
	sb       strings.Builder
	headings []Heading
	ids      map[string]int
}

var (
	reHeading = regexp.MustCompile(`^(#{1,4})\s+(.*)$`)
	reUL      = regexp.MustCompile(`^[-*]\s+(.*)$`)
	reOL      = regexp.MustCompile(`^\d+\.\s+(.*)$`)
	reTable   = regexp.MustCompile(`^\|.*\|\s*$`)
	reSep     = regexp.MustCompile(`^\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$`)
	reCode    = regexp.MustCompile("`([^`]+)`")
	reBold    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic  = regexp.MustCompile(`(^|[^*\w])\*([^*]+)\*`)
	reLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

func (r *renderer) render(lines []string) {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		switch {
		case trim == "":
			continue
		case strings.HasPrefix(trim, "```"):
			lang := strings.TrimSpace(strings.TrimPrefix(trim, "```"))
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			r.codeBlock(lang, strings.Join(code, "\n"))
		case strings.HasPrefix(trim, ":::"):
			name := strings.TrimSpace(strings.TrimPrefix(trim, ":::"))
			var body []string
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != ":::"; i++ {
				body = append(body, lines[i])
			}
			r.callout(name, body)
		case strings.HasPrefix(trim, "@demo "):
			if r.opt.Demo != nil {
				r.sb.WriteString(r.opt.Demo(strings.TrimSpace(strings.TrimPrefix(trim, "@demo "))))
			}
		case trim == "---":
			r.sb.WriteString("<hr>\n")
		case reHeading.MatchString(trim):
			m := reHeading.FindStringSubmatch(trim)
			r.heading(len(m[1]), m[2])
		case strings.HasPrefix(trim, "> "):
			var body []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				body = append(body, strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"), " "))
			}
			i--
			r.sb.WriteString("<blockquote>")
			r.render(body)
			r.sb.WriteString("</blockquote>\n")
		case reUL.MatchString(trim), reOL.MatchString(trim):
			tag, re := "ul", reUL
			if reOL.MatchString(trim) {
				tag, re = "ol", reOL
			}
			r.sb.WriteString("<" + tag + ">\n")
			for ; i < len(lines) && re.MatchString(strings.TrimSpace(lines[i])); i++ {
				item := re.FindStringSubmatch(strings.TrimSpace(lines[i]))[1]
				// Continuation lines indented by two+ spaces belong to the item.
				for i+1 < len(lines) && strings.HasPrefix(lines[i+1], "  ") && strings.TrimSpace(lines[i+1]) != "" && !re.MatchString(strings.TrimSpace(lines[i+1])) {
					i++
					item += " " + strings.TrimSpace(lines[i])
				}
				r.sb.WriteString("<li>" + r.inline(item) + "</li>\n")
			}
			i--
			r.sb.WriteString("</" + tag + ">\n")
		case reTable.MatchString(trim) && i+1 < len(lines) && reSep.MatchString(strings.TrimSpace(lines[i+1])):
			head := cells(trim)
			r.sb.WriteString("<div class=\"tabela\"><table><thead><tr>")
			for _, c := range head {
				r.sb.WriteString("<th>" + r.inline(c) + "</th>")
			}
			r.sb.WriteString("</tr></thead><tbody>\n")
			for i += 2; i < len(lines) && reTable.MatchString(strings.TrimSpace(lines[i])); i++ {
				r.sb.WriteString("<tr>")
				for _, c := range cells(strings.TrimSpace(lines[i])) {
					r.sb.WriteString("<td>" + r.inline(c) + "</td>")
				}
				r.sb.WriteString("</tr>\n")
			}
			i--
			r.sb.WriteString("</tbody></table></div>\n")
		default:
			var para []string
			for ; i < len(lines) && strings.TrimSpace(lines[i]) != "" && !isBlockStart(strings.TrimSpace(lines[i])); i++ {
				para = append(para, strings.TrimSpace(lines[i]))
			}
			i--
			r.sb.WriteString("<p>" + r.inline(strings.Join(para, " ")) + "</p>\n")
		}
	}
}

func isBlockStart(trim string) bool {
	return strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, ":::") || strings.HasPrefix(trim, "@demo ") ||
		reHeading.MatchString(trim) || strings.HasPrefix(trim, "> ") || reUL.MatchString(trim) || reOL.MatchString(trim) || trim == "---"
}

func cells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func (r *renderer) heading(level int, text string) {
	id := Slug(text)
	if r.ids == nil {
		r.ids = map[string]int{}
	}
	r.ids[id]++
	if n := r.ids[id]; n > 1 {
		id = fmt.Sprintf("%s-%d", id, n)
	}
	r.headings = append(r.headings, Heading{Level: level, ID: id, Text: stripInline(text)})
	fmt.Fprintf(&r.sb, "<h%d id=\"%s\">%s<a class=\"ancora\" href=\"#%s\" aria-label=\"Link para esta seção\">#</a></h%d>\n", level, id, r.inline(text), id, level)
}

func (r *renderer) codeBlock(lang, code string) {
	var body string
	if lang == "go" {
		body = HighlightGo(code)
	} else {
		body = html.EscapeString(code)
	}
	if lang == "" {
		lang = "text"
	}
	fmt.Fprintf(&r.sb, "<div class=\"codigo\" data-lang=\"%s\"><pre><code class=\"lang-%s\">%s</code></pre></div>\n", html.EscapeString(lang), html.EscapeString(lang), body)
}

func (r *renderer) callout(name string, body []string) {
	if canon, ok := calloutAlias[name]; ok {
		name = canon
	}
	titles := calloutTitles[r.opt.Locale]
	if titles == nil {
		titles = calloutTitles["en"]
	}
	switch name {
	case "solucao":
		r.sb.WriteString("<details class=\"solucao\"><summary>" + html.EscapeString(titles[name]) + "</summary>\n")
		r.render(body)
		r.sb.WriteString("</details>\n")
	default:
		title := titles[name]
		if title == "" {
			title = name
		}
		fmt.Fprintf(&r.sb, "<aside class=\"aviso %s\"><strong>%s</strong>\n", html.EscapeString(name), html.EscapeString(title))
		r.render(body)
		r.sb.WriteString("</aside>\n")
	}
}

// inline renders inline markup. Code spans are protected from the other
// substitutions.
func (r *renderer) inline(s string) string {
	var codes []string
	s = reCode.ReplaceAllStringFunc(s, func(m string) string {
		codes = append(codes, "<code>"+html.EscapeString(m[1:len(m)-1])+"</code>")
		return fmt.Sprintf("\x00%d\x00", len(codes)-1)
	})
	s = html.EscapeString(s)
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		p := reLink.FindStringSubmatch(m)
		href := p[2]
		if strings.HasPrefix(href, "/") {
			href = r.opt.Base + href
		}
		attrs := ""
		if strings.HasPrefix(href, "http") {
			attrs = ` rel="noopener"`
		}
		return `<a href="` + href + `"` + attrs + `>` + p[1] + `</a>`
	})
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reItalic.ReplaceAllString(s, "$1<em>$2</em>")
	for i, c := range codes {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00%d\x00", i), c)
	}
	return s
}

func stripInline(s string) string {
	s = reCode.ReplaceAllString(s, "$1")
	s = reBold.ReplaceAllString(s, "$1")
	s = reLink.ReplaceAllString(s, "$1")
	return s
}

// Slug turns a heading into an id: lowercase ASCII letters, digits and dashes.
func Slug(s string) string {
	s = strings.ToLower(stripInline(s))
	repl := map[rune]string{'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'é': "e", 'ê': "e", 'í': "i", 'ó': "o", 'ô': "o", 'õ': "o", 'ú': "u", 'ç': "c"}
	var b strings.Builder
	dash := false
	for _, r := range s {
		if v, ok := repl[r]; ok {
			b.WriteString(v)
			dash = false
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
