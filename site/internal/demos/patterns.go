package demos

import (
	"strings"

	"github.com/emersonjoe/trilha/h"
	kit "github.com/emersonjoe/trilha/ui"
)

// demoPattern is which screen pattern each demo shows (spec 163). The card
// of these demos ends with "Use this pattern": the same data an agent reads
// from `trilha ui patterns` and get_pattern, so the page and the tool cannot
// tell two stories.
var demoPattern = map[string]string{
	"ui-listagem":              "list-with-filter",
	"ui-formulario-assincrono": "async-form",
	"ui-aprovacoes":            "approval-inbox",
	"ui-indicadores":           "dashboard-chart",
	"ui-tabela":                "master-detail",
	"ui-gravador":              "upload-progress",
}

// PatternOf answers the pattern a demo shows, if any.
func PatternOf(demo string) (string, bool) {
	p, ok := demoPattern[demo]
	return p, ok
}

// patternWords are the headings of the section, per locale.
var patternWords = map[string][5]string{
	"en": {"Use this pattern", "Components", "Data it needs", "Accessibility", "The whole page.go (compiles, at most 60 lines)"},
	"pt": {"Usar este padrão", "Componentes", "Dados de que precisa", "Acessibilidade", "O page.go inteiro (compila, no máximo 60 linhas)"},
}

// PatternPT is the Portuguese of every sentence a pattern carries — the
// summary and the accessibility notes are English in Go, as the code is, and
// the Portuguese site shows them from here. A sentence missing from this
// table fails TestPatternsPageBilingual.
var PatternPT = map[string]string{
	"A listing somebody searches, filters, orders and pages, with all of it in the address.":               "Uma listagem que alguém busca, filtra, ordena e pagina, com tudo isso no endereço.",
	"The table has a caption (ListState.Caption) that a screen reader announces.":                          "A tabela tem uma legenda (ListState.Caption) que o leitor de tela anuncia.",
	"The filter select carries an aria-label, since it has no visible label.":                              "O select do filtro leva um aria-label, já que não tem rótulo visível.",
	"Sorting and paging are real links: they work with the keyboard and without JavaScript.":               "Ordenar e paginar são links de verdade: funcionam pelo teclado e sem JavaScript.",
	"A form that posts without leaving the page and answers a 422 beside the field that is wrong.":         "Um formulário que posta sem sair da página e responde um 422 ao lado do campo errado.",
	"On a 422 the kit swaps the form and moves focus to the first field with aria-invalid.":                "Num 422 o kit troca o formulário e leva o foco ao primeiro campo com aria-invalid.",
	"ui.Field ties the message to the input with aria-describedby.":                                        "O ui.Field liga a mensagem ao campo com aria-describedby.",
	"ui.FormError is the summary a screen reader hears when the answer arrives.":                           "O ui.FormError é o resumo que o leitor de tela ouve quando a resposta chega.",
	"What waits for a person's decision, with approve and reject and the reason in the same form.":         "O que espera a decisão de uma pessoa, com aprovar, recusar e o motivo no mesmo formulário.",
	"Each row's buttons name the request they decide, not only \"Approve\".":                               "Os botões de cada linha nomeiam o pedido que decidem, e não só \"Aprovar\".",
	"A late row says so in text as well as in color.":                                                      "Uma linha atrasada diz isso em texto, e não só na cor.",
	"The reason field has a label and is part of the same form as the buttons.":                            "O campo do motivo tem rótulo e faz parte do mesmo formulário dos botões.",
	"The numbers on top and the drawings beside them, server-rendered SVG with no chart library.":          "Os números em cima e os desenhos ao lado, SVG feito no servidor sem biblioteca de gráfico.",
	"Every chart carries a ui.ChartTitle, which becomes the SVG's accessible name.":                        "Todo gráfico leva um ui.ChartTitle, que vira o nome acessível do SVG.",
	"The values are in the chart's text as well, so the numbers are read and not only drawn.":              "Os valores estão também no texto do gráfico, então os números são lidos e não só desenhados.",
	"The cards stack to one column on a narrow screen (ui.Cols(1, 2)).":                                    "Os cartões viram uma coluna só numa tela estreita (ui.Cols(1, 2)).",
	"A list on one side and the chosen row on the other, the choice in the address.":                       "Uma lista de um lado e a linha escolhida do outro, a escolha no endereço.",
	"Each row is a link (ListState.RowHref), so the keyboard reaches the detail.":                          "Cada linha é um link (ListState.RowHref), então o teclado chega ao detalhe.",
	"The detail is a section with an aria-label, a landmark to jump to.":                                   "O detalhe é uma seção com aria-label, um ponto de referência para onde pular.",
	"The choice lives in the address: back and reload keep it.":                                            "A escolha mora no endereço: voltar e recarregar a mantêm.",
	"Files sent with a progress bar and checked by content on the server, working without JavaScript too.": "Arquivos enviados com barra de progresso e conferidos pelo conteúdo no servidor, funcionando também sem JavaScript.",
	"The progress bar has an aria-label and is a real <progress>, read as a percentage.":                   "A barra de progresso tem aria-label e é um <progress> de verdade, lido como porcentagem.",
	"The file input has a visible label that says the limits before anybody tries.":                        "O campo de arquivo tem rótulo visível que diz os limites antes de alguém tentar.",
	"A refused file comes back as a 422 with the message on the field and the focus on it.":                "Um arquivo recusado volta como 422 com a mensagem no campo e o foco nele.",
}

// sayIn is a pattern sentence in the locale.
func sayIn(locale, en string) string {
	if locale == "pt" {
		if pt, ok := PatternPT[en]; ok {
			return pt
		}
	}
	return en
}

// usePattern is the section under a demo: the pattern's sentence, its
// components, the data it needs, the accessibility notes and the page.go,
// collapsed so the demo stays the first thing read.
func usePattern(locale, name string) h.Node {
	var p kit.Pattern
	for _, x := range kit.Patterns() {
		if x.Name == name {
			p = x
		}
	}
	if p.Name == "" {
		return h.Group()
	}
	w := patternWords[locale]
	if w[0] == "" {
		w = patternWords["en"]
	}
	comps := make([]h.Node, 0, len(p.Components))
	for _, c := range p.Components {
		comps = append(comps, h.Code(h.Text("ui."+c)), h.Text(" "))
	}
	notes := make([]h.Node, 0, len(p.A11y))
	for _, n := range p.A11y {
		notes = append(notes, h.Li(h.Text(sayIn(locale, n))))
	}
	return h.Details(h.Class("demo-padrao"), h.ID("pattern-"+p.Name),
		h.Summary(h.Text(w[0]+" — "), h.Code(h.Text(p.Name))),
		h.P(h.Text(sayIn(locale, p.Summary))),
		h.H4(h.Text(w[1])), h.P(comps...),
		h.H4(h.Text(w[2])), h.P(h.Code(h.Text(p.Data))),
		h.H4(h.Text(w[3])), h.Ul(notes...),
		h.H4(h.Text(w[4])),
		h.Div(h.Class("codigo"), h.Data("lang", "go"),
			h.Pre(h.Code(h.Class("lang-go"), h.Raw(highlight(strings.TrimRight(p.Snippet, "\n")))))),
		h.P(h.Code(h.Text("trilha ui patterns "+p.Name)), h.Text(" · "), h.Code(h.Text(`get_pattern {"name":"`+p.Name+`"}`))),
	)
}
