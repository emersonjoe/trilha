// Package approvalinbox is the approval-inbox pattern: what waits for whoever
// is reading, and the two buttons. Who may decide is approval's answer, not
// this screen's.
package approvalinbox

import (
	"errors"
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Waiting is what the inbox shows: the queue Setup provided (trilha add
// approvals), filtered to what this person may decide.
var Waiting = func(c *trilha.Ctx) ([]approval.Record, error) {
	return trilha.Use[*approval.Approvals](c).Inbox(c, approval.ListParams{})
}

// Page renders the inbox.
func Page(c *trilha.Ctx) (h.Node, error) {
	records, err := Waiting(c)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rows := make([]ui.InboxRow, 0, len(records))
	for _, r := range records {
		rows = append(rows, ui.InboxRow{ID: r.ID, Kind: r.Kind, Subject: r.Subject, Target: r.Target,
			State: r.State, Due: r.Due, Late: r.Late(now), By: r.By, Reason: r.Reason})
	}
	return ui.Stack(ui.PageHeader("Approvals"), ui.Inbox(c, rows, ui.InboxOpts{
		Decide: c.Request().URL.Path, CSRF: trilha.CSRFInput(c), Empty: "Nothing is waiting for you.",
	})), nil
}

// POST decides. The decision and the reason travel in the same form, and the
// package writes who decided to the audit trail.
func POST(c *trilha.Ctx) error {
	err := trilha.Use[*approval.Approvals](c).Decide(c, c.Form("id"), c.Form("decision"), c.Form("reason"))
	if errors.Is(err, approval.ErrNotYours) {
		return trilha.Errorf(http.StatusForbidden, "this decision is not yours")
	}
	if err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "Decided.")
	return c.Redirect(c.Request().URL.Path)
}
