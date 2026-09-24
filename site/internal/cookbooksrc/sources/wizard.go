package cookbook

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// WizardStep1 is what a multi-step form actually needs: one struct per step,
// each with only its own rules. A single struct with every field cannot be
// validated halfway — step one would fail on an address nobody has typed —
// and what people do instead is drop the rules until the last screen, where a
// message about a field three screens back is useless.
type WizardStep1 struct {
	Name  string `form:"name"  validate:"required,max=80"`
	Email string `form:"email" validate:"required,email"`
}

// WizardStep2 is the second screen.
type WizardStep2 struct {
	Postcode string `form:"postcode" validate:"required"`
	City     string `form:"city"     validate:"required"`
}

// Wizard is what travels between the screens.
type Wizard struct {
	Who   WizardStep1 `json:"who"`
	Where WizardStep2 `json:"where"`
}

// WizardPost is the end of a step: load what is there, bind only this step,
// save, move on. A 422 here costs nothing that was typed on an earlier screen,
// because the earlier screens are in the draft and not in this form.
func WizardPost(c *trilha.Ctx) error {
	var w Wizard
	_ = c.Draft("signup").Load(&w) // nothing yet on the first screen
	if err := c.Bind(&w.Who); err != nil {
		return err // FieldErrors: 422 with the messages next to the fields
	}
	if err := c.Draft("signup").Save(w, 30*time.Minute); err != nil {
		return err
	}
	return c.Redirect("/signup/where")
}

// WizardResume is every screen after the first. No draft is not a failure: it
// is somebody whose draft expired, or who typed the address of step two
// directly, and the answer is step one — not an empty form that would lose
// whatever they filled in here.
func WizardResume(c *trilha.Ctx) (h.Node, error) {
	var w Wizard
	if err := c.Draft("signup").Load(&w); err != nil {
		return nil, c.Redirect("/signup/who")
	}
	return h.Div(ui.Steps(wizardSteps, 2), wizardForm(c, w)), nil
}

// WizardFinish is where the draft becomes a record and stops existing.
func WizardFinish(c *trilha.Ctx) error {
	var w Wizard
	if err := c.Draft("signup").Load(&w); err != nil {
		return c.Redirect("/signup/who")
	}
	if err := saveSignup(c, w); err != nil {
		return err
	}
	c.Draft("signup").Clear()
	return c.Redirect("/signup/done")
}

var wizardSteps = []ui.Step{
	{Label: "Who", Href: "/signup/who"},
	{Label: "Where", Href: "/signup/where"},
	{Label: "Review"},
}

func wizardForm(c *trilha.Ctx, w Wizard) h.Node { return h.Form(h.Method("post"), trilha.CSRFInput(c)) }

func saveSignup(c *trilha.Ctx, w Wizard) error { return nil }
