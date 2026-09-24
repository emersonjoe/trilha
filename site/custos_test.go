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
	"github.com/emersonjoe/trilha/site/internal/docs"
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

// TestLLMsTxtPerRecipe is the spec 160 contract: every cookbook slug of both
// locales has a recipe llms.txt that answers the priced pack — each page
// with its estimated cost and the total — followed by the recipe's page
// verbatim. A slug without its file is a pack the agent cannot buy.
func TestLLMsTxtPerRecipe(t *testing.T) {
	for _, l := range docs.Locales {
		section := "cookbook"
		base := "/llms/recipes/"
		if l.Code == "pt" {
			section = "receitas"
			base = "/pt/llms/receitas/"
		}
		for _, slug := range docs.RecipeSlugs(l.Code) {
			code, body := get(t, base+slug+".txt")
			if code != 200 {
				t.Errorf("%s: %s = %d", l.Code, base+slug+".txt", code)
				continue
			}
			if !strings.Contains(body, "tokens est. in all") || !strings.Contains(body, "tokens est.\n") {
				t.Errorf("%s/%s: the pack is not priced:\n%.400s", l.Code, slug, body)
			}
			p, ok := docs.Get(l.Code, section, slug)
			if !ok {
				t.Fatalf("no page for %s/%s", l.Code, slug)
			}
			// The recipe's own page comes whole: its first heading is on the
			// file, and so is a cost line naming it.
			if !strings.Contains(body, "["+p.Title+"]") {
				t.Errorf("%s/%s: the file does not list the recipe's page", l.Code, slug)
			}
			if cost, ok := docs.PackCost(l.Code, slug); !ok || cost <= 0 {
				t.Errorf("%s/%s: PackCost = %d, %v", l.Code, slug, cost, ok)
			}
		}
		// A slug nobody wrote is a 404, not an empty file.
		code, _ := get(t, base+"nao-existe.txt")
		if code != 404 {
			t.Errorf("%s: unknown slug = %d, want 404", l.Code, code)
		}
	}
}

// TestRecipePageShowsThePackBadge renders a cookbook page and checks the
// price tag: the mono badge with the pack's estimated cost, in both locales.
func TestRecipePageShowsThePackBadge(t *testing.T) {
	for _, path := range []string{"/cookbook/database", "/pt/receitas/banco-de-dados"} {
		code, body := get(t, path)
		if code != 200 {
			t.Fatalf("%s = %d", path, code)
		}
		if !strings.Contains(body, "custo-pack") || !strings.Contains(body, "ctx --pack") {
			t.Fatalf("%s: no pack badge:\n%.600s", path, body)
		}
		if !strings.Contains(body, "tokens est.") {
			t.Fatalf("%s: the badge does not say est.:", path)
		}
	}
	// A non-recipe page carries no badge.
	if _, body := get(t, "/learn/pages-and-routes"); strings.Contains(body, "custo-pack") {
		t.Fatal("a learn page grew a pack badge")
	}
}
