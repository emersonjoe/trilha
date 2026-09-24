package cookbook

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// InviteLink is what somebody inside the application sends to somebody
// outside it. The link is the credential: it says what it is for, who it is
// about and until when, all signed, and it needs no row in a table.
//
// Uses: 1 is the difference between a form and a form somebody can fill in
// twice. It is the only part that needs storage, and only because "how many
// times has this been used" cannot be answered by arithmetic.
func InviteLink(c *trilha.Ctx, taskID string) (string, error) {
	return c.Link("form", trilha.LinkOpts{
		Data: map[string]string{"task": taskID},
		TTL:  7 * 24 * time.Hour,
		Uses: 1,
		Path: "/form",
	})
}

// PublicForm is the route the link opens. There is no session here and there
// is not supposed to be one.
//
// Every way the link can fail — wrong signature, wrong purpose, expired,
// already used — answers the same 404. Telling a stranger which of the four
// happened tells them how close they are.
func PublicForm(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim("form")
	if err != nil {
		return nil, err
	}
	return h.Div(
		ui.H1(h.Text("Fill in your details")),
		h.Form(h.Method("post"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("task"), h.Value(link.Data["task"])),
			ui.Submit(h.Text("Send"))),
	), nil
}

// SubmitPublicForm spends the link — after the work and not before. A link
// burned by a validation error is a link somebody has to ask for again
// because they typed a date wrong.
func SubmitPublicForm(c *trilha.Ctx) error {
	link, err := c.Claim("form")
	if err != nil {
		return err
	}
	if err := saveAnswer(c, link.Data["task"]); err != nil {
		return err
	}
	if err := link.Consume(); err != nil {
		return err
	}
	return c.Render(http.StatusOK, ui.H1(h.Text("Thank you")))
}

// VerifyCode is the same primitive with no limit at all: a code printed on a
// document, checked as many times as anybody likes until it expires. Nothing
// is stored and nothing is looked up — verifying is a signature check.
func VerifyCode(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim("verify")
	if err != nil {
		return nil, err
	}
	return ui.H1(h.Text("Document " + link.Data["doc"] + " is authentic")), nil
}

func saveAnswer(c *trilha.Ctx, task string) error { return nil }
