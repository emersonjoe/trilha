package uitest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The fields every case of JORNADAS.md has: what a person reading the catalog
// needs to run the case by hand, and to know what is lost if it breaks.
var caseFields = []string{"**Jornada**", "**Risco**", "**Pré-condições**", "**Dados**", "**Passos**", "**Resultado esperado**"}

var caseHeading = regexp.MustCompile(`(?m)^### (TestUI\w+) — \S`)

// Every browser scenario is a documented case and every case is a scenario:
// the catalog is the list of journeys this module protects, and it cannot
// drift from the code that protects them.
func TestCatalogoDeJornadas(t *testing.T) {
	b, err := os.ReadFile("JORNADAS.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	cases := map[string]string{}
	locs := caseHeading.FindAllStringSubmatchIndex(doc, -1)
	for i, m := range locs {
		end := len(doc)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		cases[doc[m[2]:m[3]]] = doc[m[0]:end]
	}

	scenarios := map[string]bool{}
	files, _ := filepath.Glob("*_test.go")
	fset := token.NewFileSet()
	for _, f := range files {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "TestUI") {
				scenarios[fn.Name.Name] = true
			}
		}
	}

	var names []string
	for n := range scenarios {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		body, ok := cases[n]
		if !ok {
			t.Errorf("%s has no case in JORNADAS.md: add a \"### %s — <title>\" section with %s", n, n, strings.Join(caseFields, ", "))
			continue
		}
		for _, f := range caseFields {
			if !strings.Contains(body, f) {
				t.Errorf("JORNADAS.md, %s: missing %s", n, f)
			}
		}
		if !strings.Contains(doc[:strings.Index(doc, "## Casos")], n) {
			t.Errorf("JORNADAS.md: %s is not in any journey of the critical journeys table", n)
		}
	}
	for n := range cases {
		if !scenarios[n] {
			t.Errorf("JORNADAS.md documents %s, which no test file declares: remove the case or write the scenario", n)
		}
	}
}
