package ui

import (
	"strings"
	"unicode"

	"github.com/emersonjoe/trilha/h"
)

// inline renders the markup inside one run of text: code spans, links,
// images, bold, italic and strikethrough. It is a scanner rather than a chain
// of regular expressions because the order matters — a code span protects
// what is inside it, and a link protects its own label.
func (p *mdParser) inline(s string) h.Node {
	var out []h.Node
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			out = append(out, h.Text(lit.String()))
			lit.Reset()
		}
	}
	for i := 0; i < len(s); {
		switch {
		case s[i] == '\\' && i+1 < len(s) && strings.ContainsRune("\\`*_[]()#+-.!>|~", rune(s[i+1])):
			// An escape is the author saying "this is a character, not
			// markup", and it is the only place a backslash disappears.
			lit.WriteByte(s[i+1])
			i += 2
		case s[i] == '`':
			if n, code, ok := mdCodeSpan(s[i:]); ok {
				flush()
				out = append(out, h.Code(h.Class("ui-code"), h.Text(code)))
				i += n
				continue
			}
			lit.WriteByte(s[i])
			i++
		case s[i] == '!' && i+1 < len(s) && s[i+1] == '[':
			if n, label, dest, ok := mdLink(s[i+1:]); ok {
				flush()
				out = append(out, p.image(label, dest))
				i += n + 1
				continue
			}
			lit.WriteByte(s[i])
			i++
		case s[i] == '[':
			if n, label, dest, ok := mdLink(s[i:]); ok {
				flush()
				out = append(out, p.link(label, dest))
				i += n
				continue
			}
			lit.WriteByte(s[i])
			i++
		case strings.HasPrefix(s[i:], "**"), strings.HasPrefix(s[i:], "__"):
			if n, inner, ok := mdSpan(s[i:], s[i:i+2]); ok {
				flush()
				out = append(out, h.Strong(p.inline(inner)))
				i += n
				continue
			}
			lit.WriteByte(s[i])
			i++
		case strings.HasPrefix(s[i:], "~~"):
			if n, inner, ok := mdSpan(s[i:], "~~"); ok {
				flush()
				out = append(out, h.El("del", p.inline(inner)))
				i += n
				continue
			}
			lit.WriteByte(s[i])
			i++
		case s[i] == '*' || (s[i] == '_' && mdWordBoundary(s, i)):
			if n, inner, ok := mdSpan(s[i:], s[i:i+1]); ok {
				flush()
				out = append(out, h.Em(p.inline(inner)))
				i += n
				continue
			}
			lit.WriteByte(s[i])
			i++
		default:
			lit.WriteByte(s[i])
			i++
		}
	}
	flush()
	return h.Group(out...)
}

func (p *mdParser) link(label, dest string) h.Node {
	href, external, ok := mdURL(dest)
	if !ok {
		// A destination nobody can vouch for does not become a broken link;
		// it becomes what it always was, which is text.
		return h.Group(h.Text("["), p.inline(label), h.Text("]("+dest+")"))
	}
	attrs := []h.Node{h.Href(href)}
	if external {
		// The text came from a model or a stranger: whatever it links to is
		// not this site vouching for it, and it does not get a handle on the
		// window it was opened from.
		attrs = append(attrs, h.Rel("noopener nofollow ugc"))
	}
	return h.A(append(attrs, p.inline(label))...)
}

func (p *mdParser) image(alt, dest string) h.Node {
	src, _, ok := mdURL(dest)
	if !p.opt.Images || !ok {
		return h.Text(alt)
	}
	return h.Img(h.Src(src), h.Alt(alt), h.Class("ui-md-img"), h.Attr("loading", "lazy"))
}

// mdURL says whether a destination may be linked, and whether it points
// outside. The list is closed on purpose: javascript:, data: and anything
// else nobody thought about are not links.
func mdURL(dest string) (href string, external, ok bool) {
	dest = strings.TrimSpace(dest)
	if dest == "" || strings.ContainsAny(dest, " \t\n\"'<>") {
		return "", false, false
	}
	lower := strings.ToLower(dest)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return dest, true, true
	case strings.HasPrefix(lower, "mailto:"):
		return dest, false, true
	case strings.HasPrefix(dest, "/"), strings.HasPrefix(dest, "#"), strings.HasPrefix(dest, "./"):
		return dest, false, true
	}
	// A bare word with no scheme is a relative path, unless the colon in it
	// makes it a scheme nobody listed.
	if i := strings.IndexByte(dest, ':'); i >= 0 {
		if j := strings.IndexAny(dest, "/?#"); j < 0 || j > i {
			return "", false, false
		}
	}
	return dest, false, true
}

// mdCodeSpan reads `code` (or “code“ when the code has a backtick in it).
func mdCodeSpan(s string) (n int, code string, ok bool) {
	open := 0
	for open < len(s) && s[open] == '`' {
		open++
	}
	fence := s[:open]
	rest := s[open:]
	end := strings.Index(rest, fence)
	if end < 0 {
		return 0, "", false
	}
	// A run longer than the fence is not the end of this span.
	for end+len(fence) < len(rest) && rest[end+len(fence)] == '`' {
		next := strings.Index(rest[end+1:], fence)
		if next < 0 {
			return 0, "", false
		}
		end += next + 1
	}
	code = rest[:end]
	if len(code) > 1 && strings.HasPrefix(code, " ") && strings.HasSuffix(code, " ") {
		code = code[1 : len(code)-1]
	}
	return open*2 + end, code, true
}

// mdSpan reads a run delimited by mark on both sides, refusing an empty one.
func mdSpan(s, mark string) (n int, inner string, ok bool) {
	rest := s[len(mark):]
	end := strings.Index(rest, mark)
	if end <= 0 {
		return 0, "", false
	}
	// A delimiter with a space right after it opens nothing: "2 * 3 * 4".
	if strings.HasPrefix(rest, " ") || strings.HasSuffix(rest[:end], " ") {
		return 0, "", false
	}
	return len(mark)*2 + end, rest[:end], true
}

// mdLink reads [label](dest), allowing one level of balanced parentheses in
// the destination — which is what a Wikipedia URL is made of.
func mdLink(s string) (n int, label, dest string, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return 0, "", "", false
	}
	depth, lend := 0, -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				lend = i
			}
		}
		if lend >= 0 {
			break
		}
	}
	if lend < 0 || lend+1 >= len(s) || s[lend+1] != '(' {
		return 0, "", "", false
	}
	depth, end := 0, -1
	for i := lend + 1; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return 0, "", "", false
	}
	dest = s[lend+2 : end]
	// A title after the URL ("url \"title\"") is dropped: it would be one
	// more piece of untrusted text with nowhere safe to go.
	if i := strings.IndexAny(dest, " \t"); i >= 0 {
		dest = dest[:i]
	}
	return end + 1, s[1:lend], dest, true
}

// mdWordBoundary keeps snake_case out of italics: an underscore only opens
// emphasis when it is not between two word characters.
func mdWordBoundary(s string, i int) bool {
	if i > 0 && mdIsWord(rune(s[i-1])) {
		return false
	}
	return true
}

func mdIsWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

func mdWord(s string) bool {
	for _, r := range s {
		if !mdIsWord(r) && r != '-' && r != '+' && r != '.' && r != '#' {
			return false
		}
	}
	return s != ""
}

func mdIndent(line string) int {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

func mdTrimIndent(line string, n int) string {
	for i := 0; i < n && len(line) > 0 && (line[0] == ' ' || line[0] == '\t'); i++ {
		line = line[1:]
	}
	return line
}

func mdBullet(trim string) string {
	for _, m := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trim, m) {
			return m
		}
	}
	return ""
}

func mdNumber(trim string) string {
	n := 0
	for n < len(trim) && trim[n] >= '0' && trim[n] <= '9' {
		n++
	}
	if n == 0 || n > 9 || n+1 >= len(trim) {
		return ""
	}
	if (trim[n] == '.' || trim[n] == ')') && trim[n+1] == ' ' {
		return trim[:n+2]
	}
	return ""
}

func mdIsRule(trim string) bool {
	for _, r := range []byte{'-', '*', '_'} {
		if len(trim) >= 3 && strings.Trim(trim, string(r)+" ") == "" && strings.Count(trim, string(r)) >= 3 {
			return true
		}
	}
	return false
}

func mdIsSeparator(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.ContainsAny(line, "-") {
		return false
	}
	for _, c := range mdCells(line) {
		c = strings.TrimSpace(c)
		if c == "" || strings.Trim(c, ":-") != "" || !strings.Contains(c, "-") {
			return false
		}
	}
	return true
}

func mdCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
