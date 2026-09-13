package ui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

var update = flag.Bool("update", false, "rewrite the chart golden files")

// golden compares the drawing with the file beside the test. A chart is read
// by looking at it, so the review of a change is the diff of the SVG.
func golden(t *testing.T, name string, n h.Node) {
	t.Helper()
	got := render(t, n)
	p := filepath.Join("testdata", name+".svg")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%v (run: go test ./ui -run TestChart -update)", err)
	}
	if got != strings.TrimRight(string(want), "\n") {
		t.Errorf("%s changed:\ngot  %s\nwant %s", name, got, want)
	}
}

var vendas = []Datum{{Label: "Contracts", Value: 412}, {Label: "Invoices", Value: 380}, {Label: "Reports", Value: 91}}

// SC-007, SC-008, SC-009, SC-012 — the same series always writes the same file.
func TestChartGoldens(t *testing.T) {
	golden(t, "bars", Bars(vendas, ChartTitle("Documents by type")))
	golden(t, "sparkline", SparklineTitle([]float64{3, 5, 4, 9, 7, 12}, SparkOpts{}, ChartTitle("Last six weeks")))
	golden(t, "donut", Donut([]Datum{{Label: "Done", Value: 70}, {Label: "Pending", Value: 25}, {Label: "Failed", Value: 5}}, ChartTitle("Status")))
}

// SC-006 — a Stat is text: no SVG, and the value arrives formatted.
func TestStatIsText(t *testing.T) {
	got := render(t, Stat("Documents", "1.204", StatHint("+38 this week")))
	for _, want := range []string{`class="ui-stat"`, `class="ui-stat-label">Documents<`, `class="ui-stat-value">1.204<`, `class="ui-stat-hint">+38 this week<`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "<svg") {
		t.Error("a Stat draws nothing")
	}
}

// SC-010 — a named chart is an image with a name; an unnamed one is decoration
// and the hidden table is what the reader reads. Either way the numbers are
// in the HTML.
func TestChartSaysTheNumbers(t *testing.T) {
	named := render(t, Bars(vendas, ChartTitle("Documents by type")))
	if !strings.Contains(named, `role="img"`) || !strings.Contains(named, "<title>Documents by type</title>") {
		t.Error(named)
	}
	if !strings.Contains(named, `<table class="ui-sr">`) || !strings.Contains(named, "<td>412</td>") {
		t.Error(named)
	}
	// A value a person reads is the app's, not the framework's: money and
	// percentage have no meaning the framework could guess.
	money := render(t, Bars([]Datum{{Label: "Rent", Value: 120050, Text: "R$ 1.200,50"}}))
	if !strings.Contains(money, ">R$ 1.200,50<") || strings.Contains(money, ">120050<") {
		t.Error(money)
	}

	plain := render(t, Bars(vendas))
	if !strings.Contains(plain, `aria-hidden="true"`) || strings.Contains(plain, `role="img"`) {
		t.Error(plain)
	}
	if !strings.Contains(plain, "<td>412</td>") {
		t.Error("the table is there even with no title")
	}
}

// SC-011 — the shapes that have no drawing still have to answer something.
func TestChartsSurviveADegenerateSeries(t *testing.T) {
	cases := []struct {
		name string
		node h.Node
	}{
		{"no bars", Bars(nil)},
		{"every bar zero", Bars([]Datum{{Label: "a"}, {Label: "b"}})},
		{"a negative value", Bars([]Datum{{Label: "a", Value: -5}, {Label: "b", Value: 10}})},
		{"no points", Sparkline(nil, SparkOpts{})},
		{"one point", Sparkline([]float64{7}, SparkOpts{})},
		{"a flat series", Sparkline([]float64{4, 4, 4}, SparkOpts{})},
		{"no slices", Donut(nil)},
		{"a total of zero", Donut([]Datum{{Label: "a"}, {Label: "b"}})},
		{"a negative slice", Donut([]Datum{{Label: "a", Value: -1}, {Label: "b", Value: 3}})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := render(t, c.node)
			if strings.Contains(got, "NaN") || strings.Contains(got, "+Inf") {
				t.Fatal(got)
			}
		})
	}
	// A bar nobody can draw keeps its label and its number.
	got := render(t, Bars([]Datum{{Label: "a", Value: -5}, {Label: "b", Value: 10}}))
	if !strings.Contains(got, ">a<") || !strings.Contains(got, "<td>-5</td>") {
		t.Error(got)
	}
	if !strings.Contains(got, `width="0"`) {
		t.Error("a negative value draws an empty bar: " + got)
	}
}

// #195 — the value of the tallest bar used to start past the viewBox, and
// what does not fit a viewBox is cut by the browser. The estimate below (7
// units a character) is the same one the drawing itself uses: there is no
// font metric on the server, only a width wide enough not to guess short.
func TestBarsReservaEspacoParaORotuloMaisLongo(t *testing.T) {
	longo := "5 folhas · 2 NC"
	got := render(t, Bars([]Datum{
		{Label: "2026-09", Value: 5, Text: longo},
		{Label: "2026-08", Value: 1, Text: "1 folha"},
	}))
	vbWidth := viewBoxWidth(t, got)
	x := valueTextX(t, got)
	if fim := x + float64(len([]rune(longo)))*7; fim > vbWidth {
		t.Fatalf("o rótulo termina em %.2f, o viewBox só tem %.2f: %s", fim, vbWidth, got)
	}
}

var viewBoxRe = regexp.MustCompile(`viewBox="0 0 ([0-9.]+) `)
var valueXRe = regexp.MustCompile(`class="ui-chart-value" x="([0-9.]+)"`)

func viewBoxWidth(t *testing.T, svg string) float64 {
	t.Helper()
	m := viewBoxRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("sem viewBox em %s", svg)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// valueTextX is the x of the first (and, since Bars draws widest first, the
// most exposed) value label.
func valueTextX(t *testing.T, svg string) float64 {
	t.Helper()
	m := valueXRe.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("sem <text class=\"ui-chart-value\"> em %s", svg)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// SC-013 — colour comes from the theme, so dark mode is not a second drawing.
func TestChartColoursComeFromTheTheme(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("assets", "ui.theme.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []h.Node{Bars(vendas), Donut(vendas), SparklineTitle([]float64{1, 2}, SparkOpts{})} {
		got := render(t, n)
		for _, bad := range []string{"#", "rgb(", "oklch("} {
			if strings.Contains(got, bad) {
				t.Errorf("literal colour %q in %s", bad, got)
			}
		}
	}
	for i := 1; i <= 5; i++ {
		if !strings.Contains(string(css), "--chart-"+string(rune('0'+i))+":") {
			t.Errorf("the theme does not declare --chart-%d", i)
		}
	}
}
