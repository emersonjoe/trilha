---
title: Telas prontas
description: Uma tela de app interno de ponta a ponta — moldura, números, tabela que vive na URL, a parte lenta, o arquivo, o formulário em etapas — montada com o kit que você já tem.
---

O [capítulo anterior](/pt/aprender/interface-com-ui) é um catálogo: trinta componentes, um de
cada vez, cada um com a sua demo. Este aqui é uma tela. As mesmas peças, na ordem em que alguém
de fato as escreve, com o motivo de cada uma estar ali — porque saber o que é o `ui.DataTable`
não diz onde ele fica, quem o preenche nem o que o handler acima dele ainda deve.

A tela é o console de pedidos de um app interno: a moldura em volta, quatro números no topo, uma
tabela filtrável no meio, um gráfico que chega atrasado, uma linha que abre um arquivo, um
formulário que leva três telas e uma célula que muda sem ninguém clicar. Cada bloco abaixo é uma
declaração de `examples/cookbook/screens.go`, então tudo isso compila junto com o resto do
repositório.

## A tela, e onde ficam os arquivos dela

O roteamento é a árvore de pastas, então a tela já está descrita por onde os arquivos dela estão:

```text
app/
  layout.go                    ← a moldura: ui.Shell em volta de toda página
  orders/
    page.go                    ← a listagem, e a tabela como fragmento
    insights/page.go           ← o que o ui.Defer pede
    new/
      page.go                  ← etapa um
      items/page.go            ← etapa dois: o GET desenha, o POST salva o rascunho
      confirm/page.go          ← etapa três
    id_/
      page.go                  ← um pedido: metadados e a nota fiscal
      state/page.go            ← a célula viva, em rota própria
```

Nenhuma rota é registrada à mão e nenhum arquivo se registra sozinho: o `trilha gen` lê a
árvore. Veja [Páginas e rotas](/pt/aprender/paginas-e-rotas).

## A moldura

O `ui.Shell` é a barra lateral, o topo e o menu de quem está logado, escritos uma vez, no layout
raiz. Ele é uma composição de componentes que já existiam — `ui.Sidebar`, `ui.Nav`, `ui.Menu` —,
então o que ele não cobre você escreve ao lado com as mesmas peças. O item ativo é aquele cujo
`Href` é o maior prefixo do caminho atual, e é por isso que `/orders/42` acende **Orders**, e não
**Dashboard**.

```go
// OrdersLayout is the frame, written once for every screen of the app.
// ui.Shell is a composition of components that already existed, so anything it
// does not cover is written beside it with the same pieces. ui.LiveScript goes
// in the head because the screen has a fragment that arrives late and a cell
// that refreshes itself; a page with neither never downloads it.
func OrdersLayout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Html(h.Lang("en"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text("Acme Ops")),
			ui.Head(c),
			ui.LiveScript(c),
		),
		h.Body(ui.Body(),
			ui.Shell(c, ui.ShellOpts{
				Brand: h.A(h.Class("ui-brand"), h.Href("/"), h.Text("Acme Ops")),
				Nav: []ui.NavGroup{
					{Label: "Work", Items: []ui.NavItem{
						{Href: "/", Label: "Dashboard", Icon: "house"},
						{Href: "/orders", Label: "Orders", Icon: "search"},
					}},
					// Hide is a courtesy to whoever reads the menu, never a
					// permission: what keeps somebody out of /settings is the
					// middleware.go at the root of that folder.
					{Label: "Admin", Hide: !ordersAdmin(c), Items: []ui.NavItem{
						{Href: "/settings", Label: "Settings", Icon: "settings"},
					}},
				},
				User:   ui.UserMenu{Name: "Ana Reis", Detail: "ana@acme.example", Items: []h.Node{ui.MenuLink("/profile", h.Text("Profile"))}},
				Header: []h.Node{ui.ThemeToggle()},
			}, children),
			ui.Flashes(c),
		),
	), nil
}
```

Duas decisões que merecem nome. O `ui.LiveScript(c)` está no `head` porque esta tela tem um
fragmento que chega depois e uma célula que se atualiza sozinha; o `ui.Head` não carrega esse
script, e uma página sem nenhum dos dois nunca o baixa. E o `Hide` do grupo Admin é uma
**cortesia com quem lê o menu, não uma permissão** — a mesma pergunta é feita de novo, para
valer, pelo `middleware.go` na raiz de `/settings`:

```go
// ordersAdmin is the question the menu asks, and the same one the middleware
// at the root of /settings asks again — the menu hides, the middleware
// refuses, and only one of the two is a permission.
func ordersAdmin(c *trilha.Ctx) bool {
	role, _ := c.Get("role").(string)
	return role == "admin"
}
```

Veja [Shell](/pt/referencia/shell) para `Current`, a lateral que recolhe e o `IconNode`, e
[a demo](/pt/aprender/interface-com-ui#a-moldura-de-um-app-interno) com a moldura desenhada.

## O que é um pedido, e o que ele pode ser

A linha é o tipo do app, não do kit. `ui.Columns[T]` é genérico, então quem confere a tabela é o
compilador, e não um `map[string]any` que renderiza em branco quando um campo muda de nome:

```go
// Order is the row of the screen. The type is the app's, not the kit's:
// ui.Columns is generic, so a field that changes name stops compiling instead
// of rendering blank.
type Order struct {
	ID       string
	Customer string
	Total    float64
	State    string
	Updated  time.Time
	Invoice  string // where the PDF is; empty until the order ships
}
```

Os estados são uma lista só, declarada uma vez:

```go
// OrderStates is the single declaration of what an order can be: the value the
// database holds, the word a person reads and the tone the badge wears. The
// filter's select, the form's validation and ui.Status all read this list, so
// a state added here shows up in the three of them at once.
var OrderStates = trilha.Enum{
	{Value: "draft", Label: "Draft"},
	{Value: "picking", Label: "Picking", Tone: "info"},
	{Value: "shipped", Label: "Shipped", Tone: "success"},
	{Value: "returned", Label: "Returned", Tone: "danger"},
}
```

Essa lista é lida por quatro coisas diferentes — o `<select>` do filtro, a validação do
formulário (`validate:"enum=ops.OrderState"`, depois do `trilha.RegisterEnum`), o emblema da
tabela e o emblema da tela de detalhe. Acrescentar um estado é uma linha aqui, e os quatro
seguem. Veja [Validação](/pt/referencia/validacao#enum-uma-lista-de-dominio-declarada-uma-vez) e
[a demo](/pt/aprender/interface-com-ui#um-valor-do-enum-como-emblema).

## Um handler, três respostas

A tela inteira é uma função. Ela responde a página e responde só a tabela quando o kit a troca —
o id do fragmento é o id do elemento que está sendo substituído, e esse é o protocolo inteiro:

```go
// OrdersPage is the screen and each of its pieces. One handler answers three
// requests — the whole page, the table when the kit swaps it, and nothing else
// — because the id of the fragment is the id of the element being replaced,
// and that is the entire protocol.
func OrdersPage(c *trilha.Ctx) (h.Node, error) {
	var q OrdersQuery
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	// Sort arrived from the address, typed by whoever wrote it. Restrict is
	// what turns it into a column name, before the repository sees it.
	q.Restrict("customer", "total", "updated")
	rows, total, err := orders.Search(q)
	if err != nil {
		return nil, err
	}
	table := ordersTable(c, q, rows, total)
	if c.Fragment() == "orders" {
		return table, nil
	}
	sum, err := orders.Summary()
	if err != nil {
		return nil, err
	}
	c.SetTitle("Orders")
	return h.Div(
		ui.PageHeader("Orders", ui.ButtonLink("/orders/new", ui.Icon("plus"), h.Text("New order"))),
		ui.Grid(orderStats(sum)...),
		ui.Defer(c, "insights", "/orders/insights", ui.DeferOpts{Height: "12rem"}),
		table,
	), nil
}
```

O `c.Bind` preenche a struct a partir da query e aplica os limites do `ListParams`; o `Restrict`
é a linha que importa para a segurança, porque o `Sort` veio do endereço, digitado por quem
quis, e não é nome de coluna até a tela dizer quais nomes existem. Veja
[Interatividade](/pt/aprender/interatividade) para o fragmento e
[Listagens](/pt/referencia/listagens) para o resto.

## Quatro números no topo

Três contagens e um número que tem forma. O sparkline tem o tamanho de uma linha de texto, em SVG
escrito pelo servidor: chega junto com a página, imprime e não custa download nenhum.

```go
// orderStats is the row of numbers on top. Three of them are counts and the
// fourth has a shape, which is what a sparkline is for: a drawing the size of
// a line of text, in SVG written by the server, next to the number it is
// about.
func orderStats(s OrdersSummary) []h.Node {
	return []h.Node{
		ui.Card(ui.Stat("Open", strconv.Itoa(s.Open))),
		ui.Card(ui.Stat("Shipped today", strconv.Itoa(s.ShippedToday))),
		ui.Card(ui.Stat("Revenue", dollars(s.Revenue), ui.StatHint("this month"))),
		ui.Card(ui.Stat("Late", strconv.Itoa(s.Late),
			ui.SparklineTitle(s.LateByDay, ui.SparkOpts{}, ui.ChartTitle("Late orders, last 14 days")))),
	}
}

// dollars is money as text, because ui.Stat takes a string: the framework has
// no currency, so where the symbol goes is the app's decision. In a cell,
// where a node fits, ui.Number does the digits by the locale of the Ctx.
func dollars(v float64) string { return "$" + strconv.FormatFloat(v, 'f', 2, 64) }
```

O `ui.ChartTitle` é o que transforma o desenho em uma imagem com nome em vez de enfeite, e o
`dollars` é um lembrete de que o framework não tem moeda: o `ui.Stat` recebe texto, então onde o
símbolo entra é decisão do app. Onde cabe um nó — uma célula de tabela —, o `ui.Number` e o
`ui.Date` leem o locale do `Ctx`. Veja [Gráficos](/pt/referencia/graficos),
[Formatação](/pt/referencia/ui#formatacao) e
[a demo](/pt/aprender/interface-com-ui#quatro-numeros-e-os-desenhos-ao-lado).

## A tabela vive na URL

O estado da listagem é uma struct, e tudo nele está no endereço, então a tela pode ser
compartilhada, recarregada, salva nos favoritos e desandada com o botão do próprio navegador:

```go
// OrdersQuery is the state of the screen, and all of it is in the address:
// ListParams brings page, order and search, and the fields beside it are the
// filters this screen added. c.Bind fills the whole thing, applies the limits
// of ListParams and keeps the rest of the query so the links preserve it.
type OrdersQuery struct {
	trilha.ListParams
	State string `form:"state" validate:"omitempty,enum=ops.OrderState"`
}
```

As colunas são o tipo do app de novo. Duas das células são formatadas, e é por isso que elas
recebem o `Ctx`:

```go
// ordersColumns names the table. Key is what the URL orders by and what
// Restrict accepts; Cell renders one row of the app's own type. It takes the
// Ctx because two of the cells are formatted — the number and the date read
// the locale off it, and the same screen serves both languages.
func ordersColumns(c *trilha.Ctx) ui.Columns[Order] {
	return ui.Columns[Order]{
		{Key: "customer", Label: "Customer", Sort: true, Cell: func(o Order) h.Node { return h.Text(o.Customer) }},
		{Key: "total", Label: "Total", Sort: true, Num: true, Cell: func(o Order) h.Node {
			return ui.Number(c, o.Total, ui.Decimals(2))
		}},
		{Key: "state", Label: "State", Cell: orderStateCell},
		{Key: "updated", Label: "Updated", Sort: true, Cell: func(o Order) h.Node {
			return ui.Date(c, o.Updated, ui.Relative())
		}},
	}
}
```

E a tabela é uma chamada só, com tudo o que a URL disse sobre ela:

```go
// ordersTable is the listing: the filter form on top, the sortable headers,
// the pagination at the foot and the empty state, all of it built from what
// Bind read. ID is what makes ordering, filtering and paging swap the table
// instead of reloading the screen.
func ordersTable(c *trilha.Ctx, q OrdersQuery, rows []Order, total int) h.Node {
	return ui.DataTable(c, ordersColumns(c), rows, ui.ListState{
		Params:  q.ListParams,
		Total:   total,
		ID:      "orders",
		Search:  "Search by customer",
		Filters: ordersFilters(q),
		Empty:   ordersEmpty(q),
		Cards:   true,
		RowHref: func(i int) string { return "/orders/" + rows[i].ID },
	})
}
```

```go
// ordersFilters is the rest of the filter form — the search box is the kit's.
// The select's options come from the enum, so the filter cannot offer a state
// the validation would refuse.
func ordersFilters(q OrdersQuery) h.Node {
	return ui.Field("state", "State", ui.Select(h.ID("state"), h.Name("state"),
		OrderStates.Options(q.State, "Any state")))
}
```

O `ID` é o que faz ordenar, filtrar e paginar trocarem a tabela em vez de recarregarem a tela —
com o JavaScript desligado, os mesmos links navegam e o mesmo formulário envia. O `RowHref` faz
a linha abrir o pedido. As opções do filtro vêm do enum, então a tela não consegue oferecer um
estado que a validação recusaria. Veja [Listagens](/pt/referencia/listagens) e
[a demo](/pt/aprender/interface-com-ui#tabelas-que-vivem-na-url).

## Filtrada até não sobrar nada

Uma lista sem nada dentro e uma lista que um filtro esvaziou são duas telas diferentes. A
primeira diz "crie o primeiro pedido"; a segunda tem de dizer **o que desfazer**, ou a pessoa
está olhando para um app que parece ter perdido os dados dela:

```go
// ordersEmpty is the screen a filter that matched nothing shows. It is not the
// same screen as an app with no orders at all: this one says what to undo, and
// the way out is the same listing with the filter dropped and the rest of the
// address kept.
func ordersEmpty(q OrdersQuery) h.Node {
	return ui.Empty(ui.EmptyOpts{
		Icon:   "funnel",
		Title:  "No order matches this filter",
		Hint:   "Try another state, or clear the search.",
		Action: ui.ButtonLink(q.Href("q", "", "state", "", "page", ""), ui.Outline(), h.Text("Clear the filter")),
	})
}
```

O `q.Href` preserva o resto do endereço, então a saída larga o filtro e a busca, e nada mais. O
`ui.DataTable` faz essa distinção sozinho, em inglês; o `ListState.Empty` é como a aplicação diz
isso com as palavras dela. Veja
[a demo](/pt/aprender/interface-com-ui#vazia-e-filtrada-ate-vazia).

## A parte lenta, depois da página

O gráfico agrupa um mês de pedidos por canal e leva dois segundos. Nada mais na tela devia
esperar por ele, e ninguém devia ficar olhando para um spinner no lugar onde a página inteira
estava:

```go
// OrdersInsights is the route the placeholder asks for once the page has
// loaded. It is an ordinary page: with no fragment header it answers the whole
// thing, which is where the placeholder's <noscript> link goes — without
// JavaScript the slow part is one click away instead of missing.
func OrdersInsights(c *trilha.Ctx) (h.Node, error) {
	data, err := orders.Insights(c.Context())
	if err != nil {
		return nil, err
	}
	block := h.Div(h.ID("insights"), ui.Bars(data, ui.ChartTitle("Orders by channel")))
	if c.Fragment() == "insights" {
		return block, nil
	}
	return h.Div(ui.PageHeader("Insights"), block), nil
}
```

O `ui.Defer` desenha um placeholder de altura conhecida — a página não pode pular quando o
conteúdo chega — e pede o fragmento assim que a página carrega. A rota é uma página comum: sem o
cabeçalho de fragmento ela responde a coisa inteira, que é para onde vai o link do `<noscript>`
do placeholder; sem JavaScript, a parte lenta fica a um clique de distância em vez de sumir.

Um `Defer` por parte da tela, nunca um por linha de lista: uma página que adia vinte fragmentos
fez vinte requisições para se desenhar, que é justamente a SPA que ela estava evitando. Veja
[Fragmentos vivos](/pt/referencia/vivo) e
[a demo](/pt/aprender/interface-com-ui#a-parte-lenta-um-instante-depois).

## A célula que anda sozinha

Um pedido é separado e despachado por gente que não está olhando para esta tela. A célula de
estado fica de olho:

```go
// orderStateCell is the one cell that moves without anybody clicking. ui.On
// waits for an event named after the order and asks this same fragment's route
// again — the event carries the name, never the HTML, so the authorization and
// the rendering stay where they already are.
func orderStateCell(o Order) h.Node {
	return h.Div(h.ID("order-"+o.ID+"-state"), ui.On("order:"+o.ID, "/orders/"+o.ID+"/state"),
		ui.Status(OrderStates, o.State))
}

// OrderStatePage is what the event makes the browser ask for. A value the enum
// no longer knows renders muted instead of taking the screen down.
func OrderStatePage(c *trilha.Ctx) (h.Node, error) {
	o, err := orders.Find(c.Param("id"))
	if err != nil {
		return nil, err
	}
	return orderStateCell(o), nil
}
```

O `ui.On` espera um evento com o nome do pedido e pede de novo a rota da própria célula. **O
evento carrega o nome, nunca o HTML**: a autorização e a renderização continuam onde já estavam,
e o stream nunca vira um canal de dados. O `ui.Poll("30s", src)` é a mesma ideia no relógio, para
uma tela que não tem um barramento atrás.

A célula é a mesma função na tabela e na tela de detalhe, porque um fragmento não sabe em que
tela está. Veja [Fragmentos vivos](/pt/referencia/vivo) e
[a demo](/pt/aprender/interface-com-ui#uma-celula-que-se-atualiza-sozinha).

## A linha, aberta

```go
// OrderPage is what a row opens: the metadata on one side, the invoice on the
// other, and the same live cell the table had — the fragment does not care
// which screen it is on.
func OrderPage(c *trilha.Ctx) (h.Node, error) {
	o, err := orders.Find(c.Param("id"))
	if err != nil {
		return nil, err
	}
	c.SetTitle("Order " + o.ID)
	return h.Div(
		ui.PageHeader("Order "+o.ID, ui.ButtonLink("/orders", ui.Outline(), ui.Icon("arrow-left"), h.Text("Back"))),
		ui.Grid(
			ui.Card(ui.Stack(
				ui.Stat("Customer", o.Customer),
				ui.Stat("Total", dollars(o.Total)),
				orderStateCell(o),
			)),
			orderInvoice(c, o),
		),
	), nil
}
```

A nota fiscal é um arquivo, e arquivo tem três casos: um que o navegador desenha, um que ele
renderiza embutido e um que ele não mostra de jeito nenhum. O `ui.Preview` escolhe; o terceiro
caso vira um cartão com botão de baixar em vez de um quadro que renderiza em branco. O quarto
caso é o arquivo que ainda não existe, e aí é o `ui.Empty` de novo — o mesmo componente, outra
frase:

```go
// orderInvoice is the file beside its metadata. ui.Preview decides how to show
// it from the type — an image as an <img>, a PDF in a frame, anything a
// browser cannot render as a card with a download button instead of a frame
// that renders blank.
func orderInvoice(c *trilha.Ctx, o Order) h.Node {
	if o.Invoice == "" {
		return ui.Empty(ui.EmptyOpts{Icon: "download", Title: "No invoice yet",
			Hint: "It is issued when the order ships."})
	}
	return ui.Preview(c, o.Invoice, ui.PreviewOpts{Title: "Invoice " + o.ID, Height: "24rem"})
}
```

Veja [a demo](/pt/aprender/interface-com-ui#um-arquivo-ao-lado-dos-seus-metadados) e
[Uploads](/pt/receitas/uploads) para saber de onde vem o `o.Invoice`.

## O formulário que leva três telas

O `ui.Steps` desenha onde a pessoa está: o que ficou para trás linka de volta, o que está à
frente é texto puro, e a etapa atual leva `aria-current="step"`.

```go
// orderSteps is the new-order form, which is three screens. A step ahead of
// the current one never links, however hard somebody stares at it: a wizard
// whose third step is one click away is a wizard whose steps did not have to
// happen in order.
var orderSteps = []ui.Step{
	{Label: "Customer", Href: "/orders/new"},
	{Label: "Items", Href: "/orders/new/items"},
	{Label: "Confirm"},
}
```

O que viaja entre as telas é um rascunho, uma struct por etapa, para que cada tela valide os
próprios campos e nenhuma mensagem aponte para um campo duas telas atrás:

```go
// OrderDraft is what travels between the three screens, one struct per step so
// that each screen validates its own fields and no message ever points at a
// field two screens back.
type OrderDraft struct {
	Who   OrderWhoStep   `json:"who"`
	Items OrderItemsStep `json:"items"`
}

// OrderWhoStep is the first screen.
type OrderWhoStep struct {
	Customer string `form:"customer" validate:"required,max=80"`
	Email    string `form:"email"    validate:"required,email"`
}

// OrderItemsStep is the second.
type OrderItemsStep struct {
	SKU string `form:"sku" validate:"required"`
	Qty int    `form:"qty" validate:"required,min=1"`
}
```

```go
// OrderItemsPage is every screen after the first. No draft is not a failure:
// it is somebody whose draft expired or who typed the address of step two, and
// the answer is step one — not an empty form that would lose what they fill in
// here.
func OrderItemsPage(c *trilha.Ctx) (h.Node, error) {
	var d OrderDraft
	if err := c.Draft("new-order").Load(&d); err != nil {
		return nil, c.Redirect("/orders/new")
	}
	return h.Div(
		ui.PageHeader("New order"),
		ui.Steps(orderSteps, 2),
		orderItemsForm(c, d.Items),
	), nil
}
```

```go
// OrderItemsPOST is the end of a step: load what is there, bind only this
// step, save, move on. A 422 here costs nothing that was typed on an earlier
// screen, because the earlier screens are in the draft and not in this form.
func OrderItemsPOST(c *trilha.Ctx) error {
	var d OrderDraft
	_ = c.Draft("new-order").Load(&d)
	if err := c.Bind(&d.Items); err != nil {
		return err // FieldErrors: 422 with the messages next to the fields
	}
	if err := c.Draft("new-order").Save(d, 30*time.Minute); err != nil {
		return err
	}
	return c.Redirect("/orders/new/confirm")
}
```

O `c.Draft` é um cookie assinado ou uma store, conforme o `Config.Drafts`; de um jeito ou de
outro, um refresh, um 422 ou um telefonema no meio não custam nada do que foi digitado. Não ter
rascunho não é falha — é um rascunho que expirou ou um endereço colado, e a resposta é a etapa
um. O formulário posta para a própria rota, com `trilha.CSRFInput(c)` dentro. Veja
[Um formulário em etapas](/pt/receitas/formulario-em-passos) e
[a demo](/pt/aprender/interface-com-ui#um-formulario-em-varias-telas).

## O que a tela custou

Nenhum roteador no cliente, nenhum passo de build, nenhum estado duplicado entre servidor e
navegador e nenhum JavaScript sem o qual a tela pare de funcionar: ordenar, filtrar, paginar, o
formulário em etapas e a tela de detalhe funcionam com o `ui.js` desligado, e o que o script
acrescenta é que nada disso recarrega. O conjunto é uma pasta de handlers e umas duzentas linhas.

O que não coube aqui, e entra do mesmo jeito quando a tela precisar:
[`ui.Tree`](/pt/aprender/interface-com-ui#uma-hierarquia-que-abre-no-a-no) para um filtro que é
uma hierarquia, [`ui.Combobox`](/pt/aprender/interface-com-ui#um-campo-que-busca-enquanto-voce-digita)
para um campo com milhares de opções,
[`ui.SearchBox`](/pt/aprender/interface-com-ui#uma-caixa-varios-tipos-de-coisa) para a busca da
barra de cima e
[`ui.EmptyError`](/pt/aprender/interface-com-ui#nada-para-mostrar-e-o-que-nao-deu-para-carregar)
para a parte da tela que não deu para carregar — com o erro em si só em desenvolvimento.

## Desafio

Quem opera este console quer ver de relance quais pedidos estão atrasados: não despachados e sem
mexer há mais de dois dias. Acrescente a coluna sem tocar na consulta, na URL nem na tela vazia.

:::solution
```go
// ordersLateColumn is the answer to the chapter's challenge: one more column,
// and nothing else. The row already knows whether it is late, and the cell is
// the same badge the state uses — a second vocabulary of colours on one screen
// is how two tables end up with two different reds.
var ordersLateColumn = ui.Column[Order]{Key: "late", Label: "Late", Cell: func(o Order) h.Node {
	if o.State == "shipped" || time.Since(o.Updated) < 48*time.Hour {
		return ui.Muted(h.Text("—"))
	}
	return ui.Badge(ui.Destructive(), h.Text("late"))
}}
```

Uma entrada a mais nas colunas de `ordersColumns`, e nada mais se mexe: a linha já carrega o que
a resposta precisa, então não aparece filtro, nem parâmetro, nem uma segunda consulta. O emblema
é o mesmo que a célula de estado usa, porque um segundo vocabulário de cores na mesma tela é
como duas tabelas acabam com dois vermelhos diferentes.

Repare no que a coluna **não** tem: `Sort: true`. Ordenar por ela colocaria `late` na URL, e o
`Restrict` derrubaria — o banco não tem essa coluna, e a tela estaria mentindo sobre o que
consegue ordenar.
:::
