---
title: Examples
description: Four complete apps in examples/, from basic to complex, and what each one teaches.
---

The examples are real apps, with integration tests that run in the repository's `make test`.
Each has a short `README.md`. Run any of them with `trilha dev` inside the folder (or
`go run ../../cmd/trilha dev` from the clone).

:::note
The example apps are written in Portuguese: folder names, identifiers and UI texts (for
instance `app/blog/novo` is "new post", `cadastro` is "sign-up", `orcamento` is "budget").
The code is the same Trilha you read about in English here; only the words differ.
:::

| Level | Folder | What it teaches |
|---|---|---|
| Basic | `examples/blog` | every file convention, nested layouts, route groups, JSON API, middleware, signed session, `tmpl` |
| Medium | `examples/cadastro` | a form with rules: conditional fields, server-side validation with per-field errors, dependent select, disappearing toast, responsive layout |
| Complex | `examples/orcamento` | tree-shaped domain (chart of accounts), aggregation, drill-down through a dynamic route, nested and recursive components, dialog with a form, period filter, CSV |
| AI | `examples/assistente` | streaming chat, agent with tools, handoff, MCP server |

## Medium: sign-up (`cadastro`)

The form model is a struct with `form` tags; `c.Bind(&in)` fills it (nested structs are
flattened, with an optional prefix):

```go
type Cliente struct {
	Tipo     string   `form:"tipo"`      // type
	Nome     string   `form:"nome"`      // name
	Endereco Endereco            // cep, rua, uf, cidade (address)
	Cobranca Endereco `form:"cob_"` // cob_cep, cob_rua... (billing address)
	Novidades bool    `form:"novidades"` // newsletter
}
```

Validation is a pure function returning `trilha.FieldErrors`, and `POST` decides:

```go
func POST(c *trilha.Ctx) error {
	var in clientes.Cliente
	if err := c.Bind(&in); err != nil {
		return err                       // invalid conversion → 422
	}
	clientes.Normalizar(&in)             // drops what the type does not use
	if errs := clientes.Validar(in); errs.Any() {
		return c.Render(422, tela(c, in, errs)) // same page, with layouts
	}
	clientes.Salvar(in)
	return c.Redirect("/?ok=1")          // PRG + disappearing toast
}
```

On screen, each field reads its value and its error from the same place:

```go
ui.Field("cnpj", "CNPJ",
	ui.Input(h.ID("cnpj"), h.Name("cnpj"), h.Value(in.CNPJ), ui.InvalidIf(errs, "cnpj")),
	ui.Errors(errs, "cnpj"))
```

Conditional groups use `ui.ShowWhen("tipo", "pj")`: hidden ones are disabled and do not
travel in the `POST`; and since anyone can craft the `POST` by hand, `Normalizar` clears what
the type does not use before validating. The city `<select>` is filled by
`GET /api/cidades?uf=` with 20 lines of `app.js`; on a 422 the server already returns the
cities of the chosen state, so the page comes back complete without JavaScript.

## Complex: budget (`orcamento`)

The chart of accounts is a tree (`Conta{Codigo, Nome, Filhos}`); budgeted and actual values
of a summary account are the sum of its children, computed on read. The components mirror
the tree: `Linha` renders the account and calls itself for the children, `ui.Depth(n)`
indents:

```go
func Linha(c *plano.Conta, mes string, nivel, max int) h.Node {
	row := h.Tr(ui.Depth(nivel), h.Td(h.A(h.Href("/contas/"+c.Codigo), h.Text(c.Nome))), ...)
	if nivel >= max || c.Analitica() {
		return row
	}
	return h.Fragment(row, h.Map(c.Filhos, func(f *plano.Conta) h.Node {
		return Linha(f, mes, nivel+1, max)
	}))
}
```

The drill-down is the route `app/contas/codigo_/page.go`: breadcrumb with `Caminho()`,
children (same `Tabela`) or entries (leaf account). The entry form is **a single one**
(`FormLancamento`), used inside `ui.Dialog` in the overview and in the drill-down, and on
its own at `/lancamentos`; `POST` validates with `c.Bind` + `plano.Validar` and, on a 422,
`app.js` reopens the dialog because it found `.ui-field-error` inside it. `voltar` (a hidden
field) says where to redirect on success. The export lives in
`app/api/relatorio.csv/route.go`, a folder with a dot in its name.

## What became framework

Writing the two examples exposed repetition that is now API: `c.Bind`, `trilha.FieldErrors`,
`c.Render` (a page with layouts from a `POST`), `ui.Errors`, `ui.InvalidIf`,
`ui.SelectOptions`, `ui.Checked`. That is the constitution's criterion: an example that needs
repetitive code points to a gap in Trilha, not in the example.

## Challenge

In the budget app, add a "Year" column to the drill-down that sums the account's twelve
months.

:::solution
```go
func Ano(c *plano.Conta, ano string) (orcado, real int64) {
	for m := 1; m <= 12; m++ {
		mes := fmt.Sprintf("%s-%02d", ano, m)
		orcado += plano.Orcado(c, mes)
		real += plano.Realizado(c, mes)
	}
	return
}
```
Call it from `Linha` and add the two cells; since aggregation is recursive, the column
already works for summary accounts.
:::
