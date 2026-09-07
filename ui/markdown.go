package ui

import (
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// MarkdownOpts tunes Markdown. The zero value is the safe one: headings
// demoted to <h3>, no images, no raw HTML.
type MarkdownOpts struct {
	// HeadingBase is the level a single "#" becomes. 0 means 3: the text is
	// rendered inside a page that already has its own <h1>, and a heading in
	// the middle of an answer must not compete with the title of the screen.
	HeadingBase int
	// Images turns ![alt](url) into an <img>. Off by default: an image is a
	// request to somewhere else, asked for by text nobody vouches for. The
	// URL is checked either way.
	Images bool
	// Class is added to the wrapping <div>, on top of "ui-md".
	Class string
}

// Markdown renders a subset of CommonMark as a tree of nodes: paragraphs,
// headings, lists, blockquotes, fenced and inline code, GFM tables, links,
// emphasis and hard breaks.
//
// It is written for text that came from a language model or from a person, so
// the rule that matters is the one with no exception: there is no raw HTML.
// A tag in the source is text in the output, because the output is a tree the
// h package escapes by construction — never a string somebody could hand to
// h.Raw.
//
//	ui.Markdown(answer, ui.MarkdownOpts{})
func Markdown(src string, opt MarkdownOpts) h.Node {
	p := &mdParser{opt: opt, lines: strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")}
	body := p.blocks(0)
	class := "ui-md"
	if opt.Class != "" {
		class += " " + opt.Class
	}
	return h.Div(h.Class(class), body)
}

type mdParser struct {
	opt   MarkdownOpts
	lines []string
	i     int
}

// blocks reads block elements until the lines run out or a line dedents below
// indent (which is how a fenced block or a paragraph inside a list item is
// found).
func (p *mdParser) blocks(indent int) h.Node {
	var out []h.Node
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		trim := strings.TrimSpace(line)
		if trim == "" {
			p.i++
			continue
		}
		if mdIndent(line) < indent {
			break
		}
		line = mdTrimIndent(line, indent)
		trim = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~"):
			out = append(out, p.fence(indent, trim[:3]))
		case mdIsRule(trim):
			p.i++
			out = append(out, h.Hr())
		case strings.HasPrefix(trim, "#"):
			if n, ok := p.heading(trim); ok {
				out = append(out, n)
				continue
			}
			out = append(out, p.paragraph(indent))
		case strings.HasPrefix(trim, ">"):
			out = append(out, p.quote(indent))
		case mdBullet(trim) != "":
			out = append(out, p.list(indent, false))
		case mdNumber(trim) != "":
			out = append(out, p.list(indent, true))
		case p.isTable(indent):
			out = append(out, p.table(indent))
		default:
			out = append(out, p.paragraph(indent))
		}
	}
	return h.Group(out...)
}

func (p *mdParser) heading(trim string) (h.Node, bool) {
	n := 0
	for n < len(trim) && trim[n] == '#' {
		n++
	}
	if n > 6 || n >= len(trim) || trim[n] != ' ' {
		return nil, false
	}
	p.i++
	base := p.opt.HeadingBase
	if base <= 0 {
		base = 3
	}
	level := base + n - 1
	if level > 6 {
		level = 6
	}
	if level < 1 {
		level = 1
	}
	text := strings.TrimRight(strings.TrimSpace(trim[n+1:]), "#")
	return h.El("h"+string(rune('0'+level)), p.inline(strings.TrimSpace(text))), true
}

func (p *mdParser) fence(indent int, mark string) h.Node {
	open := strings.TrimSpace(mdTrimIndent(p.lines[p.i], indent))
	lang := strings.TrimSpace(strings.TrimLeft(open, "`~"))
	// A language is one word; anything else is a fence with noise on it.
	if i := strings.IndexAny(lang, " \t"); i >= 0 {
		lang = lang[:i]
	}
	p.i++
	var code []string
	for p.i < len(p.lines) {
		l := mdTrimIndent(p.lines[p.i], indent)
		if strings.HasPrefix(strings.TrimSpace(l), mark) {
			p.i++
			break
		}
		code = append(code, l)
		p.i++
	}
	attrs := []h.Node{h.Class("ui-code")}
	if lang != "" && mdWord(lang) {
		attrs = append(attrs, h.Class("language-"+lang), h.Data("lang", lang))
	}
	return h.Pre(h.Class("ui-pre"), h.Code(append(attrs, h.Text(strings.Join(code, "\n")))...))
}

func (p *mdParser) quote(indent int) h.Node {
	var body []string
	for p.i < len(p.lines) {
		l := mdTrimIndent(p.lines[p.i], indent)
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, ">") {
			break
		}
		body = append(body, strings.TrimPrefix(strings.TrimPrefix(t, ">"), " "))
		p.i++
	}
	inner := &mdParser{opt: p.opt, lines: body}
	return h.Blockquote(h.Class("ui-quote"), inner.blocks(0))
}

func (p *mdParser) list(indent int, ordered bool) h.Node {
	marker := mdBullet
	if ordered {
		marker = mdNumber
	}
	var items []h.Node
	for p.i < len(p.lines) {
		line := mdTrimIndent(p.lines[p.i], indent)
		trim := strings.TrimSpace(line)
		if trim == "" {
			// A blank line inside a list is only a break if what follows is
			// not part of the list any more.
			if p.i+1 < len(p.lines) && mdIndent(p.lines[p.i+1]) >= indent &&
				marker(strings.TrimSpace(mdTrimIndent(p.lines[p.i+1], indent))) != "" {
				p.i++
				continue
			}
			break
		}
		// A bullet further left belongs to the list that opened further left,
		// not to this one.
		if mdIndent(p.lines[p.i]) < indent {
			break
		}
		m := marker(trim)
		if m == "" {
			break
		}
		own := mdIndent(line)
		// The marker is replaced by its own width in spaces, keeping the
		// absolute indentation: what was under the item stays under it, which
		// is the whole of how a nested list is found.
		content := indent + own + len(m)
		p.lines[p.i] = strings.Repeat(" ", content) + strings.TrimPrefix(trim, m)
		items = append(items, h.Li(p.item(content)))
	}
	if ordered {
		return h.Ol(append([]h.Node{h.Class("ui-md-list")}, items...)...)
	}
	return h.Ul(append([]h.Node{h.Class("ui-md-list")}, items...)...)
}

// item is the body of a list item: its first line plus whatever is indented
// under it — a nested list, a fenced block, another paragraph.
func (p *mdParser) item(indent int) h.Node {
	first := strings.TrimSpace(mdTrimIndent(p.lines[p.i], indent))
	p.i++
	head := h.Node(h.Nil)
	if first != "" && !strings.HasPrefix(first, "```") && !strings.HasPrefix(first, "~~~") {
		head = p.inline(first)
	} else if first != "" {
		p.i--
	}
	rest := p.blocks(indent)
	return h.Group(head, rest)
}

func (p *mdParser) isTable(indent int) bool {
	line := strings.TrimSpace(mdTrimIndent(p.lines[p.i], indent))
	if !strings.Contains(line, "|") || p.i+1 >= len(p.lines) {
		return false
	}
	return mdIsSeparator(strings.TrimSpace(mdTrimIndent(p.lines[p.i+1], indent)))
}

func (p *mdParser) table(indent int) h.Node {
	head := mdCells(strings.TrimSpace(mdTrimIndent(p.lines[p.i], indent)))
	p.i += 2
	ths := make([]h.Node, 0, len(head))
	for _, c := range head {
		ths = append(ths, h.Th(p.inline(c)))
	}
	var rows []h.Node
	for p.i < len(p.lines) {
		line := strings.TrimSpace(mdTrimIndent(p.lines[p.i], indent))
		if line == "" || !strings.Contains(line, "|") {
			break
		}
		cells := mdCells(line)
		tds := make([]h.Node, 0, len(cells))
		for _, c := range cells {
			tds = append(tds, h.Td(p.inline(c)))
		}
		rows = append(rows, h.Tr(tds...))
		p.i++
	}
	return h.Div(h.Class("ui-table-wrap"),
		h.Table(h.Class("ui-table"),
			h.Thead(h.Tr(ths...)),
			h.Tbody(rows...),
		),
	)
}

func (p *mdParser) paragraph(indent int) h.Node {
	var para []string
	for p.i < len(p.lines) {
		line := mdTrimIndent(p.lines[p.i], indent)
		trim := strings.TrimSpace(line)
		if trim == "" || mdIndent(p.lines[p.i]) < indent || p.startsBlock(indent) {
			break
		}
		// Only the left side is trimmed: two spaces on the right are a hard
		// break, and trimming them is how one gets lost.
		para = append(para, strings.TrimLeft(line, " \t"))
		p.i++
	}
	if len(para) == 0 {
		p.i++
		return h.Nil
	}
	return h.P(p.inlineLines(para))
}

// startsBlock reports whether the current line opens a block, which is what
// ends a paragraph without a blank line between them.
func (p *mdParser) startsBlock(indent int) bool {
	trim := strings.TrimSpace(mdTrimIndent(p.lines[p.i], indent))
	switch {
	case strings.HasPrefix(trim, "```"), strings.HasPrefix(trim, "~~~"):
		return true
	case strings.HasPrefix(trim, ">"), mdIsRule(trim):
		return true
	case mdBullet(trim) != "", mdNumber(trim) != "":
		return true
	case strings.HasPrefix(trim, "#"):
		n := 0
		for n < len(trim) && trim[n] == '#' {
			n++
		}
		return n <= 6 && n < len(trim) && trim[n] == ' '
	}
	return false
}

// inlineLines joins the lines of a paragraph. A line ending in two spaces or a
// backslash is a hard break; everything else is one paragraph of running text.
func (p *mdParser) inlineLines(lines []string) h.Node {
	var out []h.Node
	for i, l := range lines {
		hard := strings.HasSuffix(l, "  ") || strings.HasSuffix(l, `\`)
		out = append(out, p.inline(strings.TrimRight(strings.TrimSuffix(l, `\`), " ")))
		if i == len(lines)-1 {
			break
		}
		if hard {
			out = append(out, h.Br())
		} else {
			out = append(out, h.Text(" "))
		}
	}
	return h.Group(out...)
}
