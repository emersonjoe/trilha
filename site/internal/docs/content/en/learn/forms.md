---
title: Forms
description: POST in the same page.go, automatic CSRF protection and the redirect-after-write pattern.
---

A page can receive forms by exporting `POST` (or `PUT`, `PATCH`, `DELETE`) next to `Page`.
Trilha verifies the CSRF token before calling your function.

## The page with the form

`app/events/new/page.go`:

```go
package new

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"agenda/internal/events"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("New event")
	msg := c.Query("error")
	return h.Fragment(
		h.H1(h.Text("New event")),
		h.If(msg != "", h.P(h.Class("error"), h.Text(msg))),
		h.Form(h.Method("post"), h.Action("/events/new"),
			trilha.CSRFInput(c),
			h.Label(h.For("name"), h.Text("Name")),
			h.Input(h.ID("name"), h.Name("name"), h.Required(), h.Autofocus()),
			h.Label(h.For("city"), h.Text("City")),
			h.Input(h.ID("city"), h.Name("city")),
			h.Button(h.Type("submit"), h.Text("Publish")),
		),
	), nil
}

func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err // 400 on an invalid form, 413 if it exceeded the limit
	}
	name := strings.TrimSpace(c.Form("name"))
	if name == "" {
		return c.Redirect("/events/new?error=Enter+a+name")
	}
	ev := events.Create(name, c.Form("city"))
	return c.Redirect("/events/" + ev.Slug)
}
```

@demo form

## What happens on submit

1. The browser sends `POST /events/new` with the fields and the `_csrf`.
2. Trilha compares `_csrf` with the `trilha_csrf` cookie (constant time). Different or
   missing: **403**, and `POST` does not even run.
3. `POST` runs and returns `c.Redirect(...)`: a **303 See Other** response. The browser does
   a `GET` on the new URL. Reloading the page does not resubmit the form.

`trilha.CSRFInput(c)` creates the cookie on the first render and the hidden field.
JavaScript clients may send the same value in the `X-CSRF-Token` header.

## Validation and messages

The example above checks the name by hand and answers through the query string, which keeps
the POST → redirect → GET pattern and works without JavaScript. As soon as a form has more
than a couple of fields, put the rules on the struct instead: the `validate` tag sits next
to the field it talks about, and `Bind` applies every rule before returning.

```go
type entry struct {
	Name  string `form:"name" validate:"required,min=3,max=80"`
	Email string `form:"email" validate:"required,email"`
	Seats int    `form:"seats" validate:"min=1,max=10"`
}

func POST(c *trilha.Ctx) error {
	var in entry
	if err := c.Bind(&in); err != nil {
		if errs, ok := err.(trilha.FieldErrors); ok {
			// Same page, 422, values kept, one message per field.
			return c.Render(http.StatusUnprocessableEntity, form(c, in, errs))
		}
		return err
	}
	ev := events.Create(in.Name, in.Email, in.Seats)
	return c.Redirect("/events/" + ev.Slug)
}
```

`FieldErrors` is a `map[string]string` (field → message), so the form reads it straight:
`ui.Errors(errs, "email")` prints the message and `ui.InvalidIf(errs, "email")` marks the
input with `aria-invalid`. Nothing short-circuits — the person sees every mistake at once,
not one per submit.

The rules are `required`, `min`, `max`, `len`, `email`, `url`, `oneof` and `eqfield`; the
[validation reference](/reference/validation) has what each one means per type. Two of them
are worth spelling out here:

- **Every rule but `required` ignores an empty value.** An optional field with `min=3` only
  answers for what somebody typed.
- **`required` means "not the zero value".** Where `0` or `false` is a real answer, declare
  the field as a pointer (`*int`): absent stays absent, and zero arrives as zero.

Messages come in English. An app that speaks another language calls
`trilha.UseValidationPTBR()` in `Setup`, or writes its own into `trilha.ValidationMessages`.

### When the tag is not enough

A rule about the shape of a value belongs to the type, and then every form that uses the
type is covered:

```go
type Money string

func (m Money) Validate() error {
	if v, err := ParseMoney(string(m)); err != nil || v <= 0 {
		return errors.New("must be greater than zero")
	}
	return nil
}
```

A rule that reads two fields belongs to the struct: give it a `Validate() error` and it runs
at the end, only when no field failed. A rule you repeat across projects becomes a tag of
your own:

```go
trilha.AddRule("cep", func(f trilha.Field) bool { return validZIP(f.Text) })
trilha.ValidationMessages["cep"] = "invalid ZIP code"
```

Here is where this stops: the tag says what a **value** accepts, not what the **system**
accepts. "This account exists" and "this room is free that night" are questions for your
data, and they stay in your package. Run them after `Bind` and merge the result into the
same `FieldErrors`, so both kinds of message reach the person in the same response.

## Methods the browser does not send

HTML forms only send GET and POST. For "delete", export `DELETE` for API clients and make the
page's `POST` call the same logic:

```go
func DELETE(c *trilha.Ctx) error {
	if !events.Delete(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	return c.Redirect("/events")
}

func POST(c *trilha.Ctx) error { return DELETE(c) }
```

## Limits

The request body is limited to 1 MiB by default (`Config.MaxBodyBytes`). Above that the
response is 413 before your code runs.

## Challenge

Add a numeric `seats` field to the form, accept only 1 to 10, and show the message next to
the field instead of on the next page.

:::solution
```go
type entry struct {
	Name  string `form:"name" validate:"required,min=3"`
	City  string `form:"city"`
	Seats int    `form:"seats" validate:"required,min=1,max=10"`
}

// In POST, c.Bind(&in) returns trilha.FieldErrors, and the page renders again
// with c.Render(http.StatusUnprocessableEntity, ...) and ui.Errors(errs, "seats").
```
:::
