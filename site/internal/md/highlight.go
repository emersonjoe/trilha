package md

import (
	"html"
	"strings"
	"unicode"
)

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true, "var": true,
}

var goTypes = map[string]bool{
	"string": true, "int": true, "int64": true, "bool": true, "error": true, "any": true, "byte": true,
	"rune": true, "float64": true, "nil": true, "true": true, "false": true, "iota": true,
}

// HighlightGo wraps Go tokens in spans: comments (c), strings (s), keywords
// (k), builtin types/constants (t), numbers (n) and function names (f).
func HighlightGo(src string) string {
	var sb strings.Builder
	rs := []rune(src)
	i := 0
	emit := func(class, text string) {
		if class == "" {
			sb.WriteString(html.EscapeString(text))
			return
		}
		sb.WriteString(`<span class="` + class + `">` + html.EscapeString(text) + `</span>`)
	}
	for i < len(rs) {
		c := rs[i]
		switch {
		case c == '/' && i+1 < len(rs) && rs[i+1] == '/':
			j := i
			for j < len(rs) && rs[j] != '\n' {
				j++
			}
			emit("c", string(rs[i:j]))
			i = j
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			j := i + 2
			for j+1 < len(rs) && !(rs[j] == '*' && rs[j+1] == '/') {
				j++
			}
			j = min(j+2, len(rs))
			emit("c", string(rs[i:j]))
			i = j
		case c == '"' || c == '`' || c == '\'':
			j := i + 1
			for j < len(rs) && rs[j] != c {
				if rs[j] == '\\' && c != '`' {
					j++
				}
				j++
			}
			j = min(j+1, len(rs))
			emit("s", string(rs[i:j]))
			i = j
		case unicode.IsLetter(c) || c == '_':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			word := string(rs[i:j])
			switch {
			case goKeywords[word]:
				emit("k", word)
			case goTypes[word]:
				emit("t", word)
			case j < len(rs) && rs[j] == '(':
				emit("f", word)
			default:
				emit("", word)
			}
			i = j
		case unicode.IsDigit(c):
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.' || rs[j] == '_' || rs[j] == 'x') {
				j++
			}
			emit("n", string(rs[i:j]))
			i = j
		default:
			emit("", string(c))
			i++
		}
	}
	return sb.String()
}
