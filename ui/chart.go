package ui

import (
	"io"
	"math"
	"strconv"

	"github.com/emersonjoe/trilha/h"
)

// Datum is one labelled value of a chart. The label is what a person reads;
// the value is only used to work out a proportion, so the app is free to keep
// counting in whatever unit it counts.
type Datum struct {
	Label string
	Value float64
	// Text is the value as a person reads it — "R$ 1.204,50", "38 %", "1.2k".
	// Empty prints the number as it is: the framework has no locale, and
	// formatting money is the app's job, here as in Stat.
	Text string
}

// text is what the drawing writes beside the bar and the table repeats.
func (d Datum) text() string {
	if d.Text != "" {
		return d.Text
	}
	return num(d.Value)
}

// Stat is the counter of a dashboard: a label, a value and an optional hint.
// The value arrives already formatted — the framework has no locale, and the
// app is the one that knows whether a thousand is 1,204 or 1.204.
//
//	ui.Stat("Documents", "1.204", ui.StatHint("+38 this week"))
func Stat(label, value string, children ...h.Node) h.Node {
	n := []h.Node{h.Class("ui-stat"),
		h.Span(h.Class("ui-stat-label"), h.Text(label)),
		h.Strong(h.Class("ui-stat-value"), h.Text(value)),
	}
	return h.Div(append(n, children...)...)
}

// StatHint is the second line of a Stat: a variation, a period, a total.
func StatHint(s string, attrs ...h.Node) h.Node {
	return h.Span(append([]h.Node{h.Class("ui-stat-hint"), h.Text(s)}, attrs...)...)
}

// ChartTitle names a chart for whoever cannot see it. With a title the drawing
// is an image with a name (role="img" plus <title>); without one the drawing is
// decoration (aria-hidden) and the table beside it — hidden from the eye, not
// from the reader — is what says the numbers.
func ChartTitle(s string) h.Node { return chartTitle(s) }

type chartTitle string

func (chartTitle) Render(io.Writer) error { return nil }

func splitTitle(attrs []h.Node) (string, []h.Node) {
	var title string
	rest := make([]h.Node, 0, len(attrs))
	for _, a := range attrs {
		if t, ok := a.(chartTitle); ok {
			title = string(t)
			continue
		}
		rest = append(rest, a)
	}
	return title, rest
}

// frame wraps a drawing and the table that repeats it in text.
func frame(title string, svg h.Node, data []Datum, extra ...h.Node) h.Node {
	n := []h.Node{h.Class("ui-chart")}
	if svg != nil {
		n = append(n, svg)
	}
	n = append(n, extra...)
	return h.Div(append(n, chartData(title, data))...)
}

// chartData is the same numbers as a table, hidden from the eye. It is what a
// screen reader reads, and what comes out of a printer that drops the SVG.
func chartData(title string, data []Datum) h.Node {
	n := []h.Node{h.Class("ui-sr")}
	if title != "" {
		n = append(n, h.Caption(h.Text(title)))
	}
	rows := make([]h.Node, 0, len(data))
	for _, d := range data {
		rows = append(rows, h.Tr(h.Th(h.Attr("scope", "row"), h.Text(d.Label)), h.Td(h.Text(d.text()))))
	}
	return h.Table(append(n, h.Tbody(rows...))...)
}

// svgHead is the shared opening of every chart: named and visible to a reader,
// or unnamed and out of its way.
func svgHead(title, viewBox string, attrs []h.Node) []h.Node {
	n := []h.Node{h.Class("ui-chart-svg"), h.Attr("viewBox", viewBox), h.Attr("preserveAspectRatio", "xMidYMid meet")}
	if title != "" {
		n = append(n, h.Role("img"), h.Title(h.Text(title)))
	} else {
		n = append(n, h.Aria("hidden", "true"))
	}
	return append(n, attrs...)
}

// num prints a number the shortest way that reads it back, so a golden file
// does not depend on how many zeros a float carries.
func num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// chartColor cycles the theme's five chart variables, so a sixth slice is the
// first colour again instead of a colour nobody declared.
func chartColor(i int) string { return "var(--chart-" + strconv.Itoa(i%5+1) + ")" }

// Bars draws one horizontal bar per Datum, proportional to the largest value,
// with the label to the left and the value at the end of the bar. It is the
// `<div style="width:37%">` of every dashboard, with the numbers still readable
// when the CSS does not arrive.
//
// A negative value draws no bar and keeps its label; a series whose largest
// value is zero or less draws every bar empty.
func Bars(data []Datum, attrs ...h.Node) h.Node {
	title, rest := splitTitle(attrs)
	if len(data) == 0 {
		return frame(title, nil, data)
	}
	const (
		width  = 320.0
		labelW = 96.0
		barX   = 104.0
		barMax = 176.0
		rowH   = 22.0
		barH   = 12.0
	)
	max := 0.0
	for _, d := range data {
		if d.Value > max {
			max = d.Value
		}
	}
	body := make([]h.Node, 0, len(data)*3)
	for i, d := range data {
		y := float64(i) * rowH
		w := 0.0
		if max > 0 && d.Value > 0 {
			w = d.Value / max * barMax
		}
		body = append(body,
			h.El("text", h.Class("ui-chart-label"), h.Attr("x", num(labelW)), h.Attr("y", num(y+15)), h.Attr("text-anchor", "end"), h.Text(d.Label)),
			h.El("rect", h.Attr("x", num(barX)), h.Attr("y", num(y+5)), h.Attr("width", num(w)), h.Attr("height", num(barH)),
				h.Attr("rx", "3"), h.Attr("fill", chartColor(i)), h.Title(h.Text(d.Label+": "+d.text()))),
			h.El("text", h.Class("ui-chart-value"), h.Attr("x", num(barX+w+6)), h.Attr("y", num(y+15)), h.Text(d.text())),
		)
	}
	vb := "0 0 " + num(width) + " " + num(float64(len(data))*rowH)
	return frame(title, h.Svg(append(svgHead(title, vb, rest), body...)...), data)
}

// SparkOpts is the size of a Sparkline in the same units its points are drawn
// in; zero means 120 by 32.
type SparkOpts struct {
	Width  int
	Height int
}

// Sparkline draws the series as one line, with no axis and no scale — the
// shape of the last few weeks inside a Stat, not a chart to read values off.
//
// One point alone, or a series where every value is the same, is a straight
// line through the middle; an empty series draws nothing.
func Sparkline(values []float64, o SparkOpts) h.Node {
	return sparkline(values, o, nil)
}

// SparklineTitle is Sparkline with a name and the numbers in a hidden table,
// for a line that stands on its own instead of inside a Stat.
func SparklineTitle(values []float64, o SparkOpts, attrs ...h.Node) h.Node {
	return sparkline(values, o, attrs)
}

func sparkline(values []float64, o SparkOpts, attrs []h.Node) h.Node {
	title, rest := splitTitle(attrs)
	data := make([]Datum, 0, len(values))
	for i, v := range values {
		data = append(data, Datum{Label: strconv.Itoa(i + 1), Value: v})
	}
	if len(values) == 0 {
		return frame(title, nil, data)
	}
	w, hgt := float64(o.Width), float64(o.Height)
	if w <= 0 {
		w = 120
	}
	if hgt <= 0 {
		hgt = 32
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	pts := ""
	for i, v := range values {
		x := 1.0
		if len(values) > 1 {
			x = 1 + float64(i)*(w-2)/float64(len(values)-1)
		}
		y := hgt / 2
		if max > min {
			y = hgt - 1 - (v-min)/(max-min)*(hgt-2)
		}
		if i > 0 {
			pts += " "
		}
		pts += num(x) + "," + num(y)
	}
	if len(values) == 1 {
		pts += " " + num(w-1) + "," + num(hgt/2)
	}
	line := h.El("polyline", h.Attr("points", pts), h.Attr("fill", "none"), h.Attr("stroke", chartColor(0)),
		h.Attr("stroke-width", "2"), h.Attr("stroke-linecap", "round"), h.Attr("stroke-linejoin", "round"))
	vb := "0 0 " + num(w) + " " + num(hgt)
	svg := h.Svg(append(svgHead(title, vb, rest), h.Class("ui-spark"), line)...)
	if title == "" && len(attrs) == 0 {
		return svg
	}
	return frame(title, svg, data)
}

// Donut draws the share of each Datum in the total as a ring, with the legend
// as a list beside the drawing and not inside the SVG — a legend is text, and
// text inside an SVG neither wraps nor is selected.
//
// The ring is drawn with a dashed circle instead of arcs: no trigonometry, so
// the same series always writes the same file. Negative values are ignored and
// a total of zero draws the empty track.
func Donut(data []Datum, attrs ...h.Node) h.Node {
	title, rest := splitTitle(attrs)
	if len(data) == 0 {
		return frame(title, nil, data)
	}
	const (
		cx = 21.0
		cy = 21.0
		r  = 15.9155 // circumference 100: a dash is a percentage
		sw = 4.0
	)
	total := 0.0
	for _, d := range data {
		if d.Value > 0 {
			total += d.Value
		}
	}
	ring := []h.Node{h.Attr("transform", "rotate(-90 "+num(cx)+" "+num(cy)+")")}
	ring = append(ring, h.El("circle", h.Class("ui-donut-track"), h.Attr("cx", num(cx)), h.Attr("cy", num(cy)), h.Attr("r", num(r)),
		h.Attr("fill", "none"), h.Attr("stroke-width", num(sw))))
	acc := 0.0
	legend := make([]h.Node, 0, len(data))
	for i, d := range data {
		pct := 0.0
		if total > 0 && d.Value > 0 {
			pct = d.Value / total * 100
		}
		if pct > 0 {
			ring = append(ring, h.El("circle", h.Attr("cx", num(cx)), h.Attr("cy", num(cy)), h.Attr("r", num(r)),
				h.Attr("fill", "none"), h.Attr("stroke", chartColor(i)), h.Attr("stroke-width", num(sw)),
				h.Attr("stroke-dasharray", num(pct)+" "+num(100-pct)), h.Attr("stroke-dashoffset", num(100-acc)),
				h.Title(h.Text(d.Label+": "+d.text()))))
			acc += pct
		}
		legend = append(legend, h.Li(
			h.Span(h.Class("ui-swatch"), h.StyleAttr("background:"+chartColor(i)), h.Aria("hidden", "true")),
			h.Text(d.Label), h.B(h.Text(d.text()))))
	}
	svg := h.Svg(append(svgHead(title, "0 0 42 42", rest), h.Class("ui-donut"), h.El("g", ring...))...)
	return frame(title, svg, data, h.Ul(append([]h.Node{h.Class("ui-chart-legend")}, legend...)...))
}
