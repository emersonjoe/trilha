package recipes

// searchRecipe is the one box over several types: the index, the page, and the
// field in the layout.
//
// What it deliberately leaves to the project is the one thing only the project
// knows: which records go in. The declaration of a kind is three lines, and a
// recipe that guessed them would write an index of nothing.
func searchRecipe() Recipe {
	return Recipe{
		Name: "search",
		Summary: map[string]string{
			"en": "one search box over several kinds of thing, grouped by kind",
			"pt": "uma caixa de busca sobre vários tipos, agrupada por tipo",
		},
		Doc: "/reference/search",
		Files: []File{
			{Rel: "internal/busca/busca.go", Go: true, Body: searchDecl},
			{Rel: "internal/busca/busca_test.go", Go: true, Body: searchDeclTest},
			{Rel: "{{.At}}busca/page.go", Go: true, Body: searchPage},
			{Rel: "busca_test.go", Go: true, Body: searchTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add search",
			Line:   "\tbusca.Setup(a)\n",
		}},
		Imports: []string{"{{.Module}}/internal/busca"},
		Next: map[string]string{
			"en": "Declare what goes in: one `Kind` per type in internal/busca, and a `busca.Indice.Put` " +
				"where each one is saved. Then run `trilha dev`, open {{.URL}}busca?q=… and put " +
				"`ui.SearchBox(c, \"{{.URL}}busca\", ui.SearchBoxOpts{Value: c.Query(\"q\")})` in your layout.",
			"pt": "Declare o que entra: um `Kind` por tipo no internal/busca, e um `busca.Indice.Put` " +
				"onde cada um é salvo. Depois rode `trilha dev`, abra {{.URL}}busca?q=… e ponha o " +
				"`ui.SearchBox(c, \"{{.URL}}busca\", ui.SearchBoxOpts{Value: c.Query(\"q\")})` no seu layout.",
		},
	}
}

const searchDecl = `// Package busca is the index behind the box in the top bar.
//
// One index, several kinds of thing, results grouped by kind. What this file
// owns is the list of kinds and where each record's title, body and address
// come from — the only part of a search only this application knows.
package busca

import (
	"github.com/emersonjoe/trilha"
)

// Indice is the index. It is a package variable because there is one search in
// an application, and the handler that saves a record has to reach the same one
// the page queries.
var Indice = trilha.NewSearch(trilha.SearchOpts{})

// Setup declares the kinds and hands the index to the application.
//
// Add one Kind per type you want found. A kind nobody declared is refused by
// Put with a message that says so, which is better than a record that silently
// never turns up.
//
// Module is the area of the application the kind belongs to; with an Allow in
// SearchOpts, a kind whose module the user may not see disappears from the
// results — count included. Wire Allow to your policy when you have one.
func Setup(a *trilha.App) {
	Indice.Kind("exemplo", trilha.KindOpts{Label: "{{.T.search_kind}}"})
	trilha.Provide(a, Indice)
}

// Indexar is where a record becomes a search document. Call it after saving,
// and call Indice.Delete after deleting: an index nobody maintains is a search
// that finds what is not there any more.
//
//	if err := busca.Indexar(c, "exemplo", p.ID, p.Nome, p.Email, "/exemplos/"+p.ID); err != nil { … }
func Indexar(c *trilha.Ctx, tipo, id, titulo, corpo, url string) error {
	return Indice.Put(c, trilha.Doc{Kind: tipo, ID: id, Title: titulo, Body: corpo, URL: url})
}
`

const searchDeclTest = `package busca

import (
	"testing"

	"github.com/emersonjoe/trilha"
)

// O que o índice acha é o que foi declarado: um tipo não declarado é recusado,
// e acento não muda a resposta.
func TestIndiceAchaOQueFoiIndexado(t *testing.T) {
	i := trilha.NewSearch(trilha.SearchOpts{}).Kind("exemplo", trilha.KindOpts{Label: "Exemplos"})
	if err := i.Put(nil, trilha.Doc{Kind: "exemplo", ID: "1", Title: "João da Silva", URL: "/exemplos/1"}); err != nil {
		t.Fatal(err)
	}
	res, err := i.Query(nil, "joao", trilha.SearchQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("achou %d: %+v", res.Total, res.Groups)
	}
	if err := i.Put(nil, trilha.Doc{Kind: "nao-declarado", ID: "2"}); err == nil {
		t.Fatal("gravou um tipo que ninguém declarou")
	}
}
`

const searchPage = `// Package busca is the results screen: what was found, grouped by kind.
package busca

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET {{.URL}}busca?q=…
//
// The index arrives through the application and not through an import, for the
// reason this folder is called busca too: the screen and the package that owns
// the index have the same name, and a screen that reached into it would be a
// package importing itself.
//
// The query is in the address on purpose: a search can then be sent to
// somebody, kept in a bookmark and found again in the history.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.search_title}}")
	q := c.Query("q")
	res, err := trilha.Use[*trilha.Search](c).Query(c, q, trilha.SearchQuery{Limit: 10})
	if err != nil {
		return nil, err
	}
	return ui.Stack(
		ui.PageHeader("{{.T.search_title}}"),
		ui.SearchBox(c, "{{.URL}}busca", ui.SearchBoxOpts{Value: q}),
		ui.SearchResults(c, res, ui.SearchResultsOpts{}),
	), nil
}
`

const searchTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/busca"
)

// A página responde com a caixa mesmo sem busca, e acha o que foi indexado.
func TestPaginaDeBusca(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	c := trilha.NewTestClient(t, a)

	vazia := c.Get("{{.URL}}busca").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(vazia, "role=" + "\"search\"") {
		t.Fatalf("a caixa não apareceu:\n%s", vazia)
	}

	if err := busca.Indice.Put(nil, trilha.Doc{
		Kind: "exemplo", ID: "1", Title: "João da Silva", URL: "/exemplos/1",
	}); err != nil {
		t.Fatal(err)
	}
	// Sem acento e em caixa baixa: é o que todo mundo digita.
	corpo := c.Get("{{.URL}}busca?q=joao").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(corpo, "/exemplos/1") {
		t.Fatalf("não achou o que foi indexado:\n%s", corpo)
	}
}
`
