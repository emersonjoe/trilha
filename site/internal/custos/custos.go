// Package custos is the vitrine of the Tokens 70 goal: the page /custos (and
// /pt/custos) renders the savings series bench/agent measures, with the
// medians of the green runs and the average saving as its hero number.
//
// The source of truth is the series file the ruler writes; this package holds
// a committed copy of it, and site's tests refuse to let the copy drift. The
// saving is derived here the same way everywhere else derives it — never
// stored — and every number on the page is labeled as measured.
package custos

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed results.json
var raw []byte

// Raw is the committed copy of bench/agent/results/results.json.
func Raw() []byte { return raw }

// measurement is one row of the series, in the ruler's schema.
type measurement struct {
	Date        string `json:"date"`
	Model       string `json:"model"`
	Scenario    string `json:"scenario"`
	Side        string `json:"side"` // "trilha" | "baseline"
	TokensIn    int64  `json:"tokens_in"`
	TokensOut   int64  `json:"tokens_out"`
	Rounds      int    `json:"rounds"`
	FilesOpened int    `json:"files_opened"`
	Runs        int    `json:"runs"`
}

// environment is the header of the series file.
type environment struct {
	Trilha string `json:"trilha"`
	Agent  string `json:"agent"`
	Model  string `json:"model"`
}

// Milestones of the plan, in percentage points: M1, M2 and the goal itself.
var Milestones = []int{45, 60, 70}

// Floor is the smallest saving a scenario may publish with (plan §1).
const Floor = 60

// Row is one scenario's line of the page.
type Row struct {
	Scenario string // the ruler's key ("comments", "s5-login")
	Baseline int64  // median tokens of the baseline side
	Trilha   int64  // median tokens of the trilha side
	Saving   float64
	Rounds   int // median rounds of the trilha side
	Met      bool
}

// Snapshot is everything the page draws, in neither language.
type Snapshot struct {
	HasData bool
	Average float64
	Rows    []Row
	Date    string
	Model   string
	Env     environment
}

// Load derives the snapshot from the embedded series: the latest date per
// side of each scenario, the saving from those two rows, the average over the
// rows. An empty series is a valid snapshot — the page says so honestly.
func Load() Snapshot {
	var s struct {
		environment
		Measurements []measurement `json:"measurements"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		// The copy is commit-tested; a corrupt one fails the site build here.
		panic("custos: the embedded series is not valid JSON: " + err.Error())
	}
	type side struct {
		date string
		row  measurement
	}
	latest := map[string][2]*measurement{}
	for i, m := range s.Measurements {
		idx := 0
		if m.Side == "baseline" {
			idx = 1
		}
		cur := latest[m.Scenario]
		if cur[idx] == nil || m.Date >= cur[idx].Date {
			cur[idx] = &s.Measurements[i]
			latest[m.Scenario] = cur
		}
	}
	snap := Snapshot{Env: s.environment}
	var scenarios []string
	for sc := range latest {
		scenarios = append(scenarios, sc)
	}
	sort.Strings(scenarios)
	for _, sc := range scenarios {
		pair := latest[sc]
		t, b := pair[0], pair[1]
		if t == nil || b == nil {
			continue
		}
		tt, bb := t.TokensIn+t.TokensOut, b.TokensIn+b.TokensOut
		if tt == 0 || bb == 0 {
			continue
		}
		saving := 1 - float64(tt)/float64(bb)
		snap.Rows = append(snap.Rows, Row{
			Scenario: sc,
			Baseline: bb,
			Trilha:   tt,
			Saving:   saving,
			Rounds:   t.Rounds,
			Met:      saving*100 >= Floor,
		})
	}
	if len(snap.Rows) == 0 {
		return snap
	}
	snap.HasData = true
	snap.Date = latestDate(s.Measurements)
	snap.Model = s.Model
	total := 0.0
	for _, r := range snap.Rows {
		total += r.Saving
	}
	snap.Average = total / float64(len(snap.Rows))
	return snap
}

func latestDate(rows []measurement) string {
	out := ""
	for _, m := range rows {
		if m.Date > out {
			out = m.Date
		}
	}
	return out
}

// titles are the human names of the scenarios, per locale.
var titles = map[string]map[string]string{
	"comments":      {"en": "API route with validation", "pt": "Rota de API com validação"},
	"contact-form":  {"en": "Contact form with the ui kit", "pt": "Formulário de contato com o kit ui"},
	"pagination":    {"en": "Paginate a listing", "pt": "Paginar uma listagem"},
	"generate-crud": {"en": "CRUD from a struct", "pt": "CRUD a partir de um struct"},
	"s5-login":      {"en": "Install a working login", "pt": "Instalar login funcional"},
	"s6-crud":       {"en": "Full CRUD: JSON API and page", "pt": "CRUD completo: API JSON e página"},
	"s7-tela":       {"en": "Listing with filter, pages and chart", "pt": "Listagem com filtro, páginas e gráfico"},
	"s8-conserto":   {"en": "Fix three defects guided by check", "pt": "Consertar três erros guiado pelo check"},
}

// Title names a scenario for people, in the locale of the page.
func Title(scenario, locale string) string {
	if t, ok := titles[scenario]; ok {
		if s, ok := t[locale]; ok {
			return s
		}
	}
	return scenario
}
