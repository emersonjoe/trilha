package mail

import (
	"html"
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// PlainText is the plain alternative, generated from the same node the HTML
// comes from. Writing it by hand is writing every message twice, and the
// second copy is the one that goes stale.
//
// It is not a general HTML-to-text converter, and does not try to be: the
// input is what package h renders, which is well-formed and has no comments,
// no scripts worth keeping and no CDATA. Blocks become line breaks, a link
// becomes "text <https://…>" — a URL that vanishes is the whole message
// wasted — and entities come back as characters.
func PlainText(n h.Node) string {
	if n == nil {
		return ""
	}
	s, err := h.Render(n)
	if err != nil {
		return ""
	}
	return textFromHTML(s)
}

func textFromHTML(s string) string {
	var out strings.Builder
	var skip string // tag whose content is dropped whole
	var href string // the href of the open <a>, to write after its text
	var anchor strings.Builder

	write := func(text string) {
		if href != "" {
			anchor.WriteString(text)
		}
		out.WriteString(text)
	}

	for len(s) > 0 {
		lt := strings.IndexByte(s, '<')
		if lt < 0 {
			if skip == "" {
				write(html.UnescapeString(s))
			}
			break
		}
		if lt > 0 {
			if skip == "" {
				write(html.UnescapeString(s[:lt]))
			}
			s = s[lt:]
		}
		gt := strings.IndexByte(s, '>')
		if gt < 0 {
			break
		}
		tag := s[1:gt]
		s = s[gt+1:]

		closing := strings.HasPrefix(tag, "/")
		name := strings.ToLower(tagName(strings.TrimPrefix(tag, "/")))
		switch {
		case skip != "":
			if closing && name == skip {
				skip = ""
			}
		case !closing && (name == "style" || name == "script" || name == "head" || name == "title"):
			skip = name
		case name == "a":
			if closing {
				// The URL only earns its place when it is not already the
				// text: "https://x <https://x>" reads like a bug.
				if u := href; u != "" && strings.TrimSpace(anchor.String()) != u {
					out.WriteString(" <" + u + ">")
				}
				href, anchor = "", strings.Builder{}
			} else {
				href = attrValue(tag, "href")
			}
		case name == "br", name == "hr":
			out.WriteString("\n")
		case name == "li", name == "tr":
			// One line each, not one paragraph each: a blank line between
			// two bullets reads like two lists.
			if !closing {
				out.WriteString("\n")
				if name == "li" {
					out.WriteString("- ")
				}
			}
		case block(name):
			out.WriteString("\n")
		}
	}
	return tidy(out.String())
}

func tagName(tag string) string {
	for i := 0; i < len(tag); i++ {
		switch tag[i] {
		case ' ', '\t', '\n', '\r', '/':
			return tag[:i]
		}
	}
	return tag
}

// attrValue reads one attribute out of a tag. It handles what package h
// writes, which is always name="value" with the value escaped.
func attrValue(tag, name string) string {
	low := strings.ToLower(tag)
	for i := 0; ; {
		j := strings.Index(low[i:], name+"=\"")
		if j < 0 {
			return ""
		}
		start := i + j
		// Guard against matching "data-href" when looking for "href".
		if start > 0 && low[start-1] != ' ' && low[start-1] != '\t' {
			i = start + len(name)
			continue
		}
		rest := tag[start+len(name)+2:]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			return ""
		}
		return html.UnescapeString(rest[:end])
	}
}

func block(name string) bool {
	switch name {
	case "p", "div", "tr", "li", "ul", "ol", "table", "blockquote", "section",
		"article", "header", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "pre":
		return true
	}
	return false
}

// tidy turns the raw run into something a person reads: no runs of spaces from
// the source's indentation, no more than one blank line, no trailing space.
//
// Lines are not wrapped. A wrapped line is a wrapped URL, and a URL broken in
// two is a link that does not work — quoted-printable already handles the
// length limit, with soft breaks that the receiving client undoes.
func tidy(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, strings.TrimSpace(strings.Join(strings.Fields(line), " ")))
	}
	var out []string
	blank := 0
	for _, line := range lines {
		if line == "" {
			blank++
			if blank > 1 || len(out) == 0 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
