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

## Lookup by code, without an account

A link is one half of the public. The other half is the person with a piece of paper: a protocol
number, the code printed on a receipt, plus something they already have — the last digits of the
phone the case was opened with. No link was ever sent, so there is nothing to claim. They type,
and they see their own case.

```sh
trilha add public-lookup
```

It writes `app/consulta/` — a folder of its own, deliberately outside whatever tree the app put
behind a login, because whoever types a protocol number is precisely whoever has no account (an
application with two publics gives its sign-in flow an `Options.Audience` of its own, and this
folder stays outside it) — plus `internal/consulta/`, where `Buscar` and the rule about which
events are public live.

The screen that gets written by hand instead is the one that answers "no such protocol" for a
number that does not exist and "wrong code" for a factor that does not match. That is a free
oracle: whoever is walking the number space now knows which numbers are real, and only has the
second factor left to find. This one has a single no.

**The check digit comes first.** A code somebody types should reach the database only when it
is a code that could have been issued:

```go
// Emitir mints a code: the check digits travel with the number, printed on the
// receipt beside it. Two of them catch every single-character typo and every
// swap of two neighbours, and only one code in ninety-seven is worth a query
// at all — which is what makes enumeration expensive before anything counts it.
func Emitir(base string) string { return base + trilha.CheckDigit(base) }
```

`trilha.CheckDigit` is ISO 7064 MOD 97-10, the IBAN's scheme. On the way in the screen calls
`trilha.HasCheckDigit(codigo)` and, when it is false, answers with the same words a code that
does not exist gets — never with "malformed", which would be a second answer and therefore a
way of sorting invented numbers from real ones. Letters count as `A=10 … Z=35`, and spaces and
hyphens are ignored, so a code printed as `2026-0001-04` and typed as `20260001 04` are the same
code. What it costs the person is nothing — the digits are part of what they were given. What it
costs whoever is guessing is ninety-six attempts out of every ninety-seven, refused without a
query.

**Then two budgets, because there are two attacks.** One address walking the number space is
stopped by a limit per IP; one real protocol number being hammered from a botnet is stopped only
by a limit per code:

```go
var (
	porIP     = trilha.NewLimiter(trilha.RateLimit{RPS: 0.05, Burst: 5})
	porCodigo = trilha.NewLimiter(trilha.RateLimit{RPS: 0.05, Burst: 5})
)
```

**And one no.** A code nobody issued and a factor that does not match give the same error, the
same status and the same bytes — the recipe's test asserts the two bodies are identical. The
comparison of the factor is `subtle.ConstantTimeCompare`, and a lookup that found nothing still
pays for a comparison against a dummy, so the answer for an unknown code does not come back
measurably sooner than the answer for a wrong one. Every attempt is audited with the code masked
to its last three characters: a trail that keeps whole protocol numbers is a list of valid
protocol numbers.

What comes out is `Registro` plus its `[]Evento`, and each event carries `Visivel`. Only what
the server marked as public is rendered — the timeline an operator sees and the one the citizen
sees are not the same timeline, and a filter written in the page is a filter somebody forgets on
the second page. The screen draws it with `ui.Steps` (an ordered list, `aria-current` on where
the case is) and `ui.Date(c, ev.Em, ui.Relative())`, on a form with real labels, `inputmode`,
`autocomplete="off"` and `aria-describedby` — the public here is a citizen on a phone.

@demo public-lookup

:::warning
**Do not put this route behind a login,** and do not echo back what was typed. Both are the same
mistake in two shapes: the first asks for the account the person does not have, the second makes
the refusal of a code that exists a different page from the refusal of one that does not.
:::
