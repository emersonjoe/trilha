package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// DeadlineCards is the top of a deadline panel: what is late, and how much is
// coming in each horizon.
//
//	resumo := trilha.Deadlines(itens, trilha.DeadlineOpts{Now: time.Now().In(c.Location())})
//	ui.DeadlineCards(c, resumo)
//
// The order is the order of the horizons, so the cards do not move between
// reloads. The overdue card only shouts when there is something to shout about:
// a red zero teaches people to ignore the colour.
func DeadlineCards(c *trilha.Ctx, s trilha.DeadlineSummary) h.Node {
	w := deadlineWords(c)
	cards := []h.Node{deadlineCard(w["overdue"], s.Overdue, len(s.Overdue) > 0)}
	for _, days := range s.Horizons {
		items := s.Within[days]
		label := strings.Replace(w["in days"], "{n}", strconv.Itoa(days), 1)
		cards = append(cards, deadlineCard(label, items, false))
	}
	if s.Next != nil {
		cards = append(cards, Card(h.Class("ui-deadline-card"),
			CardContent(h.Div(h.Class("ui-stat"),
				h.Span(h.Class("ui-stat-label"), h.Text(w["next"])),
				h.Strong(h.Class("ui-deadline-next"), deadlineTitle(*s.Next)),
				h.Span(h.Class("ui-stat-hint"), Date(c, s.Next.Due, Relative(), DateOnly())),
			))))
	}
	return Grid(h.Class("ui-deadline-cards"), h.Group(cards...))
}

// deadlineCard is one bucket: the count, and the nearest title under it — a
// number with no example beside it is a number people have to click to
// understand.
func deadlineCard(label string, items []trilha.Deadline, alarm bool) h.Node {
	attrs := []h.Node{h.Class("ui-deadline-card")}
	if alarm {
		attrs = append(attrs, h.Class("ui-deadline-alarm"))
	}
	kids := []h.Node{Stat(label, strconv.Itoa(len(items)))}
	if len(items) > 0 {
		kids = append(kids, Muted(h.Text(items[0].Title)))
	}
	return Card(append(attrs, CardContent(kids...))...)
}

// DeadlineListOpts configures the list.
type DeadlineListOpts struct {
	// Limit is how many rows to draw. Zero draws them all; a panel is a
	// summary, and a list of four hundred rows on a dashboard is a list nobody
	// reads.
	Limit int
	// More is where "and 12 more" links to. Empty writes the line without a
	// link, and only when Limit actually cut something off.
	More string
	// Empty is what a list with nothing in it says.
	Empty string
	// Now is what "late" is measured against. Zero means time.Now(), and a
	// screenshot test that cannot fix the clock is a screenshot test that fails
	// on its own the next morning.
	Now time.Time
	// Owner draws the column of who it is on. Off by default, because most
	// applications do not track it and an empty column is a lie about the data.
	Owner bool
}

// DeadlineList is the list under the cards: what is due, when, and where it
// lives.
//
//	ui.DeadlineList(c, itens, ui.DeadlineListOpts{Limit: 10})
//
// The dates are written by ui.Date with Relative, so "in 3 days" and "2 days
// ago" are one sentence in the reader's language. A row whose day has ended
// carries ui-late, which is the same class the inbox uses: late looks the same
// everywhere in an application or it looks like a bug.
//
// The list is drawn in the order it is given. trilha.Deadlines already answers
// with the nearest first, which is the order a person reads such a list in.
func DeadlineList(c *trilha.Ctx, items []trilha.Deadline, o DeadlineListOpts) h.Node {
	w := deadlineWords(c)
	open := make([]trilha.Deadline, 0, len(items))
	for _, it := range items {
		if !it.Done {
			open = append(open, it)
		}
	}
	if len(open) == 0 {
		empty := o.Empty
		if empty == "" {
			empty = w["empty"]
		}
		return Empty(EmptyOpts{Icon: "check", Title: empty})
	}

	shown, rest := open, 0
	if o.Limit > 0 && len(open) > o.Limit {
		shown, rest = open[:o.Limit], len(open)-o.Limit
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	lines := make([]h.Node, 0, len(shown))
	for _, it := range shown {
		cells := []h.Node{
			h.Td(deadlineTitle(it)),
			h.Td(h.Text(it.Kind)),
			h.Td(Date(c, it.Due, Relative(), DateOnly())),
		}
		if o.Owner {
			cells = append(cells, h.Td(h.Text(it.Owner)))
		}
		row := cells
		if it.Late(now, zoneOf(c)) {
			row = append([]h.Node{h.Class("ui-late")}, cells...)
		}
		lines = append(lines, h.Tr(row...))
	}

	heads := []h.Node{h.Th(h.Text(w["what"])), h.Th(h.Text(w["kind"])), h.Th(h.Text(w["due"]))}
	if o.Owner {
		heads = append(heads, h.Th(h.Text(w["owner"])))
	}
	table := Table(h.Thead(h.Tr(heads...)), h.Tbody(lines...))
	if rest == 0 {
		return table
	}
	more := strings.Replace(w["and more"], "{n}", strconv.Itoa(rest), 1)
	if o.More == "" {
		return h.Div(table, Muted(h.Text(more)))
	}
	return h.Div(table, h.P(h.A(h.Href(o.More), h.Text(more))))
}

// DeadlineBadge is the number beside a menu item: how many are late.
//
//	ui.DeadlineBadge(c, resumo.Overdue)
//
// An empty list draws nothing. A badge that shows zero is a badge people learn
// to look past, and then the one that says 3 is looked past too.
func DeadlineBadge(c *trilha.Ctx, items []trilha.Deadline) h.Node {
	if len(items) == 0 {
		return h.Fragment()
	}
	w := deadlineWords(c)
	label := strings.Replace(w["overdue n"], "{n}", strconv.Itoa(len(items)), 1)
	return Badge(h.Class("ui-badge-danger"), h.Class("ui-deadline-badge"),
		h.Aria("label", label), h.Text(strconv.Itoa(len(items))))
}

func deadlineTitle(it trilha.Deadline) h.Node {
	if it.URL == "" {
		return h.Text(it.Title)
	}
	return h.A(h.Href(it.URL), h.Text(it.Title))
}

func deadlineWords(c *trilha.Ctx) map[string]string {
	if langOf(c) == "pt-BR" {
		return map[string]string{
			"overdue": "Vencidos", "in days": "Em {n} dias", "next": "Próximo",
			"what": "O quê", "kind": "Tipo", "due": "Prazo", "owner": "Responsável",
			"empty": "Nenhum prazo aberto.", "and more": "e mais {n}",
			"overdue n": "{n} vencidos",
		}
	}
	return map[string]string{
		"overdue": "Overdue", "in days": "In {n} days", "next": "Next",
		"what": "What", "kind": "Kind", "due": "Due", "owner": "Owner",
		"empty": "No open deadline.", "and more": "and {n} more",
		"overdue n": "{n} overdue",
	}
}
