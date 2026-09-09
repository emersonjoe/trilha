---
title: A public link
description: The form somebody outside fills in, and the code that verifies a document — both with no login and no table of tokens.
---

Three flows in every internal application happen with no login: somebody outside fills in a
form, somebody checks a document by a code, somebody answers a request from an e-mail. What
gets written for them is a random string in a table, in the clear, with no deadline — and a
token in a URL that is short enough to guess is guessed.

`c.Link` and `c.Claim` are that pattern, with the parts nobody remembers already in place.

## The link

```go
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
```

`Uses: 1` is the difference between a form and a form somebody can fill in twice. It is also the
only part that needs storage, and only because "how many times has this been used" is not
something arithmetic can answer.

## The route it opens

```go
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
```

**Every way the link can fail answers the same 404** — wrong signature, wrong purpose, expired,
already spent. Telling a stranger which of the four happened tells them how close they are. And
a wrong token costs the address that sent it a point of a small budget: guessing a token in a URL
is brute force, and brute force is answered by making wrong answers expensive.

## Spending it

```go
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
```

`Consume` comes **after** the work, not before. A link burned by a validation error is a link
somebody has to ask for again because they typed a date wrong. Opening the page spends nothing
either — a reload would otherwise burn the link somebody is still filling in.

## The same thing with no limit

```go
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
```

With `Uses: 0` there is no state at all: no row, no lookup, no cleanup. Verifying is a signature
check, and the link keeps working until it expires. That is the verification code printed on a
document.

:::warning
**What is in the link is signed, not secret.** Anybody holding it can read the `Data` — it is
base64, not encryption. Put an id in it, not a name, a price or a reason. Anything that must not
be read belongs in your own table, found by that id.
:::

`Config.Links` counts the uses of limited links; nil counts them in the process, which is honest
about one replica and said once in the log. A real one is an `UPDATE ... WHERE uses < max` or a
Redis `INCR` — the interface has two methods and the hard one is atomic on purpose.
