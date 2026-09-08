package cookbook

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Row is the spreadsheet and the form at once. The csv tag is the heading in
// the file; the validate tag is the same rule a screen would apply, written
// once and holding in both places.
type Row struct {
	Code   string  `csv:"code"   validate:"required,max=20"`
	Name   string  `csv:"name"   validate:"required"`
	Parent string  `csv:"parent"`
	Weight float64 `csv:"weight" validate:"min=0"`
	id     string  `csv:"-"` // never exported, never read
}

// ExportPlan sends the whole plan as a file the person can open. The BOM, the
// separator and the date format follow Config.Locale, which is the difference
// between a spreadsheet that opens in columns and one that opens as a single
// column of mojibake.
func ExportPlan(c *trilha.Ctx) error {
	rows, err := plan(c)
	if err != nil {
		return err
	}
	return c.CSV("classification-plan.csv", rows)
}

// StreamPlan is the same export when the answer is two hundred thousand rows:
// a channel is written as it is produced, so the memory is one row and the
// person sees the download start immediately.
//
// The goroutine has to end even when the download does not: the browser that
// closes the connection cancels the request, and a producer that never learns
// that is a goroutine leak with a database cursor attached to it.
func StreamPlan(c *trilha.Ctx) error {
	ch := make(chan Row)
	go func() {
		defer close(ch)
		for _, r := range everyRow(c) {
			select {
			case ch <- r:
			case <-c.Context().Done():
				return
			}
		}
	}()
	return c.CSV("classification-plan.csv", (<-chan Row)(ch))
}

// ImportPlan reads the file back and says which cell is wrong. The separator
// and the BOM are detected, the header matches by tag in any order, and each
// row goes through the same validate tags a form would.
func ImportPlan(c *trilha.Ctx) error {
	up, err := c.File("file", trilha.FileRules{
		Accept:  []string{"text/csv", "text/plain"},
		MaxSize: 5 << 20,
	})
	if err != nil {
		return err
	}
	defer up.Close()

	var rows []Row
	res, err := trilha.BindCSV(up, &rows)
	if err != nil {
		// A file that is not a file at all — empty, or unreadable. It belongs
		// on the field the person used, not in a 500.
		return trilha.FieldErrors{"file": err.Error()}
	}
	if !res.OK() {
		return c.Render(http.StatusUnprocessableEntity, ui.CSVErrors(c, res, ui.CSVErrorsOpts{
			Action: ui.ButtonLink("/import", h.Text("Choose another file")),
		}))
	}
	if err := save(c, rows); err != nil {
		return err
	}
	return c.Redirect("/import/done")
}

// ShowCSVErrors is the shortest form of the error screen: the default table,
// the first twenty cells, no button.
func ShowCSVErrors(c *trilha.Ctx, res trilha.CSVResult) error {
	return c.Render(http.StatusUnprocessableEntity, ui.CSVErrors(c, res))
}

// plan and everyRow stand in for the repository this recipe does not have.
func plan(c *trilha.Ctx) ([]Row, error) { return everyRow(c), nil }

func everyRow(c *trilha.Ctx) []Row {
	return []Row{{Code: "A1", Name: "Documents"}, {Code: "A2", Name: "Contracts", Parent: "A1"}}
}

func save(c *trilha.Ctx, rows []Row) error { return nil }
