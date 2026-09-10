package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// APIUsageRoute is one route's share of the calls, as the screen shows it.
//
// It repeats what auth.UsageRoute holds instead of importing it, for the reason
// PolicyGrid takes an interface: the kit draws data, and an application with its
// own counters gets the same screen by filling these fields.
type APIUsageRoute struct {
	Method string
	Route  string
	Count  int
	Errors int
	Last   time.Time
}

// APIUsageDay is one day's calls.
type APIUsageDay struct {
	Day    time.Time
	Count  int
	Errors int
}

// APIUsageData is a usage report as the screen reads it.
type APIUsageData struct {
	Total  int
	Errors int
	Last   time.Time
	Days   []APIUsageDay
	Routes []APIUsageRoute
}

// APIUsageOpts configures the panel.
type APIUsageOpts struct {
	// Days names the window in the heading — "last 30 days". Zero writes no
	// window, which is honest when the caller did not bound the query.
	Days int
	// Empty replaces the sentence a key with no calls gets.
	Empty string
	// Limit is how many routes to list. Zero lists them all.
	Limit int
}

// APIUsage is the answer to "are they using this key? where? when did they
// stop?": the totals, the shape by day, and the table by route.
//
//	rel, _ := Chaves.Usage(c.Context(), auth.UsageQuery{Key: id, Since: trinta})
//	ui.APIUsage(c, ui.APIUsageData{…}, ui.APIUsageOpts{Days: 30})
//
// The chart is drawn on the server with ui.Sparkline — there is no JavaScript
// here, and the numbers are in the table underneath whether or not the drawing
// arrives.
func APIUsage(c *trilha.Ctx, u APIUsageData, o APIUsageOpts) h.Node {
	w := usageWords(c)
	if u.Total == 0 {
		empty := o.Empty
		if empty == "" {
			empty = w["empty"]
		}
		return Empty(EmptyOpts{Icon: "info", Title: empty, Hint: w["empty hint"]})
	}
	window := ""
	if o.Days > 0 {
		window = replaceN(w["window"], o.Days)
	}

	values := make([]float64, 0, len(u.Days))
	for _, d := range u.Days {
		values = append(values, float64(d.Count))
	}
	stats := []h.Node{
		Stat(w["calls"], strconv.Itoa(u.Total), StatHint(window)),
		Stat(w["errors"], strconv.Itoa(u.Errors), StatHint(percent(u.Errors, u.Total))),
	}
	if !u.Last.IsZero() {
		stats = append(stats, Card(CardContent(h.Div(h.Class("ui-stat"),
			h.Span(h.Class("ui-stat-label"), h.Text(w["last"])),
			h.Strong(Date(c, u.Last, Relative())),
		))))
	}

	routes := u.Routes
	if o.Limit > 0 && len(routes) > o.Limit {
		routes = routes[:o.Limit]
	}
	lines := make([]h.Node, 0, len(routes))
	for _, r := range routes {
		attrs := []h.Node{}
		if r.Errors > 0 && r.Count > 0 && r.Errors*20 > r.Count {
			// More than one call in twenty failing is the thing somebody is on
			// this screen to find.
			attrs = append(attrs, h.Class("ui-late"))
		}
		lines = append(lines, h.Tr(append(attrs,
			h.Td(h.Text(r.Method)),
			h.Td(Code(r.Route)),
			h.Td(Num(), h.Text(strconv.Itoa(r.Count))),
			h.Td(Num(), h.Text(strconv.Itoa(r.Errors))),
			h.Td(Date(c, r.Last, Relative())),
		)...))
	}

	return Stack(
		Grid(h.Group(stats...)),
		SparklineTitle(values, SparkOpts{}, ChartTitle(w["chart"])),
		Table(
			h.Thead(h.Tr(
				h.Th(h.Text(w["method"])), h.Th(h.Text(w["route"])),
				h.Th(h.Text(w["calls"])), h.Th(h.Text(w["errors"])), h.Th(h.Text(w["last"])),
			)),
			h.Tbody(lines...),
		),
	)
}

// percent is "3 % of 1204", or nothing when there is nothing to divide.
func percent(part, total int) string {
	if total <= 0 {
		return ""
	}
	return strconv.Itoa(part*100/total) + " %"
}

func replaceN(s string, n int) string {
	return strings.Replace(s, "{n}", strconv.Itoa(n), 1)
}

func usageWords(c *trilha.Ctx) map[string]string {
	if langOf(c) == "pt-BR" {
		return map[string]string{
			"calls": "Chamadas", "errors": "Erros", "last": "Última",
			"method": "Método", "route": "Rota", "window": "últimos {n} dias",
			"chart": "Chamadas por dia",
			"empty": "Esta chave nunca foi usada.",
			"empty hint": "Ou o parceiro ainda não integrou, ou está usando outra chave. " +
				"As duas coisas valem uma pergunta.",
		}
	}
	return map[string]string{
		"calls": "Calls", "errors": "Errors", "last": "Last",
		"method": "Method", "route": "Route", "window": "last {n} days",
		"chart": "Calls per day",
		"empty": "This key has never been used.",
		"empty hint": "Either the partner has not integrated yet, or they are using another key. " +
			"Both are worth one question.",
	}
}
