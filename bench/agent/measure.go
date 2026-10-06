package main

// The savings series (spec 159): what the agent spent on each side of each
// scenario, one row per (date, scenario, side), medians of the runs that came
// out green. The saving itself is never stored — it is derived wherever it is
// read, which is what keeps a row from quietly becoming a claim.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Sides of the series.
const (
	SideTrilha   = "trilha"
	SideBaseline = "baseline"
)

// Measurement is one row: the median spend of the green runs of one scenario,
// on one side, on one date. TokensIn is what the agent sent fresh plus what it
// read back from cache; TokensOut is what it produced. FilesOpened is the
// files written during the run — reads never reach the result JSON, so writes
// are the honest lower bound of "opened".
type Measurement struct {
	Date        string `json:"date"` // YYYY-MM-DD
	Model       string `json:"model"`
	Scenario    string `json:"scenario"`
	Side        string `json:"side"` // "trilha" | "baseline"
	TokensIn    int64  `json:"tokens_in"`
	TokensOut   int64  `json:"tokens_out"`
	Rounds      int    `json:"rounds"`
	FilesOpened int    `json:"files_opened"`
	Runs        int    `json:"runs"` // how many green runs the medians come from
}

// Series is results/results.json: the environment of the latest write and the
// rows. A missing file is an empty series, and an empty series is an honest
// page — no row is better than a row nobody measured.
type Series struct {
	Trilha       string        `json:"trilha"`
	Agent        string        `json:"agent"`
	Model        string        `json:"model"`
	Machine      string        `json:"machine"`
	Measurements []Measurement `json:"measurements"`
}

// LoadSeries reads the series file; a missing file is an empty series.
func LoadSeries(path string) (Series, error) {
	var s Series
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

// SaveSeries writes the series, indented, so a diff of it reads.
func SaveSeries(path string, s Series) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Savings derives, per scenario, 1 − trilha/baseline over the tokens of the
// latest date each side measured. A scenario without a baseline — or without
// tokens on one side — has no saving, and none is invented for it.
func Savings(trilha, baseline []Measurement) map[string]float64 {
	latestT := latestByScenario(trilha)
	latestB := latestByScenario(baseline)
	out := make(map[string]float64, len(latestT))
	for sc, t := range latestT {
		b, ok := latestB[sc]
		if !ok {
			continue
		}
		tt := t.TokensIn + t.TokensOut
		bb := b.TokensIn + b.TokensOut
		if tt == 0 || bb == 0 {
			continue
		}
		out[sc] = 1 - float64(tt)/float64(bb)
	}
	return out
}

// latestByScenario keeps the row with the greatest date per (scenario, side).
func latestByScenario(rows []Measurement) map[string]Measurement {
	out := map[string]Measurement{}
	for _, r := range rows {
		if cur, ok := out[r.Scenario]; !ok || r.Date > cur.Date {
			out[r.Scenario] = r
		}
	}
	return out
}

// Gate rule magnitudes, from the plan: the release floor per scenario, the
// regression band against the previous measurement, and the milestone
// average, which the caller passes (45 at M1, 60 at M2, 70 at M3).
const (
	gateFloor      = 0.60
	gateRegression = 5.0 / 100
)

// GateFailures is the bench-agent-verify gate: everything wrong with the
// series against the plan's rules, one sentence each; empty means pass.
//
// The rules, in the order they are reported: the series must be complete
// (every scenario measured on both sides), no scenario below the floor, the
// average at or above min, and no scenario regressed more than the band
// against its own previous measurement — where a previous measurement exists
// (both sides, on an earlier date). A first measurement has nothing to
// regress against, and the gate says so rather than inventing a delta.
func GateFailures(series Series, scenarios []string, min float64) []string {
	var fails []string
	savings := Savings(series.filter(SideTrilha), series.filter(SideBaseline))
	var missing []string
	for _, sc := range scenarios {
		if _, ok := savings[sc]; !ok {
			missing = append(missing, sc)
		}
	}
	if len(missing) > 0 {
		fails = append(fails, fmt.Sprintf("serie incompleta: sem economia para %s (faltam rodadas verdes dos dois lados)", strings.Join(missing, ", ")))
	}
	var below []string
	for sc, s := range savings {
		if s < gateFloor {
			below = append(below, fmt.Sprintf("%s (%.0f%%)", sc, s*100))
		}
	}
	if len(below) > 0 {
		sort.Strings(below)
		fails = append(fails, fmt.Sprintf("cenario(s) abaixo do piso de %.0f%%: %s", gateFloor*100, strings.Join(below, ", ")))
	}
	if len(savings) > 0 {
		total := 0.0
		for _, s := range savings {
			total += s
		}
		avg := total / float64(len(savings))
		if avg < min/100 {
			fails = append(fails, fmt.Sprintf("media de economia %.0f%% abaixo do minimo de %.0f%%", avg*100, min))
		}
	}
	for _, sc := range scenarios {
		before, ok := previousSavings(series, sc)
		if !ok {
			continue
		}
		now := savings[sc]
		if drop := before - now; drop > gateRegression {
			fails = append(fails, fmt.Sprintf("%s regrediu %.0f pontos (%.0f%% -> %.0f%%), banda de %.0f", sc, drop*100, before*100, now*100, gateRegression*100))
		}
	}
	return fails
}

// previousSavings derives the saving of a scenario at its second-latest pair
// of dates (both sides on the same date), which is what the latest is
// compared against.
func previousSavings(series Series, scenario string) (float64, bool) {
	dates := pairDates(series, scenario)
	if len(dates) < 2 {
		return 0, false
	}
	t, ok1 := rowOf(series, scenario, SideTrilha, dates[len(dates)-2])
	b, ok2 := rowOf(series, scenario, SideBaseline, dates[len(dates)-2])
	if !ok1 || !ok2 {
		return 0, false
	}
	tt, bb := t.TokensIn+t.TokensOut, b.TokensIn+b.TokensOut
	if tt == 0 || bb == 0 {
		return 0, false
	}
	return 1 - float64(tt)/float64(bb), true
}

// pairDates lists, ascending, the dates on which a scenario has rows on both
// sides.
func pairDates(series Series, scenario string) []string {
	seen := map[string]bool{}
	for _, r := range series.Measurements {
		if r.Scenario == scenario {
			seen[r.Date] = true
		}
	}
	var both []string
	for d := range seen {
		_, ok1 := rowOf(series, scenario, SideTrilha, d)
		_, ok2 := rowOf(series, scenario, SideBaseline, d)
		if ok1 && ok2 {
			both = append(both, d)
		}
	}
	sort.Strings(both)
	return both
}

func rowOf(series Series, scenario, side, date string) (Measurement, bool) {
	for _, r := range series.filter(side) {
		if r.Scenario == scenario && r.Date == date {
			return r, true
		}
	}
	return Measurement{}, false
}

func (s Series) filter(side string) []Measurement {
	var out []Measurement
	for _, r := range s.Measurements {
		if r.Side == side {
			out = append(out, r)
		}
	}
	return out
}

// measureRunner is how the measurement runs the agent; measureSeries takes it
// as a parameter so the tests can lie about the agent without lying about
// anything else.
type measureRunner func(ctx context.Context, dir, prompt string, o AgentOptions) (Usage, []byte, error)

// measureVerifier decides whether a side's work passed its gate: the trilha
// side through its scenario gate, the baseline through vet and test.
type measureVerifier func(ctx context.Context, dir string, sc Scenario, side string) bool

// measureSeries measures the savings series: both sides of every scenario
// with a baseline, runs times each, and writes the medians of the green runs
// into the series file. A run whose agent errored — authentication, network —
// leaves no row behind, exactly like the v1 ruler. repo is the read-only
// workspace copy the fixtures point at, bin the CLI the gates resolve
// `trilha` to.
func measureSeries(repo, bin string, run measureRunner, verify measureVerifier, seriesPath string, runs int, model, agent string, maxTurns int, timeout time.Duration, agentsMD bool) error {
	series, err := LoadSeries(seriesPath)
	if err != nil {
		return err
	}
	series.Agent = agentVersion(agent)
	series.Machine = machine()
	date := time.Now().UTC().Format("2006-01-02")
	work, err := os.MkdirTemp("", "trilha-measure-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	for _, sc := range SeriesScenarios() {
		turns := maxTurns
		if sc.MaxRounds > 0 {
			turns = sc.MaxRounds
		}
		for _, side := range []struct {
			name   string
			prompt string
			build  func(dir string) error
		}{
			{SideTrilha, sc.Prompt, func(dir string) error { return Build(repo, sc, dir, agentsMD) }},
			{SideBaseline, mustPrompt(sc.BaselinePromptMD), func(dir string) error { return BuildBaseline(repo, sc, dir) }},
		} {
			var greens []Measurement
			for n := 1; n <= runs; n++ {
				dir := filepath.Join(work, fmt.Sprintf("%s-%s-%d", sc.Name, side.name, n))
				if err := side.build(dir); err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				before := snapshotTree(dir)
				u, _, err := run(ctx, dir, side.prompt, AgentOptions{Model: model, MaxTurns: turns, Path: filepath.Dir(bin), Dirs: []string{repo}})
				if err != nil {
					cancel()
					return err
				}
				passed := u.Error == "" && verify(ctx, dir, sc, side.name)
				cancel()
				if !passed {
					fmt.Printf("%-12s %-8s run %d/%d: FAIL (sem linha na série)\n", sc.Name, side.name, n, runs)
					continue
				}
				greens = append(greens, Measurement{
					Date:        date,
					Model:       u.Model,
					Scenario:    sc.ID,
					Side:        side.name,
					TokensIn:    int64(u.Input + u.CacheRead),
					TokensOut:   int64(u.Output),
					Rounds:      u.Turns,
					FilesOpened: countWritten(dir, before),
					Runs:        1,
				})
				fmt.Printf("%-12s %-8s run %d/%d: PASS in=%d out=%d turns=%d\n", sc.Name, side.name, n, runs, u.Input+u.CacheRead, u.Output, u.Turns)
			}
			if m, ok := mergeGreens(date, greens); ok {
				m.Side = side.name
				if m.Model == "" {
					m.Model = series.Model
				}
				series.Measurements = append(rowsWithout(series, sc.ID, side.name, date), m)
			}
		}
	}
	if series.Trilha == "" {
		series.Trilha = version(repo)
	}
	return SaveSeries(seriesPath, series)
}

// mergeGreens folds the green runs of one (scenario, side) into the medians
// the row stores. Nothing without a green run produces a row.
func mergeGreens(date string, greens []Measurement) (Measurement, bool) {
	if len(greens) == 0 {
		return Measurement{}, false
	}
	col := func(f func(Measurement) int64) float64 {
		xs := make([]float64, 0, len(greens))
		for _, g := range greens {
			xs = append(xs, float64(f(g)))
		}
		return Median(xs)
	}
	models := make([]string, 0, len(greens))
	for _, g := range greens {
		models = append(models, g.Model)
	}
	sort.Strings(models)
	m := Measurement{
		Date:        date,
		Model:       models[len(models)/2],
		Scenario:    greens[0].Scenario,
		TokensIn:    int64(col(func(g Measurement) int64 { return g.TokensIn })),
		TokensOut:   int64(col(func(g Measurement) int64 { return g.TokensOut })),
		Rounds:      int(col(func(g Measurement) int64 { return int64(g.Rounds) })),
		FilesOpened: int(col(func(g Measurement) int64 { return int64(g.FilesOpened) })),
		Runs:        len(greens),
	}
	return m, true
}

// rowsWithout drops any previous row of the same (scenario, side, date), so
// re-measuring a day overwrites instead of duplicating.
func rowsWithout(s Series, scenario, side, date string) []Measurement {
	var out []Measurement
	for _, r := range s.Measurements {
		if r.Scenario == scenario && r.Side == side && r.Date == date {
			continue
		}
		out = append(out, r)
	}
	return out
}

// BuildBaseline copies a baseline fixture into dir: a plain copy, its module
// standing on its own.
func BuildBaseline(repo string, sc Scenario, dir string) error {
	return copyTree(filepath.Join(repo, filepath.FromSlash(sc.BaseDir)), dir, "", "")
}

// snapshotTree is the size and mtime of every file in dir, for countWritten.
func snapshotTree(dir string) map[string]fileStamp {
	out := map[string]fileStamp{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if info, err := d.Info(); err == nil {
			out[path] = fileStamp{info.Size(), info.ModTime()}
		}
		return nil
	})
	return out
}

type fileStamp struct {
	size int64
	mod  time.Time
}

// countWritten counts the files in dir that are new or changed since the
// snapshot: what the agent changed with its own hands. It compares stamps
// instead of asking "after the start?", because the kernel stamps a file with
// a coarse clock that can lag time.Now() — a fast write looked older than
// the run that made it.
func countWritten(dir string, before map[string]fileStamp) int {
	n := 0
	for path, now := range snapshotTree(dir) {
		if strings.HasSuffix(path, ".agent.json") {
			continue
		}
		if was, ok := before[path]; !ok || was.size != now.size || !was.mod.Equal(now.mod) {
			n++
		}
	}
	return n
}
