package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// InboxRow is one request of an approval queue, as the screen shows it.
//
// It is a row and not approval.Record because the kit draws data, not another
// package's types: an application with its own queue gets the same screen by
// filling these fields.
type InboxRow struct {
	ID      string
	Kind    string
	Subject string
	// Target is where the thing being decided lives. Empty draws no link, and
	// a queue whose rows link nowhere is a queue people decide blind.
	Target string
	State  string
	// Due is the deadline; zero means none. Late is what the screen shouts
	// about, and it is the caller's answer because only it knows what "now"
	// means in a test.
	Due  interface{ IsZero() bool }
	Late bool
	// Reason and By are the decision, for the rows that have one.
	Reason string
	By     string
}

// InboxOpts configures the inbox.
type InboxOpts struct {
	// Decide is where the two buttons post. Required for a queue somebody can
	// act on; empty draws the list read-only, which is what a decided tab is.
	Decide string
	// CSRF is the token field. A form that changes a decision carries one.
	CSRF h.Node
	// Empty is what an empty queue says. Default: nothing to decide.
	Empty string
	// Reason names the justification field (default "reason"). It is a field
	// and not an option: a decision nobody explained is a decision somebody
	// argues about later.
	Reason string
}

// Inbox renders the queue of things waiting for a person: what it is, when it
// is due, and the two buttons.
//
//	ui.Inbox(c, rows, ui.InboxOpts{Decide: "/admin/tarefas", CSRF: trilha.CSRFInput(c)})
//
// There is no JavaScript: it is a table and two forms. The deadline is written
// by ui.Relative, so "in 3 days" and "2 days ago" are the same sentence in the
// reader's language, and a late row carries ui-late so the theme can shout.
func Inbox(c *trilha.Ctx, rows []InboxRow, o InboxOpts) h.Node {
	if len(rows) == 0 {
		empty := o.Empty
		if empty == "" {
			empty = inboxWords(c)["empty"]
		}
		return Empty(EmptyOpts{Icon: "check", Title: empty})
	}
	w := inboxWords(c)
	reason := o.Reason
	if reason == "" {
		reason = "reason"
	}
	lines := make([]h.Node, 0, len(rows))
	for _, r := range rows {
		cells := []h.Node{
			h.Td(subjectCell(r)),
			h.Td(h.Text(r.Kind)),
			h.Td(Status(approvalStates, r.State)),
			h.Td(dueCell(c, r, w)),
		}
		if o.Decide != "" && r.State == "pending" {
			cells = append(cells, h.Td(decideForm(r, o, reason, w)))
		} else {
			cells = append(cells, h.Td(decidedCell(r)))
		}
		attrs := []h.Node{}
		if r.Late {
			attrs = append(attrs, h.Class("ui-late"))
		}
		lines = append(lines, h.Tr(append(attrs, cells...)...))
	}
	return Table(
		h.Thead(h.Tr(
			h.Th(h.Text(w["subject"])), h.Th(h.Text(w["kind"])),
			h.Th(h.Text(w["state"])), h.Th(h.Text(w["due"])), h.Th(h.Text("")),
		)),
		h.Tbody(lines...),
	)
}

// InboxBadge is the number for a menu item: how many are waiting for whoever
// is reading. Zero draws nothing, because a badge showing zero is a badge that
// teaches people to ignore badges.
func InboxBadge(n int) h.Node {
	if n <= 0 {
		return h.Fragment()
	}
	return Badge(h.Class("ui-inbox-badge"), h.Text(strconv.Itoa(n)))
}

func subjectCell(r InboxRow) h.Node {
	if r.Target == "" {
		return h.Text(r.Subject)
	}
	return h.A(h.Href(r.Target), h.Text(r.Subject))
}

func dueCell(c *trilha.Ctx, r InboxRow, w map[string]string) h.Node {
	if r.Due == nil || r.Due.IsZero() {
		return Muted(h.Text(w["no due"]))
	}
	return Date(c, r.Due, Relative())
}

func decidedCell(r InboxRow) h.Node {
	if r.By == "" && r.Reason == "" {
		return h.Fragment()
	}
	return Muted(h.Textf("%s %s", r.By, r.Reason))
}

// decideForm is the two buttons and the justification, in one form: the
// decision and the reason travel together, because a reason typed into a field
// that a second click discards is a reason nobody wrote.
func decideForm(r InboxRow, o InboxOpts, reason string, w map[string]string) h.Node {
	return h.Form(h.Method("post"), h.Action(o.Decide), h.Class("ui-inline-form"),
		o.CSRF,
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(r.ID)),
		Input(h.Name(reason), h.Placeholder(w["reason"]), h.Aria("label", w["reason"])),
		Button(Sm(), h.Type("submit"), h.Name("decision"), h.Value("approved"),
			h.Text(w["approve"]), Confirm(w["approve"]+"?", w["approve hint"])),
		Button(Outline(), Sm(), h.Type("submit"), h.Name("decision"), h.Value("rejected"),
			h.Text(w["reject"]), Confirm(w["reject"]+"?", w["reject hint"])),
	)
}

// approvalStates is the list the badge colours by. It repeats the approval
// package's values instead of importing it: the kit must not drag a queue into
// an application that only draws buttons — the same reason PolicyGrid takes an
// interface instead of auth.Policy.
var approvalStates = trilha.Enum{
	{Value: "pending", Label: "Pending", Tone: "info"},
	{Value: "approved", Label: "Approved", Tone: "success"},
	{Value: "rejected", Label: "Rejected", Tone: "danger"},
	{Value: "withdrawn", Label: "Withdrawn", Tone: "muted"},
	{Value: "expired", Label: "Expired", Tone: "warning"},
}

func inboxWords(c *trilha.Ctx) map[string]string {
	if langOf(c) == "pt-BR" {
		return map[string]string{
			"subject": "Assunto", "kind": "Tipo", "state": "Estado", "due": "Prazo",
			"empty": "Nada esperando por você.", "no due": "sem prazo",
			"reason": "Motivo", "approve": "Aprovar", "reject": "Rejeitar",
			"approve hint": "A decisão fica registrada com o seu nome.",
			"reject hint":  "A decisão fica registrada com o seu nome.",
		}
	}
	return map[string]string{
		"subject": "Subject", "kind": "Kind", "state": "State", "due": "Due",
		"empty": "Nothing waiting for you.", "no due": "no deadline",
		"reason": "Reason", "approve": "Approve", "reject": "Reject",
		"approve hint": "The decision is recorded with your name on it.",
		"reject hint":  "The decision is recorded with your name on it.",
	}
}
