package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func busca(t *testing.T) *trilha.Search {
	t.Helper()
	s := trilha.NewSearch(trilha.SearchOpts{}).
		Kind("pessoa", trilha.KindOpts{Label: "Pessoas"}).
		Kind("processo", trilha.KindOpts{Label: "Processos"})
	if err := s.Put(nil,
		trilha.Doc{Kind: "pessoa", ID: "1", Title: "João da Silva",
			Body: "joao@example.com", URL: "/pessoas/1"},
		trilha.Doc{Kind: "processo", ID: "9", Title: "Processo 2024/07",
			Body: "objeto do <script>alert(1)</script> joao", URL: "/processos/9"},
	); err != nil {
		t.Fatal(err)
	}
	return s
}

// #149 — o resultado sai agrupado, com contador e o trecho marcado. E o <mark>
// é escrito aqui, não pelo runtime: um corpo com <script> entra texto e sai
// texto.
func TestSearchResults(t *testing.T) {
	s := busca(t)
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		res, err := s.Query(c, "joao", trilha.SearchQuery{})
		if err != nil {
			t.Fatal(err)
		}
		return SearchResults(c, res, SearchResultsOpts{})
	})
	for _, quero := range []string{
		"Pessoas", "Processos", `href="/pessoas/1"`, `href="/processos/9"`,
		"<mark>", "ui-search-group",
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Fatalf("o corpo saiu como HTML:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("o corpo devia sair escapado:\n%s", got)
	}
	// A palavra inteira é marcada, não o prefixo: meia palavra em amarelo
	// parece defeito de renderização.
	if !strings.Contains(got, "<mark>João</mark>") && !strings.Contains(got, "<mark>joao</mark>") {
		t.Fatalf("marcação parcial:\n%s", got)
	}
}

// Sem busca nenhuma a tela convida; com busca e sem resultado, ela repete o que
// foi procurado — um "nada encontrado" que não diz o quê deixa a dúvida.
func TestSearchResultsVazio(t *testing.T) {
	s := busca(t)
	semNada := chatPage(t, func(c *trilha.Ctx) h.Node {
		res, _ := s.Query(c, "", trilha.SearchQuery{})
		return SearchResults(c, res, SearchResultsOpts{})
	})
	if !strings.Contains(semNada, "What are you looking for?") {
		t.Fatalf("convite:\n%s", semNada)
	}
	semResultado := chatPage(t, func(c *trilha.Ctx) h.Node {
		res, _ := s.Query(c, "helicoptero", trilha.SearchQuery{})
		return SearchResults(c, res, SearchResultsOpts{})
	})
	if !strings.Contains(semResultado, "helicoptero") {
		t.Fatalf("o vazio não repete a busca:\n%s", semResultado)
	}
}

// A caixa é um formulário GET: a busca vai para o endereço, então pode ser
// mandada para alguém, guardada e achada de novo no histórico.
func TestSearchBox(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return SearchBox(c, "/busca", SearchBoxOpts{Value: "joao"})
	})
	for _, quero := range []string{
		`method="get"`, `action="/busca"`, `role="search"`,
		`name="q"`, `value="joao"`, `data-ui-search`, "Ctrl K",
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	if semAtalho := chatPage(t, func(c *trilha.Ctx) h.Node {
		return SearchBox(c, "/busca", SearchBoxOpts{Hint: "-"})
	}); strings.Contains(semAtalho, "ui-kbd") {
		t.Fatalf("o atalho apareceu com Hint \"-\":\n%s", semAtalho)
	}
}
