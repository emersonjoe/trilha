package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/site/internal/custos"
)

// TestCustosEmbedsTheSeries is the drift alarm: the copy of the series the
// page draws from is exactly the file the ruler wrote. A measurement that
// lands in bench/agent and never makes it here is a page about a ruler that
// no longer exists.
func TestCustosEmbedsTheSeries(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "bench", "agent", "results", "results.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != string(custos.Raw()) {
		t.Fatal("site/internal/custos/results.json drifted from bench/agent/results/results.json; copy the series again")
	}
}

// TestCustosPageMatchesResults renders both locales and checks every number
// they show against an independent reading of the series file — the same
// character-by-character honesty the cookbook snippets get. It also pins the
// empty state: today's series is empty, and the page must say so instead of
// inventing a number.
func TestCustosPageMatchesResults(t *testing.T) {
	var src struct {
		Measurements []struct {
			Date      string `json:"date"`
			Scenario  string `json:"scenario"`
			Side      string `json:"side"`
			TokensIn  int64  `json:"tokens_in"`
			TokensOut int64  `json:"tokens_out"`
			Rounds    int    `json:"rounds"`
		} `json:"measurements"`
	}
	if err := json.Unmarshal(custos.Raw(), &src); err != nil {
		t.Fatal(err)
	}

	// expected recomputes, by hand, what the page is allowed to show: the
	// latest date per side of each scenario, the saving from those rows.
	latest := map[string]map[string]*struct {
		date           string
		tokens, rounds int64
	}{}
	for i := range src.Measurements {
		m := &src.Measurements[i]
		sides, ok := latest[m.Scenario]
		if !ok {
			sides = map[string]*struct {
				date           string
				tokens, rounds int64
			}{}
			latest[m.Scenario] = sides
		}
		cur := sides[m.Side]
		if cur == nil || m.Date >= cur.date {
			sides[m.Side] = &struct {
				date           string
				tokens, rounds int64
			}{m.Date, m.TokensIn + m.TokensOut, int64(m.Rounds)}
		}
	}

	for _, path := range []string{"/custos", "/pt/custos"} {
		code, body := get(t, path)
		if code != 200 {
			t.Fatalf("%s = %d", path, code)
		}
		numeros := numerosDe(body)
		if len(src.Measurements) == 0 {
			if strings.Contains(body, "custos-numero") && !strings.Contains(body, ">0%<") {
				t.Fatalf("%s shows a hero number with an empty series", path)
			}
			if !strings.Contains(body, "bench-agent-measure") {
				t.Fatalf("%s: the empty state must say how the first measurement happens", path)
			}
			continue
		}
		// Every token count of the latest date on both sides must be on the
		// page, in the site's own rendering ("12.3k"), and the average too.
		for sc, sides := range latest {
			for side, row := range sides {
				_ = side
				want := formataTokens(row.tokens)
				if !numeros[want] {
					t.Errorf("%s: %s (%s) does not show %s", path, sc, side, want)
				}
			}
		}
		media := 0.0
		n := 0
		for sc, sides := range latest {
			tk, bk := sides["trilha"], sides["baseline"]
			if tk == nil || bk == nil || tk.tokens == 0 || bk.tokens == 0 {
				t.Errorf("%s: %s incomplete on the latest date", path, sc)
				continue
			}
			media += 1 - float64(tk.tokens)/float64(bk.tokens)
			n++
		}
		if n > 0 {
			media /= float64(n)
			if !numeros[formataPct(media)] {
				t.Errorf("%s: the hero does not show the average %s", path, formataPct(media))
			}
		}
	}
}

// numerosDe collects the numbers a body prints, in the two shapes the page
// uses: token counts ("1.2k") and percentages ("73%").
func numerosDe(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range regexp.MustCompile(`\d+(?:\.\d+)?k?%`).FindAllString(body, -1) {
		out[m] = true
	}
	for _, m := range regexp.MustCompile(`>\s*(\d+(?:\.\d+)?k?)\s*<`).FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
}

// formataTokens mirrors the page's token rendering (site/internal/custos).
func formataTokens(n int64) string {
	if n >= 1000 {
		return strconv.FormatFloat(float64(n)/1000, 'f', 1, 64) + "k"
	}
	return strconv.FormatInt(n, 10)
}

// formataPct mirrors the page's percentage rendering.
func formataPct(v float64) string {
	p := v * 100
	if p == float64(int(p)) {
		return strconv.Itoa(int(p)) + "%"
	}
	return strconv.FormatFloat(p, 'f', 1, 64) + "%"
}
