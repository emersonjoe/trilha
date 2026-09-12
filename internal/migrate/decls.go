package migrate

import (
	"regexp"
	"strings"
)

// This file answers one question about a module: which part of it is this
// page's problem. A page that imports two names from a file of forty functions
// inherits, today, everything the other thirty-eight do — and a screen that
// reads an invitation and sets a password came back as an upload island
// because the module it imports has an upload function in it (#167). The
// `import { a, b } from "x"` already says which names came in; following only
// those answers the question without a TypeScript parser.

// declRe finds a declaration at the top level of a module: the ones it exports
// and the ones it keeps to itself, which count too when something exported
// calls them. Column zero is the whole heuristic — a nested declaration is
// indented in anything a formatter has touched — and it is the same one the
// export scanner above already trusts.
var declRe = regexp.MustCompile(`(?m)^(export[ \t]+)?(?:default[ \t]+)?(?:async[ \t]+)?(function\*?|const|let|var|class)[ \t]+([A-Za-z_$][\w$]*)`)

// funcInitRe says that a const holds a function and not a value:
// `const upload = async (f: File) => {...}`, which is how half of TypeScript
// writes a function. The distinction is what runs when the module is imported.
// A function body runs when something calls it, so it belongs to whoever
// imported the name; a value — `const es = new EventSource("/api/chat")` — runs
// at import, and belongs to everybody who imports anything from the file.
// Anything this expression is not sure about is read as a value, which is the
// side that keeps a signal instead of losing one.
var funcInitRe = regexp.MustCompile(`^(?:export[ \t]+)?(?:const|let|var)[ \t]+[A-Za-z_$][\w$]*[^=\n]*=[ \t]*(?:async[ \t]+)?(?:function\b|\(|[A-Za-z_$][\w$]*[ \t]*=>)`)

// wordRe is every word of a region, which is how one declaration is seen to
// mention another. A name matched here may be a property or a string; keeping a
// declaration that nothing calls is the harmless direction.
var wordRe = regexp.MustCompile(`[A-Za-z_$][\w$]*`)

// decl is one top-level declaration: its name, whether the module exports it,
// and the text that belongs to it — from its first line to the line the next
// declaration starts on.
type decl struct {
	name       string
	exported   bool
	runs       bool // a value, not a function: importing the module runs it
	start, end int
}

// declsOf lists the top-level declarations of a module, in the order they are
// written.
func declsOf(src string) []decl {
	ms := declRe.FindAllStringSubmatchIndex(src, -1)
	out := make([]decl, 0, len(ms))
	for i, m := range ms {
		end := len(src)
		if i+1 < len(ms) {
			end = ms[i+1][0]
		}
		keyword := src[m[4]:m[5]]
		value := keyword != "class" && !strings.HasPrefix(keyword, "function")
		out = append(out, decl{
			name:     src[m[6]:m[7]],
			exported: m[2] >= 0,
			runs:     value && !funcInitRe.MatchString(src[m[0]:end]),
			start:    m[0],
			end:      end,
		})
	}
	return out
}

// usedSource is the module as this page uses it: everything outside the
// declarations it asked for is blanked out, and the report says whether that
// happened. The blanking keeps the newlines — every other byte becomes a space
// — so the line the reason prints is still the line of the real file, which is
// what makes a wrong suggestion cheap to throw out.
//
// It returns the source unchanged, and whole = true, whenever nobody said which
// names came in: a default or namespace import asks for everything, and a name
// that is not a declaration of this file (a re-export, a type, a shape the
// expressions here did not see) is read the cautious way. A signal missed is a
// screen somebody starts porting as a form and abandons halfway.
func usedSource(src string, want map[string]bool) (string, bool) {
	if len(want) == 0 || want["*"] {
		return src, true
	}
	ds := declsOf(src)
	at := map[string]int{}
	for i, d := range ds {
		if _, seen := at[d.name]; !seen {
			at[d.name] = i
		}
	}
	keep := make(map[int]bool, len(ds))
	var queue []int
	for name := range want {
		if i, ok := at[name]; ok && !keep[i] {
			keep[i] = true
			queue = append(queue, i)
		}
	}
	if len(queue) == 0 {
		return src, true
	}
	// And whatever the module does on its own account stays: a top-level value
	// is evaluated the moment anything imports the file, so an EventSource
	// opened beside the functions belongs to every screen that touches it.
	for i, d := range ds {
		if d.runs && !keep[i] {
			keep[i] = true
			queue = append(queue, i)
		}
	}
	// What a kept declaration mentions is kept too: entrarNoPortal may call a
	// post() beside it, and stopping at the first level would lose the signal
	// that is really there.
	for len(queue) > 0 {
		d := ds[queue[0]]
		queue = queue[1:]
		for _, name := range wordRe.FindAllString(src[d.start:d.end], -1) {
			if i, ok := at[name]; ok && !keep[i] {
				keep[i] = true
				queue = append(queue, i)
			}
		}
	}
	b := []byte(src)
	for i, d := range ds {
		if keep[i] {
			continue
		}
		for o := d.start; o < d.end; o++ {
			if b[o] != '\n' {
				b[o] = ' '
			}
		}
	}
	return string(b), false
}

// identsIn is the set of words of a region: what a file can be said to use.
func identsIn(src string) map[string]bool {
	out := map[string]bool{}
	for _, name := range wordRe.FindAllString(src, -1) {
		out[name] = true
	}
	return out
}
