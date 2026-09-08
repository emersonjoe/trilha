---
title: CSV
description: Export a spreadsheet Excel opens without mangling it, and read one back saying which line and which column are wrong.
---

Every internal application ends up doing this twice: a button that downloads the list, and a
screen that takes it back. Both fail in the same few places. On the way out: no BOM, so every
accent opens as mojibake; a comma where the person's Excel expects a semicolon, so the file
opens as one long column; a date in a format the spreadsheet reads as text. On the way in:
"error in the file", which leaves somebody to find one bad date among four thousand rows by
eye.

`c.CSV` and `trilha.BindCSV` are those two halves.

## Exporting

```go
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
```

The heading of each column is the `csv` tag, or the field name when there is none; `csv:"-"`
leaves a field out, and an unexported field was never in. What the locale decides:

| | `Locale: "en"` | `Locale: "pt-BR"` |
|---|---|---|
| separator | `,` | `;` |
| date | `2026-09-08 15:04` | `08/09/2026 15:04` |
| decimal | `1234.5` | `1234,5` |
| boolean | `yes` / `no` | `sim` / `não` |

Both start with a UTF-8 BOM and end their lines with CRLF, which is what RFC 4180 says and
what Excel on Windows reads.

:::note
`Config.TimeZone` is what a date is shown in. Without it every timestamp is UTC, which is
correct and is also three hours off for the person reading it.
:::

## Two hundred thousand rows

Pass a channel instead of a slice and the file is written as it is produced — the memory is
one row, and the download starts before the query has finished.

```go
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
```

`c.CSV` lifts the write deadline itself, so a long export on a slow link is not killed as a
stuck handler. What it cannot do for you is end the producer: the `select` on
`c.Context().Done()` above is the whole reason this snippet is longer than the one before it.

## Importing

```go
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
```

What `BindCSV` decides for you, and why:

- **The separator and the BOM are detected**, so one handler takes the file Excel wrote in
  Brazil and the one a script wrote anywhere else.
- **The header matches by tag**, ignoring case and surrounding space, in any order. Somebody
  who moved a column has not made a mistake.
- **A heading no field claims is a warning**, not an error, and lands in `res.Warnings`. A
  spreadsheet grows a column all the time; refusing the file for it only teaches people to
  delete columns before uploading.
- **A required column that is missing is one message at line 1**, not the same message on each
  of ten thousand lines.
- **Only rows that pass are appended.** After `res.OK()` the slice is the whole file; after a
  failure it holds nothing worth saving.
- **A blank line is skipped**, and `MaxRows` (100,000 by default) is an error rather than a
  truncation — half an import that reports success is worse than one that fails.
- **A date reads as `dd/mm/yyyy` as well as ISO**, because that is what a Brazilian
  spreadsheet exports.

## The error screen

`res.Errors` is `[]trilha.CSVError{Line, Column, Message}` — the line as the person's editor
numbers it, with the header as line 1, and the column by its heading in the file. Both halves
can be searched for, which is the point.

`ui.CSVErrors` renders them as a table with the first twenty:

```go
// ShowCSVErrors is the shortest form of the error screen: the default table,
// the first twenty cells, no button.
func ShowCSVErrors(c *trilha.Ctx, res trilha.CSVResult) error {
	return c.Render(http.StatusUnprocessableEntity, ui.CSVErrors(c, res))
}
```

| Line | Column | Problem |
|---|---|---|
| 4 | `code` | must have at most 20 characters |
| 4 | `name` | required |
| 17 | `weight` | must be 0 or more |

The messages are the validation messages, so `UseValidationPTBR` translates the import along
with every form.

## Where the code comes from

[`examples/cookbook/csv.go`](https://github.com/emersonjoe/trilha/blob/main/examples/cookbook/csv.go),
and the round trip is exercised end to end in the blog example at
[`app/documentos/planilha`](https://github.com/emersonjoe/trilha/blob/main/examples/blog/app/documentos/planilha/route.go):
the file the `GET` writes is the file the `POST` accepts back.
