---
title: Paginação
description: Paginar por offset enquanto a lista é curta, por cursor quando não é mais, uma linha a mais em vez de um COUNT, e links que um robô consegue seguir.
---

Uma lista que cresce é paginada duas vezes: primeiro com `LIMIT`/`OFFSET`, porque é o óbvio, e
depois com cursor, quando alguém percebe que a página 900 demora quatro segundos. As duas
estão aqui, e a primeira serve para a maioria das listas.

## A janela

```go
// WindowFrom reads ?page= and refuses what it cannot serve. The ceiling is
// not paranoia: OFFSET 900000 makes the database walk every row it skips,
// and a crawler will ask.
func WindowFrom(c *trilha.Ctx) Window {
	n, err := strconv.Atoi(c.Query("page"))
	if err != nil || n < 1 {
		n = 1
	}
	if n > 500 {
		n = 500
	}
	return Window{Page: n, Size: PageSize}
}
```

O teto não é paranoia. `OFFSET 18000` faz o banco ler e descartar dezoito mil linhas, e algum
robô vai pedir a página 900 de tudo que você publica.

```go
// Window is a page of a list: what was asked for, and whether there is more.
type Window struct {
	Page, Size int
	HasNext    bool
}
```

## Uma linha a mais

```go
// ArticlesPage reads one page by offset. It asks for one row more than it
// shows: that extra row is how you know there is a next page without a
// second query counting the whole table.
func ArticlesPage(ctx context.Context, w Window) ([]Article, Window, error) {
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles ORDER BY published_at DESC, id DESC LIMIT $1 OFFSET $2`,
		w.Size+1, (w.Page-1)*w.Size)
	if err != nil {
		return nil, w, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, w, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, w, err
	}
	if len(out) > w.Size {
		out, w.HasNext = out[:w.Size], true
	}
	return out, w, nil
}
```

Pedir `Size+1` e mostrar `Size` responde "existe página seguinte?" sem uma segunda consulta
contando a tabela inteira. `COUNT(*)` numa tabela grande é a consulta que aparece no log de
lentidão dois meses depois — e o total quase nunca é o que o rodapé precisa.

A ordenação tem duas colunas por um motivo: `published_at` sozinho não é único, e um empate
partido na fronteira de uma página mostra a mesma linha duas vezes ou pula uma.

## O cursor

```go
// ArticlesAfter reads the next rows by cursor. The database jumps straight
// to the position with the index, so page one thousand costs the same as
// page one — and a row inserted meanwhile does not shift the whole list.
func ArticlesAfter(ctx context.Context, cursor string, size int) ([]Article, string, error) {
	at, id := time.Now().Add(100*365*24*time.Hour), int64(0)
	if cursor != "" {
		var err error
		if at, id, err = parseCursor(cursor); err != nil {
			return nil, "", err
		}
	}
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles
		 WHERE (published_at, id) < ($1, $2)
		 ORDER BY published_at DESC, id DESC LIMIT $3`,
		at, id, size)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, "", err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) < size {
		return out, "", nil // the end: no cursor to hand back
	}
	return out, Cursor(out[len(out)-1]), nil
}
```

O `WHERE (published_at, id) < ($1, $2)` é o que torna isso barato: com o índice nessas duas
colunas, o banco salta para a posição em vez de contar até ela, então a página mil custa o que
a página um custa. Ele também conserta o bug que a paginação por offset tem por construção —
uma linha inserida enquanto alguém lê desloca todas as páginas seguintes.

```go
// Cursor packs the sort key of the last row. Base64 so it survives a URL,
// not because it is a secret: whoever edits it sees another page of the
// same public list and nothing else.
func Cursor(a Article) string {
	raw := a.Published.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(a.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
```

Base64 porque ele viaja numa URL, não porque esconde alguma coisa: quem editar vê outra página
da mesma lista pública.

## O rodapé

```go
// Pages renders the footer of a list. They are links, not buttons: a page
// number belongs in the address, so it can be shared, reloaded and read by
// whoever indexes the site.
func Pages(path string, w Window) h.Node {
	href := func(n int) string { return path + "?page=" + strconv.Itoa(n) }
	return h.Nav(h.Class("paginas"), h.Aria("label", "Pagination"),
		h.If(w.Page > 1, h.A(h.Rel("prev"), h.Href(href(w.Page-1)), h.Text("Previous"))),
		h.Span(h.Textf("Page %d", w.Page)),
		h.If(w.HasNext, h.A(h.Rel("next"), h.Href(href(w.Page+1)), h.Text("Next"))),
	)
}
```

Links, não botões. O número da página pertence ao endereço, para poder ser compartilhado,
recarregado e indexado; `rel="prev"`/`rel="next"` é o que um robô lê para entender a sequência.

:::dica
`c.Fragment()` transforma isso numa lista que cresce no lugar sem recarregar tudo: o handler
responde só o `<ul>` quando a requisição é um fragmento. Veja
[Interatividade](/pt/aprender/interatividade).
:::

:::nota
Qual usar: offset enquanto a lista é navegada por gente que pula para uma página, cursor para
qualquer coisa que uma máquina percorre — uma API, uma exportação, uma rolagem infinita. Na
API a resposta é sempre o cursor: número de página sobre uma lista que muda devolve
duplicatas.
:::
