package app

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the form (empty) and the list. Uma requisição de fragmento
// (spec 018) recebe só a tela, sem os layouts: é o mesmo código, com um if.
func Page(c *trilha.Ctx) (h.Node, error) {
	return tela(c, clientes.Cliente{Tipo: "pf"}, nil, ""), nil
}

// POST validates; on errors the same page is rendered with 422, messages next
// to the fields and every value preserved; on success it redirects (PRG).
func POST(c *trilha.Ctx) error {
	var in clientes.Cliente
	if err := c.Bind(&in); err != nil {
		return err
	}
	clientes.Normalizar(&in)
	if errs := clientes.Validar(in); errs.Any() {
		return c.Render(http.StatusUnprocessableEntity, tela(c, in, errs, ""))
	}
	clientes.Salvar(in)
	if c.Fragment() != "" {
		// Sem recarga: devolve a tela nova (formulário limpo, lista com o
		// cadastro novo). A navegação normal segue no PRG de sempre.
		return c.Render(http.StatusOK, tela(c, clientes.Cliente{Tipo: "pf"}, nil, "Cadastro salvo!"))
	}
	return c.Redirect("/?ok=1")
}

// tela é o alvo das trocas: o formulário e a lista trocam juntos, então
// salvar um cliente já atualiza a tabela.
func tela(c *trilha.Ctx, in clientes.Cliente, errs trilha.FieldErrors, aviso string) h.Node {
	c.SetTitle("Cadastro de cliente")
	return h.Div(h.ID("tela"), h.Class("tela"),
		h.If(aviso != "", ui.Alert(aviso, h.Data("ui-fade", "4000"), ui.Icon("circle-check"))),
		formulario(c, in, errs),
		lista(c.Query("q")),
	)
}

// campo is a small local helper: label + input bound to the model and to the
// error map. Everything else is the kit.
func campo(id, label, value string, errs trilha.FieldErrors, attrs ...h.Node) h.Node {
	return ui.Field(id, label, ui.Input(append([]h.Node{h.ID(id), h.Name(id), h.Value(value), ui.InvalidIf(errs, id)}, attrs...)...), ui.Errors(errs, id))
}

func endereco(prefix string, a clientes.Endereco, errs trilha.FieldErrors) h.Node {
	ufs := []ui.Option{{Value: "", Label: "UF"}}
	for _, uf := range clientes.UFs() {
		ufs = append(ufs, ui.Option{Value: uf, Label: uf})
	}
	cidades := []ui.Option{{Value: "", Label: "Escolha a UF primeiro"}}
	for _, ci := range clientes.Cidades[a.UF] {
		cidades = append(cidades, ui.Option{Value: ci, Label: ci})
	}
	return h.Div(h.Class("ui-grid endereco"),
		campo(prefix+"cep", "CEP", a.CEP, errs, h.Attr("inputmode", "numeric"), h.Placeholder("00000-000")),
		campo(prefix+"rua", "Rua", a.Rua, errs),
		campo(prefix+"numero", "Número", a.Numero, errs),
		ui.Field(prefix+"uf", "UF", ui.Select(h.ID(prefix+"uf"), h.Name(prefix+"uf"), h.Data("cidades", prefix+"cidade"), ui.InvalidIf(errs, prefix+"uf"), ui.SelectOptions(ufs, a.UF)), ui.Errors(errs, prefix+"uf")),
		ui.Field(prefix+"cidade", "Cidade", ui.Select(h.ID(prefix+"cidade"), h.Name(prefix+"cidade"), h.If(a.UF == "", h.Disabled()), ui.InvalidIf(errs, prefix+"cidade"), ui.SelectOptions(cidades, a.Cidade)), ui.Errors(errs, prefix+"cidade")),
	)
}

func formulario(c *trilha.Ctx, in clientes.Cliente, errs trilha.FieldErrors) h.Node {
	return ui.Card(
		ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Novo cliente")), ui.CardDescription("Os campos mudam conforme o tipo; a validação acontece no servidor e volta para o campo certo.")),
		// ui.Swap("tela"): com JavaScript o envio troca só a tela; sem ele, o
		// mesmo formulário recarrega a página, pelo mesmo endereço.
		ui.CardContent(h.Form(h.Method("post"), h.Action("/"), h.Class("ui-stack"), h.Attr("novalidate", ""), ui.Swap("tela"), trilha.CSRFInput(c),
			h.If(errs.Any(), ui.Alert("Corrija os campos destacados", ui.Destructive(), ui.Icon("triangle-alert"))),
			h.Fieldset(h.Class("ui-stack"),
				h.Legend(h.Class("ui-label"), h.Text("Tipo")),
				ui.Row(
					ui.CheckRow(ui.Radio(h.ID("tipo-pf"), h.Name("tipo"), h.Value("pf"), ui.Checked(in.Tipo == "pf")), "Pessoa física", "tipo-pf"),
					ui.CheckRow(ui.Radio(h.ID("tipo-pj"), h.Name("tipo"), h.Value("pj"), ui.Checked(in.Tipo == "pj")), "Pessoa jurídica", "tipo-pj"),
				),
				h.If(errs.Has("tipo"), h.P(h.Class("ui-field-error"), h.Text(errs.Get("tipo")))),
			),
			h.Div(h.Class("ui-grid"),
				campo("nome", "Nome completo", in.Nome, errs, h.Autofocus()),
				campo("email", "E-mail", in.Email, errs, h.Type("email")),
			),
			h.Div(h.Class("ui-grid"), ui.ShowWhen("tipo", "pf"),
				campo("cpf", "CPF", in.CPF, errs, h.Attr("inputmode", "numeric"), h.Placeholder("000.000.000-00")),
				campo("nascimento", "Data de nascimento", in.Nascimento, errs, h.Type("date")),
			),
			h.Div(h.Class("ui-grid"), ui.ShowWhen("tipo", "pj"),
				campo("cnpj", "CNPJ", in.CNPJ, errs, h.Attr("inputmode", "numeric"), h.Placeholder("00.000.000/0000-00")),
				campo("razao_social", "Razão social", in.RazaoSocial, errs),
			),
			ui.H3(h.Text("Endereço")),
			endereco("", in.Endereco, errs),
			ui.CheckRow(ui.Switch(h.ID("cobranca_diferente"), h.Name("cobranca_diferente"), ui.Checked(in.CobrancaDif)), "Endereço de cobrança diferente", "cobranca_diferente"),
			h.Div(h.Class("ui-stack"), ui.ShowWhen("cobranca_diferente"),
				ui.H3(h.Text("Endereço de cobrança")),
				endereco("cob_", in.Cobranca, errs),
			),
			ui.CheckRow(ui.Checkbox(h.ID("novidades"), h.Name("novidades"), ui.Checked(in.Novidades)), "Quero receber novidades", "novidades"),
			h.Div(h.Class("ui-field"), ui.ShowWhen("novidades"),
				ui.Label(h.Text("Frequência")),
				ui.Row(
					ui.CheckRow(ui.Radio(h.ID("freq-semanal"), h.Name("frequencia"), h.Value("semanal"), ui.Checked(in.Frequencia == "semanal")), "Semanal", "freq-semanal"),
					ui.CheckRow(ui.Radio(h.ID("freq-mensal"), h.Name("frequencia"), h.Value("mensal"), ui.Checked(in.Frequencia == "mensal")), "Mensal", "freq-mensal"),
				),
				h.If(errs.Has("frequencia"), h.P(h.Class("ui-field-error"), h.Text(errs.Get("frequencia")))),
			),
			ui.Row(ui.Submit(h.Text("Salvar cadastro")), ui.ButtonLink("/", ui.Ghost(), h.Text("Limpar"))),
		)),
	)
}

func lista(q string) h.Node {
	todos := clientes.Buscar(q)
	return ui.Card(
		ui.CardHeader(ui.CardTitle("Cadastrados"), ui.CardDescription("Os últimos primeiro."),
			// Busca por fragmento: só a tela pisca. Sem JavaScript é um GET
			// comum, e a URL fica igual nos dois caminhos.
			h.Form(h.Method("get"), h.Action("/"), h.Class("busca"), ui.Swap("tela"),
				ui.Input(h.ID("q"), h.Name("q"), h.Type("search"), h.Value(q), h.Placeholder("Buscar por nome, documento ou cidade")),
				ui.Submit(ui.Sm(), h.Text("Buscar")),
			)),
		ui.CardContent(h.IfElse(len(todos) == 0,
			ui.Muted(h.Text(vazio(q))),
			ui.Table(
				h.Thead(h.Tr(h.Th(h.Text("Nome")), h.Th(h.Text("Documento")), h.Th(h.Text("Cidade")), h.Th(h.Text("Novidades")))),
				h.Tbody(h.Map(todos, func(c clientes.Cliente) h.Node {
					return h.Tr(
						h.Td(h.Text(c.Nome), h.Br(), ui.Muted(h.Text(c.Email))),
						h.Td(h.Text(c.Documento())),
						h.Td(h.Textf("%s/%s", c.Endereco.Cidade, c.Endereco.UF)),
						h.Td(h.IfElse(c.Novidades, ui.Badge(h.Text(c.Frequencia)), ui.Badge(ui.Outline(), h.Text("não")))),
					)
				})),
			),
		)),
	)
}

func vazio(q string) string {
	if q != "" {
		return "Nada encontrado para " + q + "."
	}
	return "Ninguém ainda."
}
