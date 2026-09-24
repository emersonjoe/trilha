package custos

import (
	"fmt"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	siteui "github.com/emersonjoe/trilha/site/internal/ui"
	"github.com/emersonjoe/trilha/ui"
)

// frases holds the page's words, per locale. The copy lives in code because
// the page is a table of numbers with a story around it; the story is short.
var frases = map[string]map[string]string{
	"en": {
		"title":    "The cost of a feature, measured",
		"hero":     "of average saving against pure Go",
		"pending":  "The first measurement is pending. The ruler is frozen and the baselines are committed; the numbers land here after the maintainer runs the agent ruler (make bench-agent-measure).",
		"table":    "Per scenario",
		"scenario": "Scenario",
		"baseline": "Pure Go",
		"trilha":   "Trilha",
		"saving":   "Saving",
		"rounds":   "Rounds",
		"chart":    "Saving per scenario, in percent",
		"method":   "Methodology",
	},
	"pt": {
		"title":    "O custo de uma feature, medido",
		"hero":     "de economia média contra Go puro",
		"pending":  "A primeira medição está pendente. A régua está congelada e os baselines commitados; os números chegam aqui depois que o mantenedor rodar a régua de agente (make bench-agent-measure).",
		"table":    "Por cenário",
		"scenario": "Cenário",
		"baseline": "Go puro",
		"trilha":   "Trilha",
		"saving":   "Economia",
		"rounds":   "Rodadas",
		"chart":    "Economia por cenário, em por cento",
		"method":   "Metodologia",
	},
}

func palavra(locale, key string) string { return frases[locale][key] }

// marcoCopy is the one line each milestone gets.
var marcoCopy = map[string]map[int]string{
	"en": {45: "M1 — context under budget and a gate that teaches", 60: "M2 — the platform recipes", 70: "M3 — machine-readable UI patterns and UI tests"},
	"pt": {45: "M1 — contexto sob orçamento e um portão que ensina", 60: "M2 — as receitas de plataforma", 70: "M3 — padrões de UI legíveis por máquina e testes ui-a-ui"},
}

// metodologiaCopy is the page's methodology section, in prose, per locale —
// the plan's anti-vice rules, in the order a reader asks about them.
var metodologiaCopy = map[string][]string{
	"en": {
		"The prompts of every scenario are frozen before the first measurement, one per side, with their SHA-256 committed in CHECKSUMS.txt; changing a prompt reopens the series.",
		"The comparison is against a Go-pure app committed in this repository — the same feature, the standard library alone — so the denominator of every saving is something you can read.",
		"Every number here is measured: the medians of the runs that came out green, three runs per side per scenario, tokens in (fresh plus cache reads) and out, from the agent's own result JSON. Nothing is estimated; when something is, it says est.",
		"The saving is never stored — it is derived from the two sides wherever it is read, including here. The series file records what was spent, nothing else.",
		"A release fails its gate when any scenario regresses more than five points against its previous measurement, when any scenario is under 60%, or when the average misses the milestone target.",
	},
	"pt": {
		"Os prompts de cada cenário são congelados antes da primeira medição, um por lado, com o SHA-256 commitado em CHECKSUMS.txt; mudar um prompt reabre a série.",
		"A comparação é contra um app em Go puro commitado neste repositório — a mesma feature, só a biblioteca padrão —, então o denominador de cada economia é algo que se pode ler.",
		"Tudo aqui é medido: as medianas das rodadas que saíram verdes, três execuções por lado por cenário, tokens de entrada (novos mais leitura de cache) e saída, do próprio JSON de resultado do agente. Nada é estimado; quando for, leva o rótulo est.",
		"A economia nunca é armazenada — ela é derivada dos dois lados onde quer que se leia, inclusive aqui. O arquivo da série registra o que foi gasto, e nada mais.",
		"Uma release falha no gate quando qualquer cenário regrediu mais de cinco pontos contra a medição anterior dele, quando qualquer cenário fica abaixo de 60%, ou quando a média perde a meta do marco.",
	},
}

// Page renders /custos (en) or /pt/custos (pt).
func Page(c *trilha.Ctx, locale string) (h.Node, error) {
	siteui.SetAlternate(c, "en", "/custos")
	siteui.SetAlternate(c, "pt", "/pt/custos")
	c.SetTitle(palavra(locale, "title"))
	snap := Load()
	if !snap.HasData {
		return h.Fragment(
			hero(locale, snap),
			h.Section(h.Class("bloco"),
				ui.Card(ui.CardContent(ui.Lead(h.Text(palavra(locale, "pending")))))),
			metodologia(locale),
		), nil
	}
	return h.Fragment(
		hero(locale, snap),
		h.Section(h.Class("bloco"),
			h.H2(h.Text(palavra(locale, "table"))),
			tabela(c, locale, snap),
		),
		h.Section(h.Class("bloco"),
			h.H2(h.Text(palavra(locale, "chart"))),
			ui.Bars(dadosDoGrafico(locale, snap), ui.ChartTitle(palavra(locale, "chart"))),
		),
		metodologia(locale),
	), nil
}

// hero is the number the page promises with: the average saving, the date and
// model it was measured with, and the milestone bar.
func hero(locale string, snap Snapshot) h.Node {
	meta := []h.Node{}
	if snap.HasData {
		meta = append(meta, ui.Muted(h.Textf("%s · %s · %s", snap.Date, snap.Model, snap.Env.Agent)))
	}
	return h.Section(h.Class("heroi"),
		h.H1(h.Text(palavra(locale, "title"))),
		h.Div(h.Class("custos-hero"),
			h.Span(h.Class("custos-numero"), h.Text(porcentagem(snap.Average))),
			h.Span(h.Class("custos-rotulo"), h.Text(palavra(locale, "hero"))),
		),
		h.Fragment(meta...),
		barraDaMeta(locale, snap),
	)
}

// barraDaMeta draws the goal as the distance already walked: one fill, three
// markers, the milestones named. An empty series draws the bar at zero.
func barraDaMeta(locale string, snap Snapshot) h.Node {
	alvo := Milestones[len(Milestones)-1]
	for _, m := range Milestones {
		if snap.Average*100 < float64(m) {
			alvo = m
			break
		}
	}
	proporcao := 0.0
	if alvo > 0 {
		proporcao = snap.Average * 100 / float64(alvo)
	}
	if proporcao > 1 {
		proporcao = 1
	}
	marcadores := []h.Node{}
	for _, m := range Milestones {
		pos := float64(m) / float64(alvo) * 100
		classe := "custos-marca"
		if snap.HasData && snap.Average*100 >= float64(m) {
			classe += " custos-marca-atingida"
		}
		marcadores = append(marcadores,
			h.Div(h.Class(classe), h.StyleAttr(fmt.Sprintf("left:%.0f%%", pos)),
				h.Span(h.Class("custos-marca-num"), h.Textf("%d%%", m))),
		)
	}
	return h.Div(h.Class("custos-meta"),
		h.Div(append([]h.Node{
			h.Class("custos-barra"),
			h.Div(h.Class("custos-preenchimento"), h.StyleAttr(fmt.Sprintf("width:%.0f%%", proporcao*100))),
		}, marcadores...)...),
		h.P(h.Class("ui-muted"), h.Text(marcoCopy[locale][alvo])),
	)
}

// tabela renders the per-scenario rows: a DataTable that becomes cards under
// 640px, and the saving marked when the scenario misses the floor.
func tabela(c *trilha.Ctx, locale string, snap Snapshot) h.Node {
	cols := ui.Columns[Row]{
		{Key: "scenario", Label: palavra(locale, "scenario"), Cell: func(r Row) h.Node { return h.Text(Title(r.Scenario, locale)) }},
		{Key: "baseline", Label: palavra(locale, "baseline"), Num: true, Cell: func(r Row) h.Node { return h.Text(tokens(r.Baseline)) }},
		{Key: "trilha", Label: palavra(locale, "trilha"), Num: true, Cell: func(r Row) h.Node { return h.Text(tokens(r.Trilha)) }},
		{Key: "saving", Label: palavra(locale, "saving"), Num: true, Cell: func(r Row) h.Node {
			classe := "custos-economia"
			if !r.Met {
				classe += " custos-economia-abaixo"
			}
			return h.Span(h.Class(classe), h.Text(porcentagem(r.Saving)))
		}},
		{Key: "rounds", Label: palavra(locale, "rounds"), Num: true, Cell: func(r Row) h.Node { return h.Textf("%d", r.Rounds) }},
	}
	return ui.DataTable(c, cols, snap.Rows, ui.ListState{Total: len(snap.Rows), Cards: true, Caption: palavra(locale, "table")})
}

// dadosDoGrafico is one bar per scenario: the saving in percentage points,
// with the text a person reads.
func dadosDoGrafico(locale string, snap Snapshot) []ui.Datum {
	out := make([]ui.Datum, 0, len(snap.Rows))
	for _, r := range snap.Rows {
		out = append(out, ui.Datum{Label: Title(r.Scenario, locale), Value: r.Saving * 100, Text: porcentagem(r.Saving)})
	}
	return out
}

func metodologia(locale string) h.Node {
	blocos := []h.Node{h.Class("bloco"), h.H2(h.Text(palavra(locale, "method")))}
	for _, p := range metodologiaCopy[locale] {
		blocos = append(blocos, h.P(h.Text(p)))
	}
	return h.Section(blocos...)
}

// tokens prints a token count the way the ruler's own table does.
func tokens(n int64) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// porcentagem prints a saving the way every number of this page prints: one
// decimal when it has one, none when it does not.
func porcentagem(s float64) string {
	if s == 0 {
		return "0%"
	}
	if v := s * 100; v == float64(int(v)) {
		return fmt.Sprintf("%d%%", int(v))
	}
	return fmt.Sprintf("%.1f%%", s*100)
}
