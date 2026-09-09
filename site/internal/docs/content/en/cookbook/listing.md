---
title: A listing that lives in the URL
description: Page, ordering, filter and a queue that refreshes itself, with ListParams, ui.DataTable and ui.Poll.
---

The screen is `/documentos` in `examples/blog`: a table of documents with a search box, a
type filter, sortable columns, pagination — and, above it, a block that keeps up with the
processing queue on its own. Everything the visitor chooses is in the address, and the
only script on the page is the kit's.

## What the screen reads

The struct of the screen embeds `trilha.ListParams` and adds the filter only it has.
`c.Bind` reads both from the query in one go and applies the limits of the listing (page
at least 1, page size capped at `trilha.MaxPerPage`):

```go
type consulta struct {
	trilha.ListParams
	Tipo string `form:"tipo"`
}
```

## What the table shows

The columns are declared once. `Sort: true` does two things: it turns the header into a
link, and it says the column may be ordered by — a `sort` in the address that is not one
of these is dropped before the query, so the repository never receives a column name
nobody declared.

```go
var colunas = ui.Columns[documentos.Documento]{
	{Key: "nome", Label: "Documento", Sort: true, Cell: func(d documentos.Documento) h.Node {
		return h.Text(d.Nome)
	}},
	{Key: "tipo", Label: "Tipo", Cell: func(d documentos.Documento) h.Node {
		return ui.Badge(ui.Outline(), h.Text(d.Tipo))
	}},
	{Key: "tamanho", Label: "Tamanho", Sort: true, Num: true, Cell: func(d documentos.Documento) h.Node {
		return h.Text(d.Tamanho())
	}},
	{Key: "status", Label: "Status", Sort: true, Cell: func(d documentos.Documento) h.Node {
		return ui.Badge(h.Text(d.Status))
	}},
}
```

## One handler for the page and for the pieces

`c.Fragment()` says who is asking. The whole page, the table alone when the kit swaps the
listing, or the queue block at every tick of `ui.Poll`:

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "fila" {
		// O tique é o worker deste exemplo: a fila anda, e quando não sobra
		// nada o servidor encerra o polling na própria resposta.
		documentos.Andar()
		return fila(c), nil
	}
	var q consulta
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	q.Tipo = tipoValido(q.Tipo)
	tabela := lista(c, q)
	if c.Fragment() == "lista" {
		return tabela, nil
	}
	c.SetTitle("Documentos")
	return h.Div(
		ui.H1(h.Text("Documentos")),
		fila(c),
		// O resumo varre a lista inteira e é a parte lenta desta tela. Com o
		// Defer a página chega pronta sem ele, e ele entra sozinho um instante
		// depois; sem JavaScript, o placeholder carrega um link para a mesma
		// rota, que responde como página.
		ui.Defer(c, "resumo", "/documentos/resumo", ui.DeferOpts{Height: "9rem"}),
		tabela,
		ui.LiveScript(c),
	), nil
}
```

`ui.LiveScript(c)` is the one extra script, and only on the pages that watch something:
`ui.Head` does not load `ui.live.js`.

## The table itself

`ui.DataTable` builds the ordering links, the filter form, the pagination and the empty
state from `ListState`. With `ID` set, all of them carry `ui.Swap("lista")`: ordering,
filtering and paging swap the table instead of reloading — and with JavaScript off the
same links navigate, because they are links.

```go
func lista(c *trilha.Ctx, q consulta) h.Node {
	docs, total := documentos.Buscar(documentos.Consulta{
		Q: q.Q, Tipo: q.Tipo, Ordem: q.Sort, Asc: q.Asc(),
		Offset: q.Offset(), Limite: q.Limit(),
	})
	return ui.DataTable(c, colunas, docs, ui.ListState{
		Params:  q.ListParams,
		Total:   total,
		ID:      "lista",
		Search:  "Buscar no nome",
		Filters: filtro(q.Tipo),
		Caption: "Documentos recebidos",
		RowHref: func(i int) string { return "/documentos?q=" + docs[i].Nome },
		// Without this the DataTable already draws a sensible empty state, and
		// it tells "no documents" from "no results for that term". This one is
		// here because the app speaks Portuguese and the kit's default does not.
		Empty: ui.Empty(ui.EmptyOpts{
			Icon:  "info",
			Title: "Nenhum documento com esse filtro",
			Hint:  "Tente outro termo, ou limpe a busca.",
		}),
	})
}
```

The filter of this screen is an ordinary `<select>`; the component puts it inside the same
`<form method=get>` as the search box, so filtering is changing the address:

```go
func filtro(sel string) h.Node {
	opcoes := []ui.Option{{Value: "", Label: "Todos os tipos"}}
	for _, t := range documentos.Tipos() {
		opcoes = append(opcoes, ui.Option{Value: t, Label: t})
	}
	return ui.Select(h.Name("tipo"), h.Aria("label", "Tipo"), ui.SelectOptions(opcoes, sel))
}
```

What comes from the URL only reaches the repository if it is one of the values the screen
offers — the same idea `Restrict` applies to the ordering:

```go
func tipoValido(t string) string {
```

## The queue that refreshes itself

The block asks for itself every two seconds, and the server ends it when there is nothing
left to wait for. The answer is still the fragment, so the last state stays on screen:

```go
func fila(c *trilha.Ctx) h.Node {
	n := documentos.Pendentes()
	if n == 0 {
		c.PollStop()
		return h.Div(h.ID("fila"), ui.Poll("2s", ""),
			ui.Alert("Fila vazia", ui.AlertDescription(h.Text("Todos os documentos foram processados."))))
	}
	return h.Div(h.ID("fila"), ui.Poll("2s", ""),
		ui.Alert("Processando", ui.AlertDescription(h.Textf("%d documentos ainda na fila.", n))))
}
```

## When the server has the events

Polling is the floor, not the ceiling. If your app already knows when something changed,
open one stream per page with `ui.Live("/events")`, mark the fragment with
`ui.On("name", src)` and send the name — never the HTML — from the route:

```go
func GET(c *trilha.Ctx) error {
	s := c.Stream()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; i < 3; i++ {
		select {
		case <-s.Done():
			return nil
		case <-t.C:
			if err := s.Notify("painel:agora"); err != nil {
				return err
			}
		}
	}
	return nil
}
```

The client asks the fragment's route again when it hears the name, so the authorization and
the rendering stay where they already are. A fragment with `ui.On` and `ui.Poll` together
falls back to the clock while the connection is down. See
[Live fragments](/reference/live) and [Listings](/reference/listings).
