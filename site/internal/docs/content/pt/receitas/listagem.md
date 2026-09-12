---
title: Uma listagem que mora na URL
description: Página, ordem, filtro e uma fila que se atualiza sozinha, com ListParams, ui.DataTable e ui.Poll.
---

A tela é a `/documentos` do `examples/blog`: uma tabela de documentos com busca, filtro de
tipo, colunas ordenáveis, paginação — e, acima dela, um bloco que acompanha a fila de
processamento sozinho. Tudo que o visitante escolhe está no endereço, e o único script da
página é o do kit.

## O que a tela lê

A struct da tela embute a `trilha.ListParams` e acrescenta o filtro que só ela tem. O
`c.Bind` lê os dois da query de uma vez e aplica os limites da listagem (página no mínimo
1, tamanho no teto do `trilha.MaxPerPage`):

```go
type consulta struct {
	trilha.ListParams
	Tipo string `form:"tipo"`
}
```

## O que a tabela mostra

As colunas são declaradas uma vez. O `Sort: true` faz duas coisas: transforma o cabeçalho
em link e diz que dá para ordenar por ela — um `sort` no endereço que não seja uma destas
é derrubado antes da consulta, então o repositório nunca recebe nome de coluna que
ninguém declarou.

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

## Um handler para a página e para os pedaços

O `c.Fragment()` diz quem está perguntando. A página inteira, só a tabela quando o kit
troca a listagem, ou o bloco da fila a cada tique do `ui.Poll`:

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

O `ui.LiveScript(c)` é o único script a mais, e só nas páginas que observam alguma coisa:
o `ui.Head` não carrega o `ui.live.js`.

## A tabela

O `ui.DataTable` monta os links de ordem, o formulário de filtro, a paginação e o estado
vazio a partir do `ListState`. Com o `ID` preenchido, todos levam o `ui.Swap("lista")`:
ordenar, filtrar e paginar trocam a tabela em vez de recarregar — e sem JavaScript os
mesmos links navegam, porque são links.

`Cards: true` é o que mantém quatro colunas legíveis no celular: abaixo de 640px cada
linha vira um cartão, com o rótulo da própria coluna — `Columns[T].Label`, o mesmo que o
cabeçalho já mostra — ao lado do valor, em vez de uma tabela que só rola de lado.

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
		// A listagem tem quatro colunas e é aberta no celular tanto quanto no
		// desktop: a linha vira cartão abaixo de 640px em vez de rolar de lado.
		Cards: true,
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

O filtro desta tela é um `<select>` comum; o componente o põe dentro do mesmo
`<form method=get>` da busca, então filtrar é mudar o endereço:

```go
func filtro(sel string) h.Node {
	opcoes := []ui.Option{{Value: "", Label: "Todos os tipos"}}
	for _, t := range documentos.Tipos() {
		opcoes = append(opcoes, ui.Option{Value: t, Label: t})
	}
	return ui.Select(h.Name("tipo"), h.Aria("label", "Tipo"), ui.SelectOptions(opcoes, sel))
}
```

O que vem da URL só chega ao repositório se for um dos valores que a tela oferece — a
mesma ideia que o `Restrict` aplica à ordem:

```go
func tipoValido(t string) string {
```

## A fila que se atualiza sozinha

O bloco pede a si mesmo de dois em dois segundos, e o servidor encerra quando não há mais
o que esperar. A resposta ainda é o fragmento, então o último estado fica na tela:

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

## Quando o servidor já tem os eventos

O polling é o piso, não o teto. Se o seu app já sabe quando algo mudou, abra uma conexão
por página com o `ui.Live("/eventos")`, marque o fragmento com o `ui.On("nome", src)` e
mande o nome — nunca o HTML — da rota:

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

O cliente pede de novo a rota do fragmento quando ouve o nome, então a autorização e o
render continuam onde já estavam. Um fragmento com `ui.On` e `ui.Poll` juntos cai no
relógio enquanto a conexão está fora. Veja
[Fragmentos vivos](/pt/referencia/vivo) e [Listagens](/pt/referencia/listagens).
