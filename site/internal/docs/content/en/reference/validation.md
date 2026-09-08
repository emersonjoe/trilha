---
title: Validation
description: The validate tag, the rules per type, your own rules and the messages Bind returns.
---

`Bind` validates while it fills: after converting the values, it applies the `validate` tag
of each field and returns `FieldErrors` (field → message) with everything that failed. The
same rules run for a form and for JSON — with a JSON body the field is named by its `json`
tag, which is the name the client recognises.

```go
type entry struct {
	Name     string    `form:"name" validate:"required,min=3,max=80"`
	Email    string    `form:"email" validate:"required,email"`
	Confirm  string    `form:"confirm" validate:"eqfield=email"`
	Date     time.Time `form:"date" validate:"required,min=2026-01-01"`
	Plan     string    `form:"plan" validate:"oneof=free pro"`
	Discount *int      `form:"discount" validate:"required,min=0"`
}
```

## Rules

| Rule | Text | Number | `time.Time` | `[]string` (checkbox, select) |
|---|---|---|---|---|
| `required` | not empty | any value, including `0` only through a pointer | not the zero date | at least one |
| `min=n` | at least `n` characters | value `>= n` | date is not before `n` (`2006-01-02`) | at least `n` chosen |
| `max=n` | at most `n` characters | value `<= n` | date is not after `n` | at most `n` chosen |
| `len=n` | exactly `n` characters | — | — | exactly `n` chosen |
| `email` | one `@`, a domain with a dot | — | — | — |
| `url` | absolute `http`/`https` | — | — | — |
| `oneof=a b c` | value is one of the options, separated by spaces | same, as text | — | — |
| `enum=<name>` | one of a registered `trilha.Enum`; the message lists the labels |
| `eqfield=other` | equal to the other field's value, by form name | same | same | — |

Rules are separated by commas and applied in order; the first one to fail is the message for
that field. Every rule but `required` ignores an empty value, so an optional field only
answers for what somebody typed. A value that does not even convert (`abc` in an `int`) gets
`trilha.BindInvalid` and no rule message — one message per field.

**`required` is the zero value**: `0`, `false`, `""` and the zero date do not pass. Where
zero is a real answer, declare the field as a pointer: a `*int` that arrived holding `0` is
present, and only an absent field fails.

## Lists and matrices

A form grows: three dependants, five order rows, one permission per module. The name carries
the position — `items[0].name`, `items[1].name` — or the key — `perm[docs]` — and `Bind`
fills a slice of structs or a map from it:

```go
type Row struct {
	Name string `form:"name" validate:"required,max=40"`
	Qty  int    `form:"qty" validate:"min=1"`
}

var in struct {
	Items []Row          `form:"items" validate:"minitems=1,maxitems=50"`
	Perm  map[string]int `form:"perm"`
}
```

| Rule | What it counts |
|---|---|
| `minitems=n` | at least `n` rows or keys |
| `maxitems=n` | at most `n` rows or keys |
| `lenitems=n` | exactly `n` rows or keys |

These three are the exception to "an empty value skips the rule": "at least one row" is
exactly a sentence about the empty case.

The name of the input is also the key of the message, so `ui.Errors(errs, "items[1].qty")`
finds the field the person is looking at. An index nobody sent is not a row: `items[0]` and
`items[7]` arrive as two rows, in that order, and the message for the second one says
`items[1].qty` — the position after compacting, which is the position the form is about to
draw again. Nothing is allocated by index, so `items[9999999999]` costs one row instead of
ten billion; the ceiling is `maxitems` when the tag has one and `trilha.MaxItems` (1000)
when it does not. A map key is taken literally, spaces and dots included, and a key with a
bracket in it is refused.

`BindJSON` speaks the same key: `{"items":[…]}` fails with `items[1].qty`, the same string
the HTML form produces, so one screen serves both.

## A form that comes as data

Some forms are not in the code: the step of a workflow, the public form behind a token, the
settings of a tenant. `trilha.Schema` is that form as data, and `trilha.BindSchema` reads it
through the engine above — the same rules, the same messages, the same `FieldErrors`:

```go
values, err := trilha.BindSchema(c, schema)
errs, ok := err.(trilha.FieldErrors)
if err != nil && !ok {
	return err
}
if ok {
	return c.Render(http.StatusUnprocessableEntity, page(c, values, errs))
}
```

The values come back as text: a schema that came from a table has no Go type to fill, and
converting to `any` would only move the conversion into the app. `SchemaField` says what a
tag would say — `Required`, `Min`, `Max`, `Pattern`, `Options` — and `Type` says which
control draws it: `text`, `textarea`, `number`, `date`, `datetime`, `select`, `checkbox`,
`file`, `signature`, `display`. A `display` field is a paragraph in the middle of the form:
it is not read and never gets a message. A file is read by `c.File`, as any file is.

A schema with an unknown type, a field with no name or a pattern that does not compile is a
bug in the app, not something the person filling the form did: `schema.Check()`, which
`BindSchema` calls first, answers with a plain error and never a 422. `ui.SchemaForm(schema,
values, errs)` draws the fields; the `<form>`, the CSRF input and the button stay yours,
because where the form posts is not in the schema.

## Enum — a domain list declared once

A status, a document type, a pipeline stage. Written by hand it exists in four places — the
badge on the table, the options of a select, the validation of the form, and a comment on the
tag — and the fourth one is where the label is wrong.

```go
// internal/docs/status.go
var Status = trilha.Enum{
	{Value: "queued",     Label: "Queued"},
	{Value: "processing", Label: "Processing", Tone: "info"},
	{Value: "processed",  Label: "Processed",  Tone: "success"},
	{Value: "error",      Label: "Error",      Tone: "danger"},
}
```

`Tone` is one of `muted`, `info`, `success`, `warning`, `danger` and `accent` — a name from the
theme and never a CSS class. Whoever declares a status picks a meaning; picking a colour is how
two screens end up with two different greens. Empty means `muted`.

| Symbol | What it does |
|---|---|
| `Enum.Label(v)` | what a person reads, or the raw value when the list does not know it |
| `Enum.Tone(v)` | the tone, `muted` for an unknown value |
| `Enum.Has(v)` | is this one of the list |
| `Enum.Options(current, placeholder…)` | the `<option>` list, current value marked |
| `Enum.Values()`, `Enum.Labels()` | both, in declaration order |
| `ui.Status(e, v)` | the badge: the label, in the tone |

### The four uses

```go
ui.Status(docs.Status, doc.Status)                       // badge
{Key: "status", Cell: func(d Doc) h.Node {               // DataTable column
	return ui.Status(docs.Status, d.Status)
}}
ui.Field("status", "Status", ui.Select(docs.Status.Options(form.Status)))   // select
type Form struct {                                       // validation
	Status string `validate:"required,enum=docs.Status"`
}
```

The tag cites the enum by the name it was registered under:

```go
func Setup(a *trilha.App) error {
	trilha.RegisterEnum("docs.Status", docs.Status)
	return nil
}
```

Registering the same list again is fine — `Setup` is where this belongs, and a test suite boots
the app once per test. Two *different* lists behind one name panics: the form would validate
against one and the select would draw the other.

The message lists the labels, not the values: `invalid option; accepts Queued, Processing,
Processed, Error`. The person filling the form read labels.

### A value the list does not know

It renders raw, in the muted tone, and `Has` says no. A row written before somebody retired
that value must not take the screen down — showing `legacy` in grey is a screen you can act
on; a panic is not.

### What is not here

Translation of the labels. `Label` is a string, and an application with two languages passes
two enums or builds one from its own table. A translation table inside this would be a second,
worse i18n.

## Your own rules

| Symbol | Role |
|---|---|
| `trilha.Validator` | `interface{ Validate() error }`: the value checks itself |
| `trilha.AddRule(name, func(Field) bool)` | registers a name for the tag; panics if the name exists |
| `trilha.Field` | what a rule sees: `Name`, `Param`, `Text`, `Value`, `Other(name)` |
| `trilha.ValidationMessages` | `map[string]string` of the messages; `{param}` is replaced |
| `trilha.UseValidationPTBR()` | switches the messages, `BindInvalid` included, to Portuguese |

A field whose **type** has `Validate() error` is checked after the tag rules pass, with the
error message going to `FieldErrors` as it is (both a value and a pointer receiver work). The
**struct** may have `Validate() error` too: it runs at the end, only when no field failed —
which is what makes a check that reads two fields safe. It may return `FieldErrors` to say
which field is at fault; any other error comes back from `Bind` untouched.

```go
trilha.AddRule("cep", func(f trilha.Field) bool { return validZIP(f.Text) })
trilha.ValidationMessages["cep"] = "invalid ZIP code"
```

`Field.Value` is the converted value (`string`, `bool`, `int64`, `float64`, `time.Time`,
`[]string`, or `nil` when the field was not sent) and `Field.Text` is the same thing as text,
which is all most rules need. A rule that compares fields reads the other one with
`f.Other("email")`.

## Where validation stops

The tag says what a **value** accepts. Whether an account exists, whether the room is free
that night, whether this person may do this — those read your data and belong to your
package. Run them after `Bind` and merge into the same `FieldErrors`, so every message
reaches the person in one response:

```go
errs := trilha.FieldErrors{}
if err := c.Bind(&in); err != nil {
	fe, ok := err.(trilha.FieldErrors)
	if !ok {
		return err
	}
	errs = fe
}
for field, msg := range plan.Check(&in) {
	errs.Add(field, msg)
}
if errs.Any() {
	return c.Render(http.StatusUnprocessableEntity, page(c, in, errs))
}
```

Unknown rule names panic on the first request that hits the field, on purpose: a typo in a
tag would otherwise be a form that accepts anything in production.
