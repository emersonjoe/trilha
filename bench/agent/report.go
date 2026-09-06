package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Run is one execution of one scenario.
type Run struct {
	Scenario string `json:"scenario"`
	N        int    `json:"n"`
	// Variant says which fixture the run measured: "" is the bare example,
	// "agents" the same example with the AGENTS.md of spec 044. Two variants
	// never share a median.
	Variant string    `json:"variant,omitempty"`
	At      time.Time `json:"at"`
	Usage   Usage     `json:"usage"`
	Passed  bool      `json:"passed"`
	Verify  string    `json:"verify,omitempty"` // tail of the failing output, when it failed
	Summary string    `json:"summary,omitempty"`
}

// Results is the file bench/agent/results.json: the environment the runs
// were made in, and the runs.
type Results struct {
	Trilha  string `json:"trilha"`
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	Machine string `json:"machine"`
	Date    string `json:"date"`
	Runs    []Run  `json:"runs"`
}

// Load reads results.json; a missing file is an empty result set.
func Load(path string) (Results, error) {
	var r Results
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}

// Save writes results.json, indented, so a diff of it reads.
func Save(path string, r Results) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Median of xs; zero for none. Even counts take the mean of the middle two.
func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if n := len(s); n%2 == 1 {
		return s[n/2]
	} else {
		return (s[n/2-1] + s[n/2]) / 2
	}
}

// row is one line of the table: the median of every column over the runs.
type row struct {
	Scenario                                 string
	Runs, Passed                             int
	Input, CacheRead, Output, Turns, Denials float64
	Seconds, Cost                            float64
	Errors                                   int
}

// variant names the fixture a run measured.
func variant(agentsMD bool) string {
	if agentsMD {
		return "agents"
	}
	return ""
}

// variants lists the fixtures measured so far, bare one first.
func variants(r Results) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, run := range r.Runs {
		if !seen[run.Variant] {
			seen[run.Variant] = true
			out = append(out, run.Variant)
		}
	}
	sort.Strings(out)
	return out
}

func rows(r Results, order []string, v string) []row {
	by := map[string][]Run{}
	for _, run := range r.Runs {
		if run.Variant != v {
			continue
		}
		by[run.Scenario] = append(by[run.Scenario], run)
	}
	names := order
	if len(names) == 0 {
		for n := range by {
			names = append(names, n)
		}
		sort.Strings(names)
	}
	var out []row
	for _, name := range names {
		runs := by[name]
		if len(runs) == 0 {
			continue
		}
		rw := row{Scenario: name, Runs: len(runs)}
		var in, cr, o, t, d, s, c []float64
		for _, run := range runs {
			if run.Passed {
				rw.Passed++
			}
			if run.Usage.Error != "" {
				rw.Errors++
			}
			u := run.Usage
			in = append(in, float64(u.Input))
			cr = append(cr, float64(u.CacheRead))
			o = append(o, float64(u.Output))
			t = append(t, float64(u.Turns))
			d = append(d, float64(u.Denials))
			s = append(s, float64(u.DurationMs)/1000)
			c = append(c, u.CostUSD)
		}
		rw.Input, rw.CacheRead, rw.Output = Median(in), Median(cr), Median(o)
		rw.Turns, rw.Denials, rw.Seconds, rw.Cost = Median(t), Median(d), Median(s), Median(c)
		out = append(out, rw)
	}
	return out
}

// Render writes RESULTS.md: the environment, the table (median per column,
// passed as n/runs) and the methodology. The methodology is in the file and
// not only in the docs because the file is what gets pasted around.
func Render(r Results, scenarios []Scenario) string {
	var order []string
	titles := map[string]string{}
	for _, s := range scenarios {
		order = append(order, s.Name)
		titles[s.Name] = s.Title
	}
	var sb strings.Builder
	sb.WriteString("# Custo por feature de um agente\n\n")
	if len(r.Runs) == 0 {
		sb.WriteString("**Ainda sem medição.** Rode `claude auth login` e depois `make bench-agent`; este arquivo é regravado a partir de `results.json`.\n\n")
	} else {
		fmt.Fprintf(&sb, "Gerado em %s por `make bench-agent` — Trilha %s, agente %s, modelo %s, %s.\n\n", r.Date, r.Trilha, r.Agent, r.Model, r.Machine)
		vs := variants(r)
		for _, v := range vs {
			if len(vs) > 1 {
				fmt.Fprintf(&sb, "### %s\n\n", variantTitle[v])
			}
			sb.WriteString("| Cenário | Entrada | Cache lido | Saída | Rodadas | Negados | Tempo (s) | Custo (US$) | Passou |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
			for _, rw := range rows(r, order, v) {
				passed := fmt.Sprintf("%d/%d", rw.Passed, rw.Runs)
				if rw.Errors > 0 {
					passed += fmt.Sprintf(" (%d erro)", rw.Errors)
				}
				fmt.Fprintf(&sb, "| `%s` | %s | %s | %s | %s | %s | %.0f | %.2f | %s |\n",
					rw.Scenario, n(rw.Input), n(rw.CacheRead), n(rw.Output), n(rw.Turns), n(rw.Denials), rw.Seconds, rw.Cost, passed)
			}
			sb.WriteString("\n")
		}
		if len(vs) > 1 {
			sb.WriteString(delta(r, order, vs))
		}
		sb.WriteString("Mediana de cada coluna sobre as execuções; *Passou* é quantas execuções deixaram o teste escondido verde.\n\n")
	}
	sb.WriteString(methodology)
	sb.WriteString("\n## Cenários\n\n")
	for _, s := range scenarios {
		fmt.Fprintf(&sb, "### `%s` — %s\n\nFixture: `%s`.\n\n> %s\n\n", s.Name, s.Title, s.Example, strings.ReplaceAll(strings.TrimSpace(s.Prompt), "\n", "\n> "))
	}
	return sb.String()
}

// variantTitle names each fixture in the report.
var variantTitle = map[string]string{
	"":       "Sem `AGENTS.md`",
	"agents": "Com `AGENTS.md` (`trilha new --agents`)",
}

// delta compares the last variant with the bare one, scenario by scenario:
// the number the spec 044 has to justify.
func delta(r Results, order, vs []string) string {
	base := map[string]row{}
	for _, rw := range rows(r, order, "") {
		base[rw.Scenario] = rw
	}
	last := vs[len(vs)-1]
	if last == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("### Diferença\n\n| Cenário | Rodadas | Cache lido | Tempo | Custo |\n|---|---:|---:|---:|---:|\n")
	for _, rw := range rows(r, order, last) {
		b, ok := base[rw.Scenario]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "| `%s` | %s | %s | %s | %s |\n", rw.Scenario,
			pct(b.Turns, rw.Turns), pct(b.CacheRead, rw.CacheRead), pct(b.Seconds, rw.Seconds), pct(b.Cost, rw.Cost))
	}
	sb.WriteString("\nNegativo é menos: menos rodadas, menos tokens, menos tempo, menos dinheiro.\n\n")
	sb.WriteString("As duas tabelas medem a **mesma** fixture, com o `AGENTS.md` como única diferença. " +
		"Quando o exemplo muda de forma, a comparação morre e a linha de base tem que ser remedida — " +
		"uma tabela contra outro exemplo mediria a troca do exemplo.\n\n")
	return sb.String()
}

// pct renders the change from before to after, in percent.
func pct(before, after float64) string {
	if before == 0 {
		return "—"
	}
	return fmt.Sprintf("%+.0f%%", (after-before)/before*100)
}

func n(f float64) string {
	if f >= 1000 {
		return fmt.Sprintf("%.1fk", f/1000)
	}
	return fmt.Sprintf("%.0f", f)
}

const methodology = `## Metodologia

- **O que conta.** Tokens que o agente mandou (*Entrada*: novos, incluindo o que foi escrito
  no cache) e leu de volta do cache (*Cache lido*), tokens que produziu (*Saída*), rodadas de
  modelo (*Rodadas*), chamadas de ferramenta recusadas (*Negados*), tempo de parede e o custo
  que o próprio agente informa. Tudo vem do JSON de ` + "`claude -p --output-format json`" + `.
- **O que não conta.** Nada é descontado: um comando repetido, um arquivo aberto à toa e uma
  assinatura errada são exatamente o que a régua quer ver.
- **Isolamento.** O agente roda com cwd numa cópia do exemplo em diretório temporário, com
  ` + "`go.mod`" + ` próprio e ` + "`replace`" + ` para uma cópia somente-leitura do repositório no mesmo
  diretório (o agente lê o framework como leria o cache de módulos, mas não o altera), a CLI
  ` + "`trilha`" + ` compilada no ` + "`PATH`" + `, sem servidores MCP, sem plugins, sem memória do usuário
  (` + "`--setting-sources project`" + `): só o que está dentro do projeto conta, que é o que a #46
  (` + "`AGENTS.md`" + `) muda.
- **Negados.** A lista de comandos liberados cobre o que um ciclo Go precisa (` + "`go`" + `, ` + "`gofmt`" + `,
  ` + "`trilha`" + `, ` + "`make`" + ` e utilitários de leitura). Uma recusa é o agente pedindo algo fora dela; a
  coluna existe para que um vão na lista apareça em vez de virar custo do framework, e
  ` + "`results.json`" + ` guarda o comando recusado.
- **Passou.** Depois do agente, um teste escondido é copiado para a cópia e ` + "`go vet ./...`" + ` +
  ` + "`go test ./...`" + ` rodam. Verde é passou; o resto, não. O teste falha na fixture intocada, e
  ` + "`go test ./...`" + ` do módulo ` + "`bench`" + ` prova isso sem agente nenhum.
- **Três execuções, mediana.** Cada cenário roda três vezes; a tabela mostra a mediana de
  cada coluna e quantas execuções passaram.
- **Antes × depois.** A comparação é sempre Trilha contra Trilha: mesma tarefa, mesmo
  agente, mesmo modelo, versões diferentes. Nunca contra outro framework (spec 011).
- **Duas fixtures.** ` + "`make bench-agent`" + ` mede o projeto cru; ` + "`make bench-agent-agents`" + ` mede o
  mesmo projeto com o ` + "`AGENTS.md`" + ` que a 0.36.0 escreve (` + "`trilha new --agents`" + `). Cada uma tem
  sua tabela e sua mediana; a seção *Diferença* é a segunda contra a primeira.
- **Reproduzir.** ` + "`claude auth login`" + `, depois ` + "`make bench-agent`" + ` (12 execuções, dezenas de minutos e
  custo real). ` + "`make bench-agent-dry`" + ` monta os cenários sem gastar token.
`
