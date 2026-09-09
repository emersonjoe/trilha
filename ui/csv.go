package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// CSVErrorsOpts is what the list of rejected cells says. The zero value is a
// sensible screen.
type CSVErrorsOpts struct {
	// Title is the sentence above the table. Empty uses "The file has N
	// errors" in the app's language.
	Title string
	// Limit is how many cells are listed. Default 20: the person is going back
	// to the spreadsheet either way, and a page with four hundred rows of
	// error is a page nobody reads.
	Limit int
	// Action is what comes after the table — "Choose another file", usually.
	Action h.Node
}

// CSVErrors renders what BindCSV rejected, by line and column.
//
//	res, err := trilha.BindCSV(up, &nodes)
//	if err != nil {
//		return trilha.FieldErrors{"file": err.Error()}
//	}
//	if !res.OK() {
//		return c.Render(422, page(ui.CSVErrors(c, res, ui.CSVErrorsOpts{
//			Action: ui.ButtonLink("/import", h.Text("Choose another file")),
//		})))
//	}
//
// It exists because the alternative is what every import screen actually
// shows — "error in the file" — which leaves somebody to find one bad date in
// four thousand rows by eye. Line is numbered the way their editor numbers it,
// with the header as line 1, and Column is the heading as it appears in the
// file, so both halves of the message can be searched for.
//
// Warnings are listed under the errors and read differently on purpose: a
// column nobody claimed did not stop the import.
//
//	see: trilha.BindCSV, ui.Empty
func CSVErrors(c *trilha.Ctx, res trilha.CSVResult, opts ...CSVErrorsOpts) h.Node {
	var o CSVErrorsOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	limit := o.Limit
	if limit <= 0 {
		limit = 20
	}
	pt := langOf(c) == "pt-BR"

	title := o.Title
	switch {
	case title != "":
	case len(res.Errors) == 1:
		title = word(pt, "The file has one error", "O arquivo tem um erro")
	default:
		title = replaceCount(word(pt, "The file has %s errors", "O arquivo tem %s erros"), len(res.Errors))
	}

	head := h.Thead(h.Tr(
		h.Th(h.Text(word(pt, "Line", "Linha")), Num()),
		h.Th(h.Text(word(pt, "Column", "Coluna"))),
		h.Th(h.Text(word(pt, "Problem", "Problema"))),
	))
	rows := make([]h.Node, 0, limit)
	for i, e := range res.Errors {
		if i >= limit {
			break
		}
		col := e.Column
		if col == "" {
			// A whole-line problem has no column, and an empty cell there
			// reads as a missing value rather than as "the line itself".
			col = word(pt, "line", "linha")
		}
		rows = append(rows, h.Tr(
			h.Td(h.Text(strconv.Itoa(e.Line)), Num()),
			h.Td(Code(col)),
			h.Td(h.Text(e.Message)),
		))
	}

	kids := []h.Node{
		Alert(title, Destructive(), AlertDescription(h.Text(
			word(pt,
				"Fix these lines in the spreadsheet and send it again. Nothing was imported.",
				"Corrija estas linhas na planilha e envie de novo. Nada foi importado.")))),
		Table(head, h.Tbody(rows...)),
	}
	if rest := len(res.Errors) - limit; rest > 0 {
		kids = append(kids, Muted(h.Text(replaceCount(
			word(pt, "and %s more", "e mais %s"), rest))))
	}
	for _, w := range res.Warnings {
		kids = append(kids, Muted(h.Text(w)))
	}
	if o.Action != nil {
		kids = append(kids, o.Action)
	}
	return Stack(kids...)
}

// word picks the sentence the person reads. The kit's components carry their
// own text in both languages, so a screen never has to pass a label just to
// get one that is not English.
func word(pt bool, en, br string) string {
	if pt {
		return br
	}
	return en
}

// replaceCount fills the one placeholder these sentences have. fmt.Sprintf
// would do it, and would also accept a caller's Title as a format string —
// which is how a title with a stray percent sign becomes %!s(MISSING) on the
// screen that is already reporting an error.
func replaceCount(s string, n int) string {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '%' && s[i+1] == 's' {
			return s[:i] + strconv.Itoa(n) + s[i+2:]
		}
	}
	return s
}
