// Package uploadprogress is the upload-progress pattern: files sent with a
// progress bar, checked by content on the server, and the same form working
// with JavaScript off.
package uploadprogress

import (
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// rules are checked against the bytes, not against what the browser claims.
var rules = trilha.FileRules{MaxSize: 10 << 20, MaxFiles: 5, Accept: []string{"image/*", "application/pdf"}}

// Save keeps one file — your blob store. The upload is closed after it.
var Save = func(c *trilha.Ctx, up *trilha.Upload) error { return nil }

// Page draws the form.
func Page(c *trilha.Ctx) (h.Node, error) { return screen(c, nil), nil }

// POST receives the files. A refusal answers the fragment the kit swaps in.
func POST(c *trilha.Ctx) error {
	ups, err := c.Files("files", rules)
	if err != nil {
		var fe trilha.FieldErrors
		if !errors.As(err, &fe) {
			return err
		}
		if c.Fragment() == "upload" {
			return c.Render(http.StatusUnprocessableEntity, panel(c, fe))
		}
		return c.Render(http.StatusUnprocessableEntity, screen(c, fe))
	}
	for _, up := range ups {
		err := Save(c, up)
		up.Close()
		if err != nil {
			return err
		}
	}
	c.Flash(ui.FlashSuccess, "Sent.")
	return c.Redirect(c.Request().URL.Path)
}

func screen(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	return ui.Stack(ui.PageHeader("Files"), panel(c, errs), ui.UploadScript(c))
}

func panel(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	return h.Form(h.ID("upload"), h.Method("post"), h.Enctype("multipart/form-data"), ui.UploadTo("upload"),
		h.Class("ui-stack"), trilha.CSRFInput(c),
		ui.Field("files", "Files (images or PDF, 10 MB each)", ui.Input(h.ID("files"), h.Name("files"),
			h.Type("file"), h.Attr("multiple", ""), ui.InvalidIf(errs, "files")), ui.Errors(errs, "files")),
		ui.UploadBar(h.Aria("label", "Upload progress")), ui.Submit(h.Text("Send")))
}
