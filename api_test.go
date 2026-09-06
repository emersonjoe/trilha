package trilha_test

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/apisurface"
)

var update = flag.Bool("update", false, "rewrite api/current.txt")

// arquivoDaAPI is the written promise: every exported symbol of every public
// package, one per line. A change to it is a change to what the framework
// promises, and the diff is where that gets noticed.
const arquivoDaAPI = "api/current.txt"

var pacotesPublicos = []apisurface.Package{
	{Dir: ".", Name: "trilha"},
	{Dir: "ai", Name: "ai"},
	{Dir: "ai/mcp", Name: "ai/mcp"},
	{Dir: "auth", Name: "auth"},
	{Dir: "cache", Name: "cache"},
	{Dir: "h", Name: "h"},
	{Dir: "tmpl", Name: "tmpl"},
	{Dir: "ui", Name: "ui"},
}

func TestSuperficiePublica(t *testing.T) {
	atual, err := apisurface.Render(".", pacotesPublicos)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.MkdirAll(filepath.Dir(arquivoDaAPI), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(arquivoDaAPI, []byte(atual), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	bruto, err := os.ReadFile(arquivoDaAPI)
	if err != nil {
		t.Fatal(err)
	}
	entrou, saiu := diferenca(string(bruto), atual)
	if len(entrou) == 0 && len(saiu) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("a superfície pública mudou.\n")
	for _, l := range saiu {
		sb.WriteString("- " + l + "\n")
	}
	for _, l := range entrou {
		sb.WriteString("+ " + l + "\n")
	}
	sb.WriteString("\nSe a mudança é intencional: `make api` regrava " + arquivoDaAPI +
		", e o diff dele entra na revisão. Remoção de símbolo exige o ciclo de\n" +
		"depreciação descrito em API.md.")
	t.Fatal(sb.String())
}

func diferenca(antigo, novo string) (entrou, saiu []string) {
	conjunto := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, l := range strings.Split(s, "\n") {
			if l != "" {
				m[l] = true
			}
		}
		return m
	}
	a, n := conjunto(antigo), conjunto(novo)
	for l := range n {
		if !a[l] {
			entrou = append(entrou, l)
		}
	}
	for l := range a {
		if !n[l] {
			saiu = append(saiu, l)
		}
	}
	sort.Strings(entrou)
	sort.Strings(saiu)
	return entrou, saiu
}

// TestPacotesPublicosNaLista guards the guard: a public package nobody added
// to the list would have no surface at all, and nothing would say so.
func TestPacotesPublicosNaLista(t *testing.T) {
	listados := map[string]bool{}
	for _, p := range pacotesPublicos {
		listados[p.Dir] = true
	}
	fora := map[string]bool{
		"api": true, "bench": true, "cmd": true, "docs": true,
		"examples": true, "internal": true, "scripts": true, "site": true,
		"specs": true, "testdata": true,
	}
	err := filepath.WalkDir(".", func(caminho string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		nome := d.Name()
		if caminho != "." && (strings.HasPrefix(nome, ".") || strings.HasPrefix(nome, "_") || fora[caminho]) {
			return fs.SkipDir
		}
		if !temFonteGo(t, caminho) {
			return nil
		}
		if !listados[filepath.ToSlash(caminho)] {
			t.Errorf("pacote público %q fora de pacotesPublicos: acrescente-o ou mova-o para internal/", caminho)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func temFonteGo(t *testing.T, dir string) bool {
	t.Helper()
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			return true
		}
	}
	return false
}
