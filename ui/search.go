package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// SearchBoxOpts configures the box.
type SearchBoxOpts struct {
	// Name is the query parameter (default "q"). It is what the results page
	// reads with c.Query, so the two have to say the same word.
	Name string
	// Value is what is already typed — the current query, so the box still
	// holds it after the results load.
	Value string
	// Placeholder overrides the default sentence.
	Placeholder string
	// Hint is the shortcut written beside the box. Empty writes the platform's
	// own; "-" writes none, which is what an application with its own shortcut
	// wants.
	Hint string
}

// SearchBox is the search field of the top bar.
//
//	ui.SearchBox(c, "/busca", ui.SearchBoxOpts{Value: c.Query("q")})
//
// It is a GET form: the query ends up in the address, so a search can be sent
// to somebody, kept in a bookmark and found again in the history. Ctrl+K and
// "/" move the focus here, which the kit's script does — without JavaScript the
// box is still a form, and the shortcut is the only thing missing.
func SearchBox(c *trilha.Ctx, action string, o SearchBoxOpts) h.Node {
	w := searchWords(c)
	name := o.Name
	if name == "" {
		name = "q"
	}
	placeholder := o.Placeholder
	if placeholder == "" {
		placeholder = w["placeholder"]
	}
	field := []h.Node{
		h.Type("search"), h.Name(name), h.Value(o.Value),
		h.Placeholder(placeholder), h.Aria("label", w["label"]),
		h.Data("ui-search", ""), h.Autocomplete("off"),
	}
	kids := []h.Node{Input(field...)}
	if o.Hint != "-" {
		hint := o.Hint
		if hint == "" {
			hint = "Ctrl K"
		}
		kids = append(kids, Kbd(hint))
	}
	return h.Form(h.Method("get"), h.Action(action), h.Role("search"),
		h.Class("ui-search-box"), h.Group(kids...))
}

// SearchResultsOpts configures the results.
type SearchResultsOpts struct {
	// Empty is the sentence for a search that found nothing. Empty uses the
	// kit's, which names the query back — a "no results" that does not repeat
	// what was searched leaves somebody wondering what was searched.
	Empty string
	// Hint is the smaller line under it: what to try instead.
	Hint string
	// More is a link per group, given the kind, for "see all 42". Nil draws
	// none, which is right until the application has a listing per type.
	More func(kind string) string
}

// SearchResults renders what trilha.Search found: one section per kind, in the
// order the kinds were declared, each with its count.
//
//	res, _ := Busca.Query(c, c.Query("q"), trilha.SearchQuery{Limit: 10})
//	ui.SearchResults(c, res, ui.SearchResultsOpts{})
//
// The matched words are marked with <mark>, and the tag is written here and not
// by the runtime: the snippet arrives cut into parts, so a body containing
// <script> is text on the way in and text on the way out.
func SearchResults(c *trilha.Ctx, res trilha.SearchResult, o SearchResultsOpts) h.Node {
	w := searchWords(c)
	if res.Query == "" {
		return Empty(EmptyOpts{Icon: "search", Title: w["ask"], Hint: w["ask hint"]})
	}
	if res.Total == 0 {
		title := o.Empty
		if title == "" {
			title = w["empty"] + " “" + res.Query + "”."
		}
		hint := o.Hint
		if hint == "" {
			hint = w["empty hint"]
		}
		return Empty(EmptyOpts{Icon: "search", Title: title, Hint: hint})
	}
	sections := make([]h.Node, 0, len(res.Groups))
	for _, g := range res.Groups {
		sections = append(sections, searchGroup(c, g, o, w))
	}
	return h.Div(h.Class("ui-search-results"), h.Group(sections...))
}

func searchGroup(c *trilha.Ctx, g trilha.SearchGroup, o SearchResultsOpts, w map[string]string) h.Node {
	rows := make([]h.Node, 0, len(g.Hits))
	for _, hit := range g.Hits {
		rows = append(rows, h.Li(h.Class("ui-search-hit"),
			searchTitle(hit),
			h.P(h.Class("ui-search-snippet"), h.Group(marked(hit.Snippet)...)),
		))
	}
	head := []h.Node{
		h.Span(h.Class("ui-search-kind"), h.Text(g.Label)),
		Badge(Outline(), h.Text(strconv.Itoa(g.Total))),
	}
	if o.More != nil && g.Total > len(g.Hits) {
		if href := o.More(g.Kind); href != "" {
			head = append(head, h.A(h.Class("ui-search-more"), h.Href(href), h.Text(w["see all"])))
		}
	}
	return h.Section(h.Class("ui-search-group"),
		h.H2(h.Class("ui-search-heading"), h.Group(head...)),
		h.Ul(h.Class("ui-search-list"), h.Group(rows...)),
	)
}

func searchTitle(hit trilha.Hit) h.Node {
	if hit.URL == "" {
		return h.Strong(h.Text(hit.Title))
	}
	return h.A(h.Class("ui-search-title"), h.Href(hit.URL), h.Text(hit.Title))
}

// marked turns the parts into text and <mark>. It is the whole reason the
// runtime hands back parts instead of a string.
func marked(s trilha.Snippet) []h.Node {
	out := make([]h.Node, 0, len(s))
	for _, p := range s {
		if p.Match {
			out = append(out, h.Mark(h.Text(p.Text)))
			continue
		}
		out = append(out, h.Text(p.Text))
	}
	return out
}

func searchWords(c *trilha.Ctx) map[string]string {
	if langOf(c) == "pt-BR" {
		return map[string]string{
			"label": "Buscar", "placeholder": "Buscar…", "see all": "ver todos",
			"ask": "O que você procura?", "ask hint": "Digite um nome, um número, um trecho.",
			"empty": "Nada encontrado para", "empty hint": "Tente menos palavras, ou parte do nome.",
		}
	}
	return map[string]string{
		"label": "Search", "placeholder": "Search…", "see all": "see all",
		"ask": "What are you looking for?", "ask hint": "Type a name, a number, a fragment.",
		"empty": "Nothing found for", "empty hint": "Try fewer words, or part of the name.",
	}
}
