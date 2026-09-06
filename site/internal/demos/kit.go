package demos

import (
	"strconv"

	"github.com/emersonjoe/trilha/h"
	kit "github.com/emersonjoe/trilha/ui"
)

// Demos of the ui kit. The result pane is wrapped in .ui-body.kit so the kit's
// styles apply inside the docs site without touching the rest of the page.
func wrap(n h.Node) h.Node { return h.Div(h.Class("ui-body ui-stack kit"), n) }

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
}
