package demos

import (
	"errors"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	kit "github.com/emersonjoe/trilha/ui"
)

// Demos of the ui kit. The result pane is wrapped in .ui-body.kit so the kit's
// styles apply inside the docs site without touching the rest of the page.
func wrap(n h.Node) h.Node { return h.Div(h.Class("ui-body ui-stack kit"), n) }

// pedidoStatusPT and orderStatusEN back the ui-status demo: a trilha.Enum
// declared once, the same shape an app would use for the status of anything.
var pedidoStatusPT = trilha.Enum{
	{Value: "enviado", Label: "Enviado", Tone: "info"},
	{Value: "entregue", Label: "Entregue", Tone: "success"},
	{Value: "cancelado", Label: "Cancelado", Tone: "danger"},
}

var orderStatusEN = trilha.Enum{
	{Value: "shipped", Label: "Shipped", Tone: "info"},
	{Value: "delivered", Label: "Delivered", Tone: "success"},
	{Value: "cancelled", Label: "Cancelled", Tone: "danger"},
}

// errKitDemo stands in for a real failure in the ui-vazio demo; EmptyError
// only shows it in trilha.Dev, and the demo passes a nil Ctx, so the message
// never reaches the rendered output either way.
var errKitDemo = errors.New("connection refused")

// kitDemoLogoDataURL is a 1x1 transparent PNG, so the ui-preview demo's image
// branch has something real to decode without shipping a binary asset.
const kitDemoLogoDataURL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func init() {
	// ---- pt ----
	add("pt", Demo{
		Name:  "ui-botoes",
		Title: "Botões: variantes e tamanhos são atributos",
		Source: `ui.Row(
	ui.Button(h.Text("Salvar")),
	ui.Button(ui.Secondary(), h.Text("Rascunho")),
	ui.Button(ui.Outline(), h.Text("Cancelar")),
	ui.Button(ui.Ghost(), ui.Icon("settings")),
	ui.Button(ui.Destructive(), ui.Sm(), ui.Icon("trash"), h.Text("Apagar")),
	ui.Badge(h.Text("novo")),
	ui.Badge(ui.Outline(), h.Text("beta")),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Button(h.Text("Salvar")),
				kit.Button(kit.Secondary(), h.Text("Rascunho")),
				kit.Button(kit.Outline(), h.Text("Cancelar")),
				kit.Button(kit.Ghost(), kit.Icon("settings")),
				kit.Button(kit.Destructive(), kit.Sm(), kit.Icon("trash"), h.Text("Apagar")),
				kit.Badge(h.Text("novo")),
				kit.Badge(kit.Outline(), h.Text("beta")),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-formulario",
		Title: "Campos condicionais sem escrever JavaScript",
		Source: `h.Form(h.Class("ui-stack"),
	ui.Field("tipo", "Tipo de pessoa",
		ui.Select(h.ID("tipo"), h.Name("tipo"),
			h.Option(h.Value("pf"), h.Text("Física")),
			h.Option(h.Value("pj"), h.Text("Jurídica")))),
	ui.Field("cpf", "CPF", ui.Input(h.ID("cpf"), h.Name("cpf")),
		ui.With(ui.ShowWhen("tipo", "pf"))),
	ui.Field("cnpj", "CNPJ", ui.Input(h.ID("cnpj"), h.Name("cnpj")),
		ui.Help("14 dígitos"), ui.With(ui.ShowWhen("tipo", "pj"))),
	ui.CheckRow(ui.Switch(h.ID("nf"), h.Name("nf")), "Emitir nota fiscal", "nf"),
	ui.Field("email", "E-mail para a nota", ui.Input(h.ID("email"), h.Type("email")),
		ui.With(ui.ShowWhen("nf"))),
)`,
		Node: func() h.Node {
			return wrap(h.Form(h.Class("ui-stack"),
				kit.Field("tipo", "Tipo de pessoa",
					kit.Select(h.ID("tipo"), h.Name("tipo"),
						h.Option(h.Value("pf"), h.Text("Física")),
						h.Option(h.Value("pj"), h.Text("Jurídica")))),
				kit.Field("cpf", "CPF", kit.Input(h.ID("cpf"), h.Name("cpf")),
					kit.With(kit.ShowWhen("tipo", "pf"))),
				kit.Field("cnpj", "CNPJ", kit.Input(h.ID("cnpj"), h.Name("cnpj")),
					kit.Help("14 dígitos"), kit.With(kit.ShowWhen("tipo", "pj"))),
				kit.CheckRow(kit.Switch(h.ID("nf"), h.Name("nf")), "Emitir nota fiscal", "nf"),
				kit.Field("email", "E-mail para a nota", kit.Input(h.ID("email"), h.Type("email")),
					kit.With(kit.ShowWhen("nf"))),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-confirmar",
		Title: "Perguntar antes de destruir, no próprio formulário",
		Source: `h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
	ui.Confirm("Apagar este post?", "Não dá para desfazer."),
	h.Data("ui-confirm-cancel", "Cancelar"),
	ui.Submit(ui.Destructive(), h.Text("Apagar")),
)`,
		Node: func() h.Node {
			return wrap(h.Form(h.Method("get"), h.Action("#"),
				h.Input(h.Type("hidden"), h.Name("_csrf"), h.Value("token-gerado-por-requisicao")),
				kit.Confirm("Apagar este post?", "Não dá para desfazer."),
				h.Data("ui-confirm-cancel", "Cancelar"),
				kit.Submit(kit.Destructive(), h.Text("Apagar")),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-card",
		Title: "Card, abas e progresso",
		Source: `ui.Card(
	ui.CardHeader(ui.CardTitle("Meta do mês"), ui.CardDescription("7 de 10 posts")),
	ui.CardContent(
		ui.Progress(7, 10),
		ui.Tabs("meta",
			ui.Tab{Label: "Resumo", Content: h.P(h.Text("Faltam 3."))},
			ui.Tab{Label: "Detalhes", Content: h.P(h.Text("Abas com teclado e ARIA."))},
		),
	),
	ui.CardFooter(ui.Button(ui.Sm(), h.Text("Publicar"))),
)`,
		Node: func() h.Node {
			return wrap(kit.Card(
				kit.CardHeader(kit.CardTitle("Meta do mês"), kit.CardDescription("7 de 10 posts")),
				kit.CardContent(
					kit.Progress(7, 10),
					kit.Tabs("meta",
						kit.Tab{Label: "Resumo", Content: h.P(h.Text("Faltam 3."))},
						kit.Tab{Label: "Detalhes", Content: h.P(h.Text("Abas com teclado e ARIA."))},
					),
				),
				kit.CardFooter(kit.Button(kit.Sm(), h.Text("Publicar"))),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-dialogo",
		Title: "Diálogo nativo e aviso que some sozinho",
		Source: `ui.Row(
	ui.DialogTrigger("confirma", ui.Outline(), h.Text("Abrir diálogo")),
	ui.Dialog("confirma", "Publicar agora?",
		ui.DialogDescription("O post fica visível para todos."),
		ui.DialogFooter(ui.DialogClose(ui.Ghost(), h.Text("Depois")), ui.DialogClose(h.Text("Publicar")))),
	ui.Button(ui.Secondary(), h.Data("ui-toast", "Salvo!"), h.Text("Mostrar aviso")),
)
// data-ui-toast mostra um aviso ao clicar; do servidor, após um POST,
// renderize ui.Toast("success", "Salvo!", 4000) dentro de ui.Toaster().`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.DialogTrigger("confirma", kit.Outline(), h.Text("Abrir diálogo")),
				kit.Dialog("confirma", "Publicar agora?",
					kit.DialogDescription("O post fica visível para todos."),
					kit.DialogFooter(kit.DialogClose(kit.Ghost(), h.Text("Depois")), kit.DialogClose(h.Text("Publicar")))),
				kit.Button(kit.Secondary(), h.Data("ui-toast", "Salvo!"), h.Text("Mostrar aviso")),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-tabela",
		Title: "Tabela com linhas aninhadas (drill-down)",
		Source: `ui.Table(
	h.Thead(h.Tr(h.Th(h.Text("Conta")), h.Th(ui.Num(), h.Text("Orçado")), h.Th(ui.Num(), h.Text("Realizado")))),
	h.Tbody(
		h.Tr(ui.Depth(0), h.Td(h.Strong(h.Text("Despesas"))), h.Td(ui.Num(), h.Text("12.000")), h.Td(ui.Num(), h.Text("11.240"))),
		h.Tr(ui.Depth(1), h.Td(h.Text("Pessoal")), h.Td(ui.Num(), h.Text("8.000")), h.Td(ui.Num(), h.Text("8.000"))),
		h.Tr(ui.Depth(1), h.Td(h.Text("Marketing")), h.Td(ui.Num(), h.Text("4.000")), h.Td(ui.Num(), ui.Badge(ui.Destructive(), h.Text("3.240")))),
	),
)`,
		Node: func() h.Node {
			return wrap(kit.Table(
				h.Thead(h.Tr(h.Th(h.Text("Conta")), h.Th(kit.Num(), h.Text("Orçado")), h.Th(kit.Num(), h.Text("Realizado")))),
				h.Tbody(
					h.Tr(kit.Depth(0), h.Td(h.Strong(h.Text("Despesas"))), h.Td(kit.Num(), h.Text("12.000")), h.Td(kit.Num(), h.Text("11.240"))),
					h.Tr(kit.Depth(1), h.Td(h.Text("Pessoal")), h.Td(kit.Num(), h.Text("8.000")), h.Td(kit.Num(), h.Text("8.000"))),
					h.Tr(kit.Depth(1), h.Td(h.Text("Marketing")), h.Td(kit.Num(), h.Text("4.000")), h.Td(kit.Num(), kit.Badge(kit.Destructive(), h.Text("3.240")))),
				),
			))
		},
	})

	add("pt", Demo{
		Name:  "ui-paginacao",
		Title: "Paginação em links de verdade, dica sem JavaScript",
		Source: `ui.Row(
	ui.Tooltip("Só quem escreveu vê os rascunhos",
		ui.Button(ui.Outline(), h.Text("Rascunhos"))),
	ui.Pagination(ui.Pages{
		Page: 4, Total: 12,
		Href: func(n int) string { return "?pagina=" + strconv.Itoa(n) },
		Prev: "Anterior", Next: "Próxima", Label: "Paginação",
	}),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Tooltip("Só quem escreveu vê os rascunhos",
					kit.Button(kit.Outline(), h.Text("Rascunhos"))),
				kit.Pagination(kit.Pages{
					Page: 4, Total: 12,
					Href: func(n int) string { return "?pagina=" + strconv.Itoa(n) },
					Prev: "Anterior", Next: "Próxima", Label: "Paginação",
				}),
			))
		},
	})

	add("pt", Demo{
		Name:  "ui-shell",
		Title: "A moldura de um app interno, com o item ativo",
		Source: `ui.Shell(c, ui.ShellOpts{
	Brand:   h.Text("Acervo"),
	Current: "/documentos",
	Nav: []ui.NavGroup{
		{Label: "Trabalho", Items: []ui.NavItem{
			{Href: "/painel", Label: "Painel", Icon: "house"},
			{Href: "/documentos", Label: "Documentos", Badge: "12"},
		}},
	},
	User: ui.UserMenu{Name: "Ana Paula", Detail: "ana@acervo.com", Items: []h.Node{
		ui.MenuLink("/perfil", h.Text("Perfil")),
	}},
},
	ui.PageHeader("Documentos",
		ui.ButtonLink("/documentos/novo", ui.Icon("plus"), h.Text("Novo documento"))),
)`,
		Node: func() h.Node {
			return wrap(kit.Shell(nil, kit.ShellOpts{
				Brand:   h.Text("Acervo"),
				Current: "documentos",
				Nav: []kit.NavGroup{
					{Label: "Trabalho", Items: []kit.NavItem{
						{Href: "#", Label: "Painel", Icon: "house"},
						{Href: "documentos", Label: "Documentos", Badge: "12"},
					}},
				},
				User: kit.UserMenu{Name: "Ana Paula", Detail: "ana@acervo.com", Items: []h.Node{
					kit.MenuLink("#", h.Text("Perfil")),
				}},
			},
				kit.PageHeader("Documentos",
					kit.ButtonLink("#", kit.Icon("plus"), h.Text("Novo documento"))),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-breadcrumb",
		Title: "Trilha de migalhas e o avatar de quem está logado",
		Source: `ui.Row(
	ui.Breadcrumb(
		ui.Crumb{Label: "Documentos", Href: "/documentos"},
		ui.Crumb{Label: "Contrato 41"},
	),
	ui.Spacer(),
	ui.Avatar("AP", ""),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Breadcrumb(
					kit.Crumb{Label: "Documentos", Href: "#"},
					kit.Crumb{Label: "Contrato 41"},
				),
				kit.Spacer(),
				kit.Avatar("AP", ""),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-menu",
		Title: "Menu num popover nativo",
		Source: `ui.Row(
	ui.MenuTrigger("acoes", ui.Outline(), ui.Icon("settings"), h.Text("Ações")),
	ui.Menu("acoes",
		ui.MenuItem(h.Text("Duplicar")),
		ui.MenuItem(h.Text("Arquivar")),
		ui.MenuLink("/documentos/41/editar", h.Text("Editar")),
	),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.MenuTrigger("kit-acoes", kit.Outline(), kit.Icon("settings"), h.Text("Ações")),
				kit.Menu("kit-acoes",
					kit.MenuItem(h.Text("Duplicar")),
					kit.MenuItem(h.Text("Arquivar")),
					kit.MenuLink("#", h.Text("Editar")),
				),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-colapsavel",
		Title: "Um FAQ sem uma linha de JavaScript",
		Source: `ui.Stack(
	ui.Collapsible("Como funciona o repasse?",
		h.P(h.Text("O pedido passa para outro atendente sem perder o histórico."))),
	ui.Collapsible("Dá para reabrir um pedido fechado?",
		h.P(h.Text("Sim, em até 7 dias após o fechamento."))),
)`,
		Node: func() h.Node {
			return wrap(kit.Stack(
				kit.Collapsible("Como funciona o repasse?",
					h.P(h.Text("O pedido passa para outro atendente sem perder o histórico."))),
				kit.Collapsible("Dá para reabrir um pedido fechado?",
					h.P(h.Text("Sim, em até 7 dias após o fechamento."))),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-abas",
		Title: "Abas com teclado e ARIA, à parte do card",
		Source: `ui.Tabs("config",
	ui.Tab{Label: "Geral", Content: h.P(h.Text("Nome, fuso horário, idioma."))},
	ui.Tab{Label: "Notificações", Content: h.P(h.Text("E-mail e push, por evento."))},
	ui.Tab{Label: "Segurança", Content: h.P(h.Text("Senha e sessões abertas."))},
)`,
		Node: func() h.Node {
			return wrap(kit.Tabs("kit-config",
				kit.Tab{Label: "Geral", Content: h.P(h.Text("Nome, fuso horário, idioma."))},
				kit.Tab{Label: "Notificações", Content: h.P(h.Text("E-mail e push, por evento."))},
				kit.Tab{Label: "Segurança", Content: h.P(h.Text("Senha e sessões abertas."))},
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-grade",
		Title: "Row, Stack, Grid e Separator: os blocos de layout",
		Source: `ui.Stack(
	ui.Row(ui.H3(h.Text("Resumo do mês")), ui.Spacer(), ui.Muted(h.Text("Setembro"))),
	ui.Separator(),
	ui.Grid(
		ui.Card(ui.CardContent(ui.Muted(h.Text("Pedidos")), h.P(h.Text("128")))),
		ui.Card(ui.CardContent(ui.Muted(h.Text("Receita")), h.P(h.Text("R$ 42.300")))),
		ui.Card(ui.CardContent(ui.Muted(h.Text("Em aberto")), h.P(h.Text("9")))),
	),
)`,
		Node: func() h.Node {
			return wrap(kit.Stack(
				kit.Row(kit.H3(h.Text("Resumo do mês")), kit.Spacer(), kit.Muted(h.Text("Setembro"))),
				kit.Separator(),
				kit.Grid(
					kit.Card(kit.CardContent(kit.Muted(h.Text("Pedidos")), h.P(h.Text("128")))),
					kit.Card(kit.CardContent(kit.Muted(h.Text("Receita")), h.P(h.Text("R$ 42.300")))),
					kit.Card(kit.CardContent(kit.Muted(h.Text("Em aberto")), h.P(h.Text("9")))),
				),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-carregamento",
		Title: "Skeleton e Progress: o que aparece antes do dado chegar",
		Source: `ui.Stack(
	ui.Progress(7, 10),
	ui.Skeleton(h.StyleAttr("height: 1.2rem")),
	ui.Skeleton(h.StyleAttr("height: 1.2rem; width: 70%")),
)`,
		Node: func() h.Node {
			return wrap(kit.Stack(
				kit.Progress(7, 10),
				kit.Skeleton(h.StyleAttr("height: 1.2rem")),
				kit.Skeleton(h.StyleAttr("height: 1.2rem; width: 70%")),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-tipografia",
		Title: "Uma tecla e um trecho de código, em linha com o texto",
		Source: `h.P(
	h.Text("Abra a busca com "), ui.Kbd("⌘"), ui.Kbd("K"),
	h.Text(", ou rode "), ui.Code("trilha dev"), h.Text(" no terminal."),
)`,
		Node: func() h.Node {
			return wrap(h.P(
				h.Text("Abra a busca com "), kit.Kbd("⌘"), kit.Kbd("K"),
				h.Text(", ou rode "), kit.Code("trilha dev"), h.Text(" no terminal."),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-status",
		Title: "Status é um valor do enum, com o tom que ele declarou",
		Source: `var Status = trilha.Enum{
	{Value: "enviado", Label: "Enviado", Tone: "info"},
	{Value: "entregue", Label: "Entregue", Tone: "success"},
	{Value: "cancelado", Label: "Cancelado", Tone: "danger"},
}

ui.Row(
	ui.Status(Status, "enviado"),
	ui.Status(Status, "entregue"),
	ui.Status(Status, "cancelado"),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Status(pedidoStatusPT, "enviado"),
				kit.Status(pedidoStatusPT, "entregue"),
				kit.Status(pedidoStatusPT, "cancelado"),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-vazio",
		Title: "Nada para mostrar, e o que não deu para carregar",
		Source: `ui.Row(
	ui.Empty(ui.EmptyOpts{
		Icon:   "info",
		Title:  "Nenhum documento ainda",
		Hint:   "Envie o primeiro PDF e a classificação começa sozinha.",
		Action: ui.ButtonLink("/documentos/novo", h.Text("Enviar documento")),
	}),
	ui.EmptyError(c, "Não deu para carregar os documentos", err,
		ui.ButtonLink("/documentos", h.Text("Tentar de novo"))),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Empty(kit.EmptyOpts{
					Icon:   "info",
					Title:  "Nenhum documento ainda",
					Hint:   "Envie o primeiro PDF e a classificação começa sozinha.",
					Action: kit.ButtonLink("#", h.Text("Enviar documento")),
				}),
				kit.EmptyError(nil, "Não deu para carregar os documentos", errKitDemo,
					kit.ButtonLink("#", h.Text("Tentar de novo"))),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-etapas",
		Title: "O indicador de um formulário em várias telas",
		Source: `ui.Steps([]ui.Step{
	{Label: "Arquivo", Href: "/importar/1"},
	{Label: "Mapeamento", Href: "/importar/2"},
	{Label: "Confirmação"},
}, 2)`,
		Node: func() h.Node {
			return wrap(kit.Steps([]kit.Step{
				{Label: "Arquivo", Href: "#"},
				{Label: "Mapeamento", Href: "#"},
				{Label: "Confirmação"},
			}, 2))
		},
	})
	add("pt", Demo{
		Name:  "ui-atraso",
		Title: "A parte lenta chega um instante depois",
		Source: `ui.Container(
	ui.Grid(estatisticasRapidas...),
	ui.Defer(c, "insights", "/painel/insights", ui.DeferOpts{Height: "8rem"}),
	ui.LiveScript(c),
)
// /painel/insights responde Ctx.Fragment com um elemento id="insights";
// ui.LiveScript(c) é o que liga a troca automática.`,
		Node: func() h.Node {
			return wrap(kit.Defer(nil, "kit-atraso", "#", kit.DeferOpts{Height: "6rem"}))
		},
	})
	add("pt", Demo{
		Name:  "ui-preview",
		Title: "Documento e imagem em linha, com o que fazer quando não dá",
		Source: `ui.Row(
	ui.Preview(c, logoURL, ui.PreviewOpts{Title: "Logo", Type: "image/png"}),
	ui.Preview(c, "/anexos/relatorio.zip", ui.PreviewOpts{
		Title: "Relatório", Type: "application/zip", NoOpen: true,
	}),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Preview(nil, kitDemoLogoDataURL, kit.PreviewOpts{Title: "Logo", Type: "image/png"}),
				kit.Preview(nil, "relatorio.zip", kit.PreviewOpts{
					Title: "Relatório", Type: "application/zip", NoOpen: true,
				}),
			))
		},
	})
	add("pt", Demo{
		Name:  "ui-espera",
		Title: "O que aparece enquanto uma troca está no ar",
		Source: `ui.Row(
	ui.ButtonLink("/pedidos/atualizar", ui.Swap("lista"), ui.PendingAfter(150), ui.NoTransition(),
		h.Text("Atualizar")),
	ui.Spinner(ui.Indicator("lista")),
)
// ui.Spinner(ui.Indicator("lista")) só aparece enquanto o alvo "lista" estiver
// esperando além do limite de ui.PendingAfter; ui.NoTransition desliga o
// esmaecimento cruzado deste gatilho.`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Button(kit.Outline(), h.Text("Atualizar")),
				kit.Spinner(),
			))
		},
	})

	// ---- en ----
	add("en", Demo{
		Name:  "ui-botoes",
		Title: "Buttons: variants and sizes are attributes",
		Source: `ui.Row(
	ui.Button(h.Text("Save")),
	ui.Button(ui.Secondary(), h.Text("Draft")),
	ui.Button(ui.Outline(), h.Text("Cancel")),
	ui.Button(ui.Ghost(), ui.Icon("settings")),
	ui.Button(ui.Destructive(), ui.Sm(), ui.Icon("trash"), h.Text("Delete")),
	ui.Badge(h.Text("new")),
	ui.Badge(ui.Outline(), h.Text("beta")),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Button(h.Text("Save")),
				kit.Button(kit.Secondary(), h.Text("Draft")),
				kit.Button(kit.Outline(), h.Text("Cancel")),
				kit.Button(kit.Ghost(), kit.Icon("settings")),
				kit.Button(kit.Destructive(), kit.Sm(), kit.Icon("trash"), h.Text("Delete")),
				kit.Badge(h.Text("new")),
				kit.Badge(kit.Outline(), h.Text("beta")),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-formulario",
		Title: "Conditional fields without writing JavaScript",
		Source: `h.Form(h.Class("ui-stack"),
	ui.Field("kind", "Customer type",
		ui.Select(h.ID("kind"), h.Name("kind"),
			h.Option(h.Value("person"), h.Text("Individual")),
			h.Option(h.Value("company"), h.Text("Company")))),
	ui.Field("tax-id", "Personal tax ID", ui.Input(h.ID("tax-id"), h.Name("tax-id")),
		ui.With(ui.ShowWhen("kind", "person"))),
	ui.Field("company-id", "Company tax ID", ui.Input(h.ID("company-id"), h.Name("company-id")),
		ui.Help("14 digits"), ui.With(ui.ShowWhen("kind", "company"))),
	ui.CheckRow(ui.Switch(h.ID("invoice"), h.Name("invoice")), "Send an invoice", "invoice"),
	ui.Field("email", "E-mail for the invoice", ui.Input(h.ID("email"), h.Type("email")),
		ui.With(ui.ShowWhen("invoice"))),
)`,
		Node: func() h.Node {
			return wrap(h.Form(h.Class("ui-stack"),
				kit.Field("kind", "Customer type",
					kit.Select(h.ID("kind"), h.Name("kind"),
						h.Option(h.Value("person"), h.Text("Individual")),
						h.Option(h.Value("company"), h.Text("Company")))),
				kit.Field("tax-id", "Personal tax ID", kit.Input(h.ID("tax-id"), h.Name("tax-id")),
					kit.With(kit.ShowWhen("kind", "person"))),
				kit.Field("company-id", "Company tax ID", kit.Input(h.ID("company-id"), h.Name("company-id")),
					kit.Help("14 digits"), kit.With(kit.ShowWhen("kind", "company"))),
				kit.CheckRow(kit.Switch(h.ID("invoice"), h.Name("invoice")), "Send an invoice", "invoice"),
				kit.Field("email", "E-mail for the invoice", kit.Input(h.ID("email"), h.Type("email")),
					kit.With(kit.ShowWhen("invoice"))),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-confirmar",
		Title: "Asking before destroying, on the form itself",
		Source: `h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
	ui.Confirm("Delete this post?", "There is no undo."),
	ui.Submit(ui.Destructive(), h.Text("Delete")),
)`,
		Node: func() h.Node {
			return wrap(h.Form(h.Method("get"), h.Action("#"),
				h.Input(h.Type("hidden"), h.Name("_csrf"), h.Value("token-generated-per-request")),
				kit.Confirm("Delete this post?", "There is no undo."),
				kit.Submit(kit.Destructive(), h.Text("Delete")),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-card",
		Title: "Card, tabs and progress",
		Source: `ui.Card(
	ui.CardHeader(ui.CardTitle("Monthly goal"), ui.CardDescription("7 of 10 posts")),
	ui.CardContent(
		ui.Progress(7, 10),
		ui.Tabs("goal",
			ui.Tab{Label: "Summary", Content: h.P(h.Text("3 to go."))},
			ui.Tab{Label: "Details", Content: h.P(h.Text("Tabs with keyboard support and ARIA."))},
		),
	),
	ui.CardFooter(ui.Button(ui.Sm(), h.Text("Publish"))),
)`,
		Node: func() h.Node {
			return wrap(kit.Card(
				kit.CardHeader(kit.CardTitle("Monthly goal"), kit.CardDescription("7 of 10 posts")),
				kit.CardContent(
					kit.Progress(7, 10),
					kit.Tabs("goal",
						kit.Tab{Label: "Summary", Content: h.P(h.Text("3 to go."))},
						kit.Tab{Label: "Details", Content: h.P(h.Text("Tabs with keyboard support and ARIA."))},
					),
				),
				kit.CardFooter(kit.Button(kit.Sm(), h.Text("Publish"))),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-dialogo",
		Title: "Native dialog and a toast that dismisses itself",
		Source: `ui.Row(
	ui.DialogTrigger("confirm", ui.Outline(), h.Text("Open dialog")),
	ui.Dialog("confirm", "Publish now?",
		ui.DialogDescription("The post becomes visible to everyone."),
		ui.DialogFooter(ui.DialogClose(ui.Ghost(), h.Text("Later")), ui.DialogClose(h.Text("Publish")))),
	ui.Button(ui.Secondary(), h.Data("ui-toast", "Saved!"), h.Text("Show toast")),
)
// data-ui-toast shows a toast on click; from the server, after a POST,
// render ui.Toast("success", "Saved!", 4000) inside ui.Toaster().`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.DialogTrigger("confirm", kit.Outline(), h.Text("Open dialog")),
				kit.Dialog("confirm", "Publish now?",
					kit.DialogDescription("The post becomes visible to everyone."),
					kit.DialogFooter(kit.DialogClose(kit.Ghost(), h.Text("Later")), kit.DialogClose(h.Text("Publish")))),
				kit.Button(kit.Secondary(), h.Data("ui-toast", "Saved!"), h.Text("Show toast")),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-tabela",
		Title: "Table with nested rows (drill-down)",
		Source: `ui.Table(
	h.Thead(h.Tr(h.Th(h.Text("Account")), h.Th(ui.Num(), h.Text("Budget")), h.Th(ui.Num(), h.Text("Actual")))),
	h.Tbody(
		h.Tr(ui.Depth(0), h.Td(h.Strong(h.Text("Expenses"))), h.Td(ui.Num(), h.Text("12,000")), h.Td(ui.Num(), h.Text("11,240"))),
		h.Tr(ui.Depth(1), h.Td(h.Text("Staff")), h.Td(ui.Num(), h.Text("8,000")), h.Td(ui.Num(), h.Text("8,000"))),
		h.Tr(ui.Depth(1), h.Td(h.Text("Marketing")), h.Td(ui.Num(), h.Text("4,000")), h.Td(ui.Num(), ui.Badge(ui.Destructive(), h.Text("3,240")))),
	),
)`,
		Node: func() h.Node {
			return wrap(kit.Table(
				h.Thead(h.Tr(h.Th(h.Text("Account")), h.Th(kit.Num(), h.Text("Budget")), h.Th(kit.Num(), h.Text("Actual")))),
				h.Tbody(
					h.Tr(kit.Depth(0), h.Td(h.Strong(h.Text("Expenses"))), h.Td(kit.Num(), h.Text("12,000")), h.Td(kit.Num(), h.Text("11,240"))),
					h.Tr(kit.Depth(1), h.Td(h.Text("Staff")), h.Td(kit.Num(), h.Text("8,000")), h.Td(kit.Num(), h.Text("8,000"))),
					h.Tr(kit.Depth(1), h.Td(h.Text("Marketing")), h.Td(kit.Num(), h.Text("4,000")), h.Td(kit.Num(), kit.Badge(kit.Destructive(), h.Text("3,240")))),
				),
			))
		},
	})

	add("en", Demo{
		Name:  "ui-paginacao",
		Title: "Pagination as real links, a hint without JavaScript",
		Source: `ui.Row(
	ui.Tooltip("Only the author sees the drafts",
		ui.Button(ui.Outline(), h.Text("Drafts"))),
	ui.Pagination(ui.Pages{
		Page: 4, Total: 12,
		Href: func(n int) string { return "?page=" + strconv.Itoa(n) },
	}),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Tooltip("Only the author sees the drafts",
					kit.Button(kit.Outline(), h.Text("Drafts"))),
				kit.Pagination(kit.Pages{
					Page: 4, Total: 12,
					Href: func(n int) string { return "?page=" + strconv.Itoa(n) },
				}),
			))
		},
	})

	add("en", Demo{
		Name:  "ui-shell",
		Title: "The frame of an internal app, with the active item",
		Source: `ui.Shell(c, ui.ShellOpts{
	Brand:   h.Text("Acervo"),
	Current: "/documents",
	Nav: []ui.NavGroup{
		{Label: "Work", Items: []ui.NavItem{
			{Href: "/dashboard", Label: "Dashboard", Icon: "house"},
			{Href: "/documents", Label: "Documents", Badge: "12"},
		}},
	},
	User: ui.UserMenu{Name: "Ana Paula", Detail: "ana@acervo.com", Items: []h.Node{
		ui.MenuLink("/profile", h.Text("Profile")),
	}},
},
	ui.PageHeader("Documents",
		ui.ButtonLink("/documents/new", ui.Icon("plus"), h.Text("New document"))),
)`,
		Node: func() h.Node {
			return wrap(kit.Shell(nil, kit.ShellOpts{
				Brand:   h.Text("Acervo"),
				Current: "documents",
				Nav: []kit.NavGroup{
					{Label: "Work", Items: []kit.NavItem{
						{Href: "#", Label: "Dashboard", Icon: "house"},
						{Href: "documents", Label: "Documents", Badge: "12"},
					}},
				},
				User: kit.UserMenu{Name: "Ana Paula", Detail: "ana@acervo.com", Items: []h.Node{
					kit.MenuLink("#", h.Text("Profile")),
				}},
			},
				kit.PageHeader("Documents",
					kit.ButtonLink("#", kit.Icon("plus"), h.Text("New document"))),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-breadcrumb",
		Title: "A breadcrumb trail and the avatar of who is signed in",
		Source: `ui.Row(
	ui.Breadcrumb(
		ui.Crumb{Label: "Documents", Href: "/documents"},
		ui.Crumb{Label: "Contract 41"},
	),
	ui.Spacer(),
	ui.Avatar("AP", ""),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Breadcrumb(
					kit.Crumb{Label: "Documents", Href: "#"},
					kit.Crumb{Label: "Contract 41"},
				),
				kit.Spacer(),
				kit.Avatar("AP", ""),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-menu",
		Title: "A menu in a native popover",
		Source: `ui.Row(
	ui.MenuTrigger("actions", ui.Outline(), ui.Icon("settings"), h.Text("Actions")),
	ui.Menu("actions",
		ui.MenuItem(h.Text("Duplicate")),
		ui.MenuItem(h.Text("Archive")),
		ui.MenuLink("/documents/41/edit", h.Text("Edit")),
	),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.MenuTrigger("kit-actions", kit.Outline(), kit.Icon("settings"), h.Text("Actions")),
				kit.Menu("kit-actions",
					kit.MenuItem(h.Text("Duplicate")),
					kit.MenuItem(h.Text("Archive")),
					kit.MenuLink("#", h.Text("Edit")),
				),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-colapsavel",
		Title: "An FAQ without a line of JavaScript",
		Source: `ui.Stack(
	ui.Collapsible("How does handoff work?",
		h.P(h.Text("The ticket moves to another agent without losing the history."))),
	ui.Collapsible("Can a closed ticket be reopened?",
		h.P(h.Text("Yes, up to 7 days after it closes."))),
)`,
		Node: func() h.Node {
			return wrap(kit.Stack(
				kit.Collapsible("How does handoff work?",
					h.P(h.Text("The ticket moves to another agent without losing the history."))),
				kit.Collapsible("Can a closed ticket be reopened?",
					h.P(h.Text("Yes, up to 7 days after it closes."))),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-abas",
		Title: "Tabs with keyboard support and ARIA, on their own",
		Source: `ui.Tabs("settings",
	ui.Tab{Label: "General", Content: h.P(h.Text("Name, time zone, language."))},
	ui.Tab{Label: "Notifications", Content: h.P(h.Text("E-mail and push, per event."))},
	ui.Tab{Label: "Security", Content: h.P(h.Text("Password and open sessions."))},
)`,
		Node: func() h.Node {
			return wrap(kit.Tabs("kit-settings",
				kit.Tab{Label: "General", Content: h.P(h.Text("Name, time zone, language."))},
				kit.Tab{Label: "Notifications", Content: h.P(h.Text("E-mail and push, per event."))},
				kit.Tab{Label: "Security", Content: h.P(h.Text("Password and open sessions."))},
			))
		},
	})
	add("en", Demo{
		Name:  "ui-grade",
		Title: "Row, Stack, Grid and Separator: the layout boxes",
		Source: `ui.Stack(
	ui.Row(ui.H3(h.Text("This month's summary")), ui.Spacer(), ui.Muted(h.Text("September"))),
	ui.Separator(),
	ui.Grid(
		ui.Card(ui.CardContent(ui.Muted(h.Text("Orders")), h.P(h.Text("128")))),
		ui.Card(ui.CardContent(ui.Muted(h.Text("Revenue")), h.P(h.Text("$8,420")))),
		ui.Card(ui.CardContent(ui.Muted(h.Text("Open")), h.P(h.Text("9")))),
	),
)`,
		Node: func() h.Node {
			return wrap(kit.Stack(
				kit.Row(kit.H3(h.Text("This month's summary")), kit.Spacer(), kit.Muted(h.Text("September"))),
				kit.Separator(),
				kit.Grid(
					kit.Card(kit.CardContent(kit.Muted(h.Text("Orders")), h.P(h.Text("128")))),
					kit.Card(kit.CardContent(kit.Muted(h.Text("Revenue")), h.P(h.Text("$8,420")))),
					kit.Card(kit.CardContent(kit.Muted(h.Text("Open")), h.P(h.Text("9")))),
				),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-carregamento",
		Title: "Skeleton and Progress: what shows before the data arrives",
		Source: `ui.Stack(
	ui.Progress(7, 10),
	ui.Skeleton(h.StyleAttr("height: 1.2rem")),
	ui.Skeleton(h.StyleAttr("height: 1.2rem; width: 70%")),
)`,
		Node: func() h.Node {
			return wrap(kit.Stack(
				kit.Progress(7, 10),
				kit.Skeleton(h.StyleAttr("height: 1.2rem")),
				kit.Skeleton(h.StyleAttr("height: 1.2rem; width: 70%")),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-tipografia",
		Title: "A key and a code snippet, inline with the text",
		Source: `h.P(
	h.Text("Open search with "), ui.Kbd("⌘"), ui.Kbd("K"),
	h.Text(", or run "), ui.Code("trilha dev"), h.Text(" in the terminal."),
)`,
		Node: func() h.Node {
			return wrap(h.P(
				h.Text("Open search with "), kit.Kbd("⌘"), kit.Kbd("K"),
				h.Text(", or run "), kit.Code("trilha dev"), h.Text(" in the terminal."),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-status",
		Title: "Status is one enum value, in the tone it declared",
		Source: `var Status = trilha.Enum{
	{Value: "shipped", Label: "Shipped", Tone: "info"},
	{Value: "delivered", Label: "Delivered", Tone: "success"},
	{Value: "cancelled", Label: "Cancelled", Tone: "danger"},
}

ui.Row(
	ui.Status(Status, "shipped"),
	ui.Status(Status, "delivered"),
	ui.Status(Status, "cancelled"),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Status(orderStatusEN, "shipped"),
				kit.Status(orderStatusEN, "delivered"),
				kit.Status(orderStatusEN, "cancelled"),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-vazio",
		Title: "Nothing to show, and what could not load",
		Source: `ui.Row(
	ui.Empty(ui.EmptyOpts{
		Icon:   "info",
		Title:  "No documents yet",
		Hint:   "Send the first PDF and classification starts on its own.",
		Action: ui.ButtonLink("/documents/new", h.Text("Send a document")),
	}),
	ui.EmptyError(c, "Could not load the documents", err,
		ui.ButtonLink("/documents", h.Text("Try again"))),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Empty(kit.EmptyOpts{
					Icon:   "info",
					Title:  "No documents yet",
					Hint:   "Send the first PDF and classification starts on its own.",
					Action: kit.ButtonLink("#", h.Text("Send a document")),
				}),
				kit.EmptyError(nil, "Could not load the documents", errKitDemo,
					kit.ButtonLink("#", h.Text("Try again"))),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-etapas",
		Title: "The indicator of a form in several screens",
		Source: `ui.Steps([]ui.Step{
	{Label: "File", Href: "/import/1"},
	{Label: "Mapping", Href: "/import/2"},
	{Label: "Confirm"},
}, 2)`,
		Node: func() h.Node {
			return wrap(kit.Steps([]kit.Step{
				{Label: "File", Href: "#"},
				{Label: "Mapping", Href: "#"},
				{Label: "Confirm"},
			}, 2))
		},
	})
	add("en", Demo{
		Name:  "ui-atraso",
		Title: "The slow part arrives a moment later",
		Source: `ui.Container(
	ui.Grid(quickStats...),
	ui.Defer(c, "insights", "/panel/insights", ui.DeferOpts{Height: "8rem"}),
	ui.LiveScript(c),
)
// /panel/insights answers Ctx.Fragment with an element id="insights";
// ui.LiveScript(c) is what turns the automatic swap on.`,
		Node: func() h.Node {
			return wrap(kit.Defer(nil, "kit-atraso", "#", kit.DeferOpts{Height: "6rem"}))
		},
	})
	add("en", Demo{
		Name:  "ui-preview",
		Title: "Document and image inline, and what to do when it cannot show",
		Source: `ui.Row(
	ui.Preview(c, logoURL, ui.PreviewOpts{Title: "Logo", Type: "image/png"}),
	ui.Preview(c, "/attachments/report.zip", ui.PreviewOpts{
		Title: "Report", Type: "application/zip", NoOpen: true,
	}),
)`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Preview(nil, kitDemoLogoDataURL, kit.PreviewOpts{Title: "Logo", Type: "image/png"}),
				kit.Preview(nil, "report.zip", kit.PreviewOpts{
					Title: "Report", Type: "application/zip", NoOpen: true,
				}),
			))
		},
	})
	add("en", Demo{
		Name:  "ui-espera",
		Title: "What shows while a swap is in flight",
		Source: `ui.Row(
	ui.ButtonLink("/orders/refresh", ui.Swap("list"), ui.PendingAfter(150), ui.NoTransition(),
		h.Text("Refresh")),
	ui.Spinner(ui.Indicator("list")),
)
// ui.Spinner(ui.Indicator("list")) only shows while target "list" is still
// waiting past ui.PendingAfter; ui.NoTransition turns off the crossfade for
// this trigger.`,
		Node: func() h.Node {
			return wrap(kit.Row(
				kit.Button(kit.Outline(), h.Text("Refresh")),
				kit.Spinner(),
			))
		},
	})
}
