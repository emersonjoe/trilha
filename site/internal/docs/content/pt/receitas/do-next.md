---
title: Do Next.js para a Trilha
description: Os padrões de React que você escreve sem pensar — useEffect + fetch, useState do modal, toast, useSearchParams — e a linha de Go que substitui cada um.
---

Esta página é para quem já monta telas em Next.js e agora olha para um arquivo Go se
perguntando onde foi parar o estado. É uma tabela de tradução, não um argumento: o app que
está mudando de casa é um painel de artigos — lista com filtro e ordenação, uma fila que se
atualiza sozinha, um modal de confirmação, um PDF — e cada seção mostra o React que você
escreveria e a Trilha que ocupa o lugar dele.

A versão curta da página inteira: **o estado que morava no browser passa a morar na URL ou no
servidor**, e o que sobra no browser é o pouco que o kit já traz pronto.

| No app React | Aqui |
|---|---|
| `useEffect` + `fetch` + `useState(loading)` | uma função de página que lê os dados e devolve o HTML |
| `useSearchParams` e um estado por filtro | [`trilha.ListParams`](/pt/referencia/listagens) lida pelo `c.Bind` |
| `setInterval` + limpeza no `useEffect` | [`ui.Poll`](/pt/referencia/vivo) |
| `toast()` de um provider | `c.Flash` |
| `useState(open)` + portal + tratador de Escape | [`ui.Dialog`](/pt/referencia/ui) |
| `createObjectURL` sobre um blob | `c.Inline` |
| `if (!user) router.replace("/login")` | `Auth.Require()` no `middleware.go` do ramo |
| estado da barra lateral no `localStorage` | [`ui.Shell`](/pt/referencia/shell) |
| `dangerouslySetInnerHTML` | [`ui.Markdown`](/pt/referencia/ui) |
| uma biblioteca de gráfico no cliente | `c.Island` |
| `next.config.js` | `app/setup.go` e `Config` |

Antes da primeira linha de Go, `trilha migrate next ../web` grava a árvore de pastas do `app/`
e um `MIGRATION.md` dizendo, tela por tela, de qual arquivo veio, o que chamava e qual dos
formatos abaixo ela provavelmente é. O esqueleto compila; o porte é o que sobra, e esta página
é a referência dele. Veja
[o comando](/pt/referencia/cli#trilha-migrate).

## A tela que busca

O painel em React: um efeito, três estados e um render que precisa dizer algo sensato em cada
um deles.

```js
export default function Dashboard() {
  const [rows, setRows] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  useEffect(() => {
    fetch("/api/articles")
      .then(r => r.json()).then(setRows)
      .catch(setError).finally(() => setLoading(false))
  }, [])
  if (loading) return <Spinner />
  if (error) return <Error err={error} />
  return <Table rows={rows} />
}
```

A mesma tela aqui é uma função. Ela roda no servidor, então ler o banco é uma chamada de
função e a página chega pronta:

```go
// Dashboard is app/dashboard/page.go. There is no useEffect, no loading flag
// and no error state: the function runs on the server, so it reads the
// database the way any Go function does and returns the page already filled.
// What React needed three renders for happens here before the first byte.
func Dashboard(c *trilha.Ctx) (h.Node, error) {
	var q DashQuery
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	rows, total, err := dashRows(c.Context(), q)
	if err != nil {
		return nil, err
	}
	return ui.DataTable(c, DashColumns, rows, ui.ListState{
		Params: q.ListParams,
		Total:  total,
		ID:     "articles",
		Search: "Search articles",
		Empty:  h.Text("No article matches this filter."),
		RowHref: func(i int) string {
			return "/dashboard/" + rows[i].Slug
		},
	}), nil
}
```

Não há estado de carregando porque não existe o instante em que a página existe sem os dados,
e não há estado de erro porque o erro é um `error` devolvido — o framework transforma nele o
status e a página que a pessoa vê.

## Os filtros que eram estado

Cada filtro do app React era um `useState` mais um `useSearchParams` para guardá-lo no
endereço, mais um efeito para buscar de novo quando qualquer um dos dois mudasse. Aqui o
endereço *é* o estado:

```go
// DashQuery is the whole state the dashboard used to keep in React: the page,
// the ordering, the search box and the type filter. Embedding ListParams is
// what makes them live in the address instead of in memory — the visitor can
// bookmark the screen, reload it, or send it to somebody else.
type DashQuery struct {
	trilha.ListParams
	Status string `form:"status"`
}
```

O `c.Bind` preenche as duas metades a partir da query, aplica os limites de uma listagem
(página no mínimo 1, tamanho de página com teto) e devolve a struct. As colunas dizem por
quais delas dá para ordenar:

```go
// DashColumns replaces the <thead> written by hand and the onClick that
// re-sorted the array in the browser. Sort: true turns the header into a real
// link — the ordering is a GET, so it works with the script blocked and it is
// still there after a reload.
var DashColumns = ui.Columns[Article]{
	{Key: "title", Label: "Title", Sort: true, Cell: func(a Article) h.Node {
		return h.Text(a.Title)
	}},
	{Key: "published", Label: "Published", Sort: true, Cell: func(a Article) h.Node {
		return h.Text(a.Published.Format("2006-01-02"))
	}},
}
```

E a consulta é onde a URL deixa de ser confiável:

```go
// dashRows is the query behind the screen. The ordering comes from the URL,
// so it is checked before it reaches SQL: Restrict drops a column nobody
// declared, and what is left is one of two names this function knows.
func dashRows(ctx context.Context, q DashQuery) ([]Article, int, error) {
	q.Restrict("title", "published")
	order := `published_at DESC`
	if q.Sort == "title" {
		order = `title`
		if !q.Asc() {
			order = `title DESC`
		}
	}
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles WHERE ($1 = '' OR title ILIKE '%' || $1 || '%') AND ($2 = '' OR status = $2) ORDER BY `+order+` LIMIT $3 OFFSET $4`,
		q.Q, q.Status, q.Limit(), q.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	err = DB.QueryRowContext(ctx, `SELECT count(*) FROM articles WHERE ($1 = '' OR status = $1)`, q.Status).Scan(&total)
	return out, total, err
}
```

## O intervalo que vazava

```js
useEffect(() => {
  const id = setInterval(() => refetch(), 6000)
  return () => clearInterval(id)
}, [])
```

O fragmento pede a si mesmo, e quem decide quando parar é o servidor:

```go
// Queue is the block that used to be a useEffect with a setInterval and a
// cleanup nobody remembered to write. ui.Poll asks this same route for the
// fragment every six seconds; when the work is done the handler stops the
// clock from the server side and the browser stops asking.
func Queue(c *trilha.Ctx) (h.Node, error) {
	left, err := pendingArticles(c.Context())
	if err != nil {
		return nil, err
	}
	if left == 0 {
		c.PollStop()
		return h.Div(h.ID("queue"), h.Text("Everything processed.")), nil
	}
	return h.Div(h.ID("queue"), ui.Poll("6s", "#queue"),
		h.Text(strconv.Itoa(left)+" articles in the queue"),
	), nil
}
```

`c.PollStop()` escreve um cabeçalho que o kit lê; `c.PollEvery(d)` muda o intervalo do mesmo
lugar. O browser ainda pausa enquanto a aba está escondida — comportamento que você teria de
escrever e de lembrar de escrever de novo na tela seguinte.

## O toast

`toast.success("Artigo publicado")` precisa de um provider na raiz, de um portal e de um
estado que sobreviva à navegação. O flash é escrito antes do redirect e mostrado pela página
que vem depois:

```go
// Publish is the POST behind the button. The toast() of the React app is a
// flash here: the message is written before the redirect and shown by the
// page that comes next, so a refresh does not repeat the action and the
// message survives the navigation without any state to carry.
func Publish(c *trilha.Ctx) error {
	if _, err := DB.ExecContext(c.Context(), `UPDATE articles SET status = 'published' WHERE slug = $1`, c.Param("slug")); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "Article published")
	return c.Redirect("/dashboard")
}
```

## O modal

O modal em React é um `useState(false)`, um portal, um efeito para o Escape e um foco preso
por biblioteca. O `<dialog>` faz os quatro:

```go
// ConfirmDelete is the modal that used to be a useState(false), a portal and
// an effect closing it on Escape. The dialog is in the HTML from the start,
// closed; the browser opens it, traps the focus and closes it on Escape by
// itself, because <dialog> already does all of that.
func ConfirmDelete(slug string) h.Node {
	return h.Div(
		ui.DialogTrigger("delete", h.Text("Delete")),
		ui.Dialog("delete", "Delete this article?",
			h.P(h.Text("This cannot be undone.")),
			h.Form(h.Method("post"), h.Action("/dashboard/"+slug+"/delete"),
				ui.Button(ui.Destructive(), h.Text("Delete")),
			),
		),
	)
}
```

## O blob

```js
const res = await fetch(`/api/preview/${slug}`)
const url = URL.createObjectURL(await res.blob())
setSrc(url)                       // e revogue, se o componente viver até lá
```

Um endereço é mais simples que um blob, e sobrevive a um recarregamento:

```go
// Preview answers the PDF the <iframe> shows. In the Next.js app this was a
// route handler returning a blob, a createObjectURL and a revoke that leaked
// when the component unmounted early: here the browser asks for a URL and
// gets a document.
//
// Nothing else is needed for the <iframe>: Inline is the answer that says it
// may be framed by a page of this origin. It is the framed document that
// refuses, never the page around it — a page adding frame-src to its own
// policy changes nothing while the file it frames still carries DENY.
func Preview(c *trilha.Ctx) error {
	f, err := os.Open("var/previews/" + c.Param("slug") + ".pdf")
	if err != nil {
		return trilha.ErrNotFound
	}
	defer f.Close()
	return c.Inline("preview.pdf", f, "application/pdf")
}
```

## O redirect que ninguém escreveu duas vezes

`if (!user) router.replace("/login")` no topo de cada página é regra mantida por repetição — e
a décima primeira página esquece. Aqui a regra é a pasta:

```go
// Middleware is the whole of app/dashboard/middleware.go, and it replaces the
// `if (!user) router.replace("/login")` that every page repeated. The check
// runs before the handler, so a route added tomorrow is covered by having been
// put in this folder — not by remembering the first line.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return Panel.Require()(c, next)
}
```

Tudo que está sob `app/dashboard/` está coberto por ter sido posto ali. O `Panel` que ele usa
é montado uma vez, no `app/setup.go`:

```go
// Panel is the session of the migrated app. It is set in app/setup.go; the
// pages never touch it.
var Panel *auth.Auth
```

## A moldura

Barra lateral, barra de cima, menu do usuário, o link ativo, o estado recolhido no
`localStorage`: uma semana de trabalho no app React, e uma struct de opções aqui.

```go
// Frame is app/dashboard/layout.go: the sidebar, the top bar and the user
// menu that the React app assembled from a component library, a state for the
// collapsed sidebar and a localStorage to remember it. Shell renders the
// frame; the current item comes from the path, so no page has to say which
// link is active.
func Frame(c *trilha.Ctx, children ...h.Node) h.Node {
	return ui.Shell(c, ui.ShellOpts{
		Brand:   h.Text("Editorial"),
		Current: c.Request().URL.Path,
		Nav: []ui.NavGroup{{Label: "Content", Items: []ui.NavItem{
			{Href: "/dashboard", Label: "Articles", Icon: "file"},
			{Href: "/dashboard/queue", Label: "Queue", Icon: "clock"},
		}}},
		User: ui.UserMenu{Name: "Editor", Items: []h.Node{
			ui.MenuLink("/logout", h.Text("Sign out")),
		}},
	}, children...)
}
```

## O HTML que não foi você que escreveu

`dangerouslySetInnerHTML` é a saída de emergência que termina em incidente. O `ui.Markdown`
renderiza para uma árvore de nós, então uma tag na origem é texto na saída:

```go
// Notes renders text written by a person or by a model. It is the answer to
// dangerouslySetInnerHTML: the source becomes a tree of nodes, a tag in the
// source comes out as text, and there is no string on the way that anybody
// could hand to h.Raw.
func Notes(src string) h.Node {
	return ui.Markdown(src, ui.MarkdownOpts{HeadingBase: 3})
}
```

## O que continua no browser

Nem tudo deve mudar de lado. Uma biblioteca de gráfico, um mapa, um editor de texto rico —
código que roda no cliente de verdade — vira uma ilha: um módulo, as props em JSON e os
filhos renderizados no servidor como alternativa.

```go
// Sales is the one place the React answer was right: a chart library that
// runs in the browser. The island loads the module, hands it the data as
// JSON and mounts it on this element — and the children are what a visitor
// with the script blocked reads instead.
func Sales(c *trilha.Ctx, points []float64) h.Node {
	return c.Island("/chart.js", map[string]any{"points": points},
		h.Class("chart"),
		ui.Sparkline(points, ui.SparkOpts{}),
	)
}
```

Essa é a história inteira do lado do cliente: sem bundler, sem hidratar o resto da página, e
a alternativa é o que lê quem está com o script bloqueado.

## `next.config.js`

```go
// SetupPreviews is the last piece of the move. next.config.js had rewrites,
// headers and a public/ folder; here public/ is served by the framework and
// the rest is this: a mount for the generated files. The <iframe> needs no
// entry at all — that line used to be here, and it never did anything.
func SetupPreviews(cfg *trilha.Config) {
	cfg.Mounts = map[string]fs.FS{"/previews/": os.DirFS("var/previews")}
}
```

## Achar o nome

O reflexo "como é que se chama esse componente?" tem resposta que não precisa do browser:

```bash
trilha ui describe            # tudo, agrupado
trilha ui describe DataTable  # assinatura, para que serve, um exemplo
```

Ele lê um catálogo montado a partir dos próprios comentários do kit, então não tem como
divergir do código que descreve — e o `--json` o deixa legível para o agente que escreve a
tela com você.

:::note
Dois hábitos vale largar cedo. O primeiro é apelar para uma rota de API: no Next.js uma
página que precisa de dados precisa de um endpoint, e aqui a página *é* o servidor — rota em
`app/api/` é para o que outra pessoa chama. O segundo é o componente de cliente: não existe
bundle de cliente com que se preocupar, então a pergunta nunca é "servidor ou cliente?", e sim
"isso precisa mesmo rodar no browser?".
:::
