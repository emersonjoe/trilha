// Package asyncform is the async-form pattern: a form that posts without
// leaving the page, and a 422 that comes back with the message beside the
// field and the focus on it.
package asyncform

import (
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Profile is what the form edits; the tags are its rules.
type Profile struct {
	Name  string `form:"name" validate:"required,max=80" label:"Name"`
	Email string `form:"email" validate:"required,email" label:"E-mail"`
}

// Save is where a valid profile goes — your store.
var Save = func(c *trilha.Ctx, p Profile) error { return nil }

// Page draws the form.
func Page(c *trilha.Ctx) (h.Node, error) { return form(c, Profile{}, nil), nil }

// POST validates by the tags. The 422 answers the fragment: the kit swaps it
// in place and focuses the first field marked invalid.
func POST(c *trilha.Ctx) error {
	var p Profile
	if err := c.Bind(&p); err != nil {
		var fe trilha.FieldErrors
		if errors.As(err, &fe) {
			return c.Render(http.StatusUnprocessableEntity, form(c, p, fe))
		}
		return err
	}
	if err := Save(c, p); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "Saved.")
	return c.Redirect(c.Request().URL.Path)
}

func form(c *trilha.Ctx, p Profile, errs trilha.FieldErrors) h.Node {
	return h.Form(h.ID("profile"), h.Method("post"), ui.Swap("profile"), h.Class("ui-stack"),
		trilha.CSRFInput(c), ui.FormError(),
		ui.Field("name", "Name", ui.Input(h.ID("name"), h.Name("name"), h.Value(p.Name),
			ui.InvalidIf(errs, "name")), ui.Errors(errs, "name")),
		ui.Field("email", "E-mail", ui.Input(h.ID("email"), h.Name("email"), h.Type("email"),
			h.Value(p.Email), ui.InvalidIf(errs, "email")), ui.Errors(errs, "email")),
		ui.Submit(h.Text("Save")))
}
