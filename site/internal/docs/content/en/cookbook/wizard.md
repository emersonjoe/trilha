---
title: A form in steps
description: Where step one lives while somebody is on step two, and what happens when the draft expires.
---

Three screens, one hard question, and it is not the HTML: **where does step one live while
somebody is on step two?** What gets written instead is a page full of `<input type="hidden">`
(which the first upload breaks), or a half-filled row in the database (which every report then
has to learn to ignore), or one enormous screen with everything on it.

`c.Draft` is the framework's answer: a named draft, kept between one request and the next, with
a deadline.

## One struct per step

```go
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
```

This is the part worth copying. A single struct with every field and every `validate` tag cannot
be checked halfway: step one would fail on an address nobody has typed yet, and the way out
people find is to drop the rules until the last screen — where a message about a field three
screens back is useless.

## The end of a step

```go
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
```

Load, bind **only this step**, save, move on. A 422 here costs nothing that was typed earlier,
because the earlier screens are in the draft and not in this form.

## Every screen after the first

```go
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
```

No draft is not a failure. It is somebody whose draft expired, or who typed the address of step
two directly, or who finished this wizard yesterday — and the answer is step one, not an empty
form that would lose whatever they fill in here.

## The end

```go
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
```

`Clear` before the redirect, not after: a draft that became a record must stop existing, or the
next visit to step two resumes something that already happened.

## Where the draft lives

Under 2 KB of JSON it is a **signed cookie** — nothing to configure, nothing to clean up, and it
expires by itself. Above that it needs `Config.Drafts`, an interface of three methods over
whatever the app already runs; without one, `Save` returns an error naming that field instead of
setting a cookie the browser would silently drop.

:::warning
A draft is signed, so it cannot be edited by hand. It is **not secret**: what is in a cookie
travels to the browser and can be read there. A price, a discount, somebody else's name — those
belong behind `Config.Drafts`, with only the key in the cookie.
:::

The cookie is the person's own, so a draft is invisible to everybody else without a line of code
about ownership, and `TRILHA_SECRET` is what signs it: without a secret, `Save` says so.

## The indicator

`ui.Steps(steps, current)` draws where somebody is. Steps already done are links; the current one
carries `aria-current="step"`; the ones ahead are plain text — a wizard where step three is one
click away is a wizard whose steps did not have to happen in order.

The whole flow runs in
[`examples/cadastro`](https://github.com/emersonjoe/trilha/tree/main/examples/cadastro/app/assistente).
