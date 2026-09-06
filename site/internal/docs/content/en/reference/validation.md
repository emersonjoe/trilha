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
| `eqfield=other` | equal to the other field's value, by form name | same | same | — |

Rules are separated by commas and applied in order; the first one to fail is the message for
that field. Every rule but `required` ignores an empty value, so an optional field only
answers for what somebody typed. A value that does not even convert (`abc` in an `int`) gets
`trilha.BindInvalid` and no rule message — one message per field.

**`required` is the zero value**: `0`, `false`, `""` and the zero date do not pass. Where
zero is a real answer, declare the field as a pointer: a `*int` that arrived holding `0` is
present, and only an absent field fails.

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
