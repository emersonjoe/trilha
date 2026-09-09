package app

import (
	"net/http"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/setores"
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
	lidos := trilha.FieldErrors{}
	if err := c.Bind(&in); err != nil {
		// Uma linha de dependente acima do teto da tag volta como FieldErrors,
		// com a chave do item; o resto é erro de verdade.
		fe, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		lidos = fe
	}
	clientes.Normalizar(&in)
	errs := clientes.Validar(in)
	for campo, msg := range lidos {
		errs.Add(campo, msg)
	}
	if errs.Any() {
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
	return h.Div(h.Class("ui-grid endereco"),
		campo(prefix+"cep", "CEP", a.CEP, errs, h.Attr("inputmode", "numeric"), h.Placeholder("00000-000")),
		campo(prefix+"rua", "Rua", a.Rua, errs),
		campo(prefix+"numero", "Número", a.Numero, errs),
		ui.Field(prefix+"uf", "UF", ui.Select(h.ID(prefix+"uf"), h.Name(prefix+"uf"), ui.InvalidIf(errs, prefix+"uf"), ui.SelectOptions(ufs, a.UF)), ui.Errors(errs, prefix+"uf")),
		// A cidade é um combobox servido por fragmento: o servidor busca dentro
		// da UF escolhida (With), e sem JavaScript o texto digitado vai junto e
		// o servidor resolve. As vinte linhas de app.js que faziam isso sumiram.
		ui.Field(prefix+"cidade", "Cidade", ui.Combobox(ui.ComboboxOpts{
			Name: prefix + "cidade", Value: a.Cidade, Label: primeiro(a.Cidade, a.CidadeQ),
			Source: "/cidades/busca", With: []string{prefix + "uf"},
			Placeholder: "Digite o começo do nome",
		}, ui.InvalidIf(errs, prefix+"cidade")), ui.Errors(errs, prefix+"cidade")),
	)
}

// primeiro devolve o rótulo que a pessoa deve ver: o que já foi escolhido, ou
// o que ela digitou e o servidor não reconheceu.
func primeiro(escolhido, digitado string) string {
	if escolhido != "" {
		return escolhido
	}
	return digitado
}

// dependentes desenha a lista de sub-registros: cada linha é
// dependentes[i].nome, que é o nome do input, a chave da mensagem e o que o
// Bind lê de volta. A linha vazia do fim é a próxima — sem JavaScript nenhum.
func dependentes(in clientes.Cliente, errs trilha.FieldErrors) h.Node {
	linhas := append(append([]clientes.Dependente{}, in.Dependentes...), clientes.Dependente{})
	rows := make([]h.Node, 0, len(linhas))
	for i, d := range linhas {
		row := "dependentes[" + strconv.Itoa(i) + "]."
		rows = append(rows, h.Div(h.Class("ui-grid"),
			campo(row+"nome", "Nome do dependente", d.Nome, errs),
			campo(row+"nascimento", "Nascimento", d.Nascimento, errs, h.Type("date")),
		))
	}
	return h.Div(h.Class("ui-stack"),
		ui.H3(h.Text("Dependentes")),
		ui.Muted(h.Text("Cada linha vira um item da lista; a linha em branco é ignorada.")),
		h.Group(rows...),
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
			// A árvore de setores: dá para navegar ou para procurar, e o que o
			// formulário posta é um radio — sem script, é o mesmo campo.
			ui.Field("setor", "Setor", ui.TreePicker(ui.TreePickerOpts{
				Name:        "setor",
				Value:       in.Setor,
				Nodes:       arvore(in.Setor),
				Source:      "/setores/nos",
				Search:      "/setores/busca",
				Placeholder: "Buscar por código ou nome",
				Label:       "Setores",
			}), ui.Errors(errs, "setor")),
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
			dependentes(in, errs),
			ui.CheckRow(ui.Switch(h.ID("cobranca_diferente"), h.Name("cobranca_diferente"), ui.Checked(in.CobrancaDif)), "Endereço de cobrança diferente", "cobranca_diferente"),
			h.Div(h.Class("ui-stack"), ui.ShowWhen("cobranca_diferente"),
				ui.H3(h.Text("Endereço de cobrança")),
				endereco("cob_", in.Cobranca, errs),
			),
			ui.CheckRow(ui.Checkbox(h.ID("novidades"), h.Name("novidades"), ui.Checked(in.Novidades)), "Quero receber novidades", "novidades"),
			h.Div(h.Class("ui-field"), ui.ShowWhen("novidades"),
				ui.Label(h.Text("Frequência")),
				ui.Row(
					// The radios come out of the same declaration as the badge and
					// the validation: three uses, one list.
					h.Map(clientes.Frequencias, func(f trilha.EnumValue) h.Node {
						id := "freq-" + f.Value
						return ui.CheckRow(ui.Radio(h.ID(id), h.Name("frequencia"), h.Value(f.Value),
							ui.Checked(in.Frequencia == f.Value)), clientes.Frequencias.Label(f.Value), id)
					}),
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
			//
			// Esta busca lê um mapa em memória e responde em microssegundos, e
			// quem busca busca de novo. Então o limiar sobe para 250 ms — o
			// spinner só aparece se a rede estiver ruim de verdade — e o
			// crossfade sai: numa sequência de buscas ele vira cintilação, não
			// suavidade.
			h.Form(h.Method("get"), h.Action("/"), h.Class("busca"), ui.Swap("tela"),
				ui.PendingAfter(250), ui.NoTransition(),
				ui.Input(h.ID("q"), h.Name("q"), h.Type("search"), h.Value(q), h.Placeholder("Buscar por nome, documento ou cidade")),
				ui.Submit(ui.Sm(), h.Text("Buscar")),
				ui.Spinner(ui.Indicator("tela")),
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
						h.Td(h.IfElse(c.Novidades, ui.Status(clientes.Frequencias, c.Frequencia), ui.Badge(ui.Outline(), h.Text("não")))),
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

// arvore monta as raízes com o caminho até o escolhido já aberto: um
// formulário que voltou com erro volta mostrando a escolha, não uma árvore
// fechada em que a pessoa tem de achar de novo onde estava.
func arvore(escolhido string) []ui.TreeNode {
	abertos := map[string]bool{}
	for _, a := range setores.Abertos(escolhido) {
		abertos[a] = true
	}
	var monta func(pai string) []ui.TreeNode
	monta = func(pai string) []ui.TreeNode {
		var out []ui.TreeNode
		for _, s := range setores.Filhos(pai) {
			n := ui.TreeNode{Value: s.Codigo, Label: setores.Rotulo(s), Leaf: setores.Folha(s.Codigo)}
			if abertos[s.Codigo] {
				n.Open = true
				n.Children = monta(s.Codigo)
			}
			out = append(out, n)
		}
		return out
	}
	return monta("")
}
