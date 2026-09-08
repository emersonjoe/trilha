package trilha

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// The two halves of the same chore. Exporting and importing a spreadsheet is
// what every internal application does, and the parts people get wrong are
// always the same ones: the BOM without which Excel mangles every accent, the
// separator that is a semicolon in half the world, and — on the way back in —
// saying *which line and which column* is wrong instead of "error in the file".

// CSVError is one cell that did not pass, named so the person can find it in
// their spreadsheet: the line as the editor numbers it (the header is line 1)
// and the column by its heading.
type CSVError struct {
	Line    int    `json:"line"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// CSVResult is what came of reading a file.
type CSVResult struct {
	// Rows is how many data lines were read, blank ones aside.
	Rows int `json:"rows"`
	// Errors is every bad cell, in file order.
	Errors []CSVError `json:"errors,omitempty"`
	// Warnings is what was odd but not fatal — a column in the header that no
	// field claims. It is a warning and not an error because a spreadsheet
	// grows a column all the time, and refusing the file for it would only
	// teach people to delete columns before uploading.
	Warnings []string `json:"warnings,omitempty"`
}

// OK reports whether every row can be used.
func (r CSVResult) OK() bool { return len(r.Errors) == 0 }

// CSVRules bounds what a file may be. The zero value is the default.
type CSVRules struct {
	// MaxRows is the ceiling against the file that does not end. Default
	// 100,000; a file above it is an error and not a truncation, because half
	// an import that reports success is worse than one that fails.
	MaxRows int
	// Separator forces the delimiter. Zero detects it from the header, which
	// is what a file written in another country needs.
	Separator rune
}

const (
	defaultCSVMaxRows = 100_000
	// maxCSVErrors stops the collection, not the world. A file with five
	// hundred bad cells is not going to be fixed one cell at a time, and
	// holding a message for each of a hundred thousand rows is how a 5 MB
	// upload becomes a gigabyte of process.
	maxCSVErrors = 500
	// bomUTF8 is what makes Excel read UTF-8 instead of the local codepage.
	bomUTF8 = "\ufeff"
	langPT  = "pt-BR"
)

// langOf is the same two-branch decision ui makes, on the string rather than
// the Ctx: a spreadsheet is written for a person, and the separator, the date
// and the word for true all follow from where that person is.
func langOf(locale string) string {
	if strings.EqualFold(locale, langPT) || strings.EqualFold(locale, "pt") {
		return langPT
	}
	return "en"
}

// ---- export ----------------------------------------------------------------

// CSV writes rows as a spreadsheet the person can open.
//
//	type Line struct {
//		When time.Time `csv:"When"`
//		Who  string    `csv:"User"`
//		Kept bool      `csv:"Kept"`
//		id   string    `csv:"-"`
//	}
//
//	return c.CSV("audit-2026-09.csv", rows)
//
// Each column's heading comes from the csv tag, or from the field name when
// there is none; a field tagged "-" is left out. Dates, decimals and booleans
// are written the way Config.Locale and Config.TimeZone write them, and the
// file starts with a UTF-8 BOM — without it Excel reads every accent as
// mojibake, which is the first thing anybody notices and the last thing
// anybody expects a framework to have handled.
//
// rows is a slice or a receive-only channel. The channel is what an export of
// two hundred thousand lines wants: rows are written as they arrive and
// nothing is held. (The issue asked for an iter.Seq too; this module builds on
// Go 1.22, where that does not exist.)
//
// It streams, so the status is already sent by the time the second row goes
// out: a failure after that cannot become a 500 and comes back here instead,
// for the handler to log.
//
//	see: Ctx.Attachment, BindCSV
func (c *Ctx) CSV(name string, rows any) error {
	name = safeName(name)
	if !strings.HasSuffix(strings.ToLower(name), ".csv") {
		name += ".csv"
	}
	// Everything reflection can refuse is refused before a byte goes out, so a
	// bad call is still a 500 and not a half-written download.
	x, err := newCSVWriter(c.w, rows, langOf(c.app.cfg.Locale), c.Location())
	if err != nil {
		return err
	}
	h := c.w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", disposition("attachment", name))
	h.Set("X-Content-Type-Options", "nosniff")
	// A long export on a slow link is not a stuck handler; it is a handler
	// doing what it was told.
	_ = c.NoWriteDeadline()
	c.w.WriteHeader(http.StatusOK)
	if c.r.Method == http.MethodHead {
		return nil
	}
	return x.run()
}

// csvWriter is the export with the response taken out of it, so a test — and
// an application writing a file to disk — can call the same code.
type csvWriter struct {
	w    io.Writer
	rows reflect.Value
	cols []csvColumn
	lang string
	loc  *time.Location
}

func newCSVWriter(w io.Writer, rows any, lang string, loc *time.Location) (*csvWriter, error) {
	rv := reflect.ValueOf(rows)
	elem, err := csvElemType(rv)
	if err != nil {
		return nil, err
	}
	cols := csvColumns(elem)
	if len(cols) == 0 {
		return nil, fmt.Errorf("trilha: CSV: %s has no exported field to write", elem)
	}
	return &csvWriter{w: w, rows: rv, cols: cols, lang: lang, loc: loc}, nil
}

func (x *csvWriter) run() error {
	if _, err := io.WriteString(x.w, bomUTF8); err != nil {
		return err
	}
	cw := csv.NewWriter(x.w)
	cw.Comma = csvComma(x.lang)
	// CRLF is what RFC 4180 says and what Excel on Windows expects; a bare
	// newline is read there as one very long first line.
	cw.UseCRLF = true
	head := make([]string, len(x.cols))
	for i, col := range x.cols {
		head[i] = col.head
	}
	if err := cw.Write(head); err != nil {
		return err
	}
	rec := make([]string, len(x.cols))
	write := func(v reflect.Value) error {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
		}
		for i, col := range x.cols {
			rec[i] = csvCell(v.Field(col.index), x.lang, x.loc)
		}
		return cw.Write(rec)
	}
	switch x.rows.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < x.rows.Len(); i++ {
			if err := write(x.rows.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Chan:
		for {
			v, ok := x.rows.Recv()
			if !ok {
				break
			}
			if err := write(v); err != nil {
				return err
			}
			// Flushing per row is what makes a long export arrive while it is
			// produced instead of after it.
			cw.Flush()
			if err := cw.Error(); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

type csvColumn struct {
	index int
	head  string // the heading in the file
	form  string // the key bind uses, so a message comes back on the right field
}

func csvElemType(rv reflect.Value) (reflect.Type, error) {
	if !rv.IsValid() {
		return nil, errors.New("trilha: CSV needs rows, got nil")
	}
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Chan:
	default:
		return nil, fmt.Errorf("trilha: CSV needs a slice or a channel, got %s", rv.Kind())
	}
	elem := rv.Type().Elem()
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("trilha: CSV needs rows of a struct, got %s", elem.Kind())
	}
	return elem, nil
}

// csvColumns reads the shape of a row once.
func csvColumns(t reflect.Type) []csvColumn {
	var out []csvColumn
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("csv")
		if tag == "-" {
			continue
		}
		head := tag
		if head == "" {
			head = f.Name
		}
		form := f.Tag.Get("form")
		if form == "" || form == "-" {
			form = f.Name
		}
		out = append(out, csvColumn{index: i, head: head, form: form})
	}
	return out
}

// csvCell writes one value the way the person's spreadsheet expects it.
func csvCell(v reflect.Value, lang string, loc *time.Location) string {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if t, ok := v.Interface().(time.Time); ok {
		if t.IsZero() {
			return ""
		}
		if lang == langPT {
			return t.In(loc).Format("02/01/2006 15:04")
		}
		return t.In(loc).Format("2006-01-02 15:04")
	}
	switch v.Kind() {
	case reflect.Bool:
		return csvBool(lang, v.Bool())
	case reflect.Float32, reflect.Float64:
		s := strconv.FormatFloat(v.Float(), 'f', -1, 64)
		if lang == langPT {
			s = strings.Replace(s, ".", ",", 1)
		}
		return s
	case reflect.String:
		return v.String()
	}
	return fmt.Sprint(v.Interface())
}

func csvBool(lang string, b bool) string {
	if lang == langPT {
		if b {
			return "sim"
		}
		return "não"
	}
	if b {
		return "yes"
	}
	return "no"
}

// csvComma is the separator on the way out. A semicolon is not a preference in
// pt-BR: it is what Excel expects there, and a comma-separated file opens as
// one long column.
func csvComma(lang string) rune {
	if lang == langPT {
		return ';'
	}
	return ','
}

// ---- import ----------------------------------------------------------------

// BindCSV reads an uploaded spreadsheet into a slice, and says which cell is
// wrong instead of which file.
//
//	type Node struct {
//		Code string `csv:"code" validate:"required,max=20"`
//		Name string `csv:"name" validate:"required"`
//	}
//
//	up, err := c.File("file", trilha.FileRules{Accept: "text/csv", MaxSize: 5 << 20})
//	if err != nil {
//		return err
//	}
//	defer up.Close()
//	var nodes []Node
//	res, err := trilha.BindCSV(up, &nodes)
//	if err != nil {
//		return trilha.FieldErrors{"file": err.Error()}
//	}
//	if !res.OK() {
//		return c.Render(422, page(res))
//	}
//
// The separator and the BOM are detected, so one handler takes the file Excel
// wrote in Brazil and the one a script wrote anywhere else. The header matches
// by the csv tag, ignoring case and surrounding space, in any order — somebody
// who moved a column has not made a mistake. A heading no field claims is a
// warning; a blank line is skipped.
//
// Each row is validated by the same validate tags as a form, so a rule written
// once holds on the screen and in the import. Only rows that pass are
// appended: after res.OK() the slice is the whole file, and after a failure it
// holds nothing worth saving.
//
// A date is read as dd/mm/yyyy as well as ISO, because that is what a
// Brazilian spreadsheet exports and refusing it would send somebody to a text
// editor with ten thousand dates.
//
//	see: Ctx.CSV, Ctx.File, FieldErrors
func BindCSV(r io.Reader, dst any, rules ...CSVRules) (CSVResult, error) {
	var rule CSVRules
	if len(rules) > 0 {
		rule = rules[0]
	}
	max := rule.MaxRows
	if max <= 0 {
		max = defaultCSVMaxRows
	}

	var res CSVResult
	dv := reflect.ValueOf(dst)
	if dv.Kind() != reflect.Pointer || dv.Elem().Kind() != reflect.Slice {
		return res, fmt.Errorf("trilha: BindCSV needs a pointer to a slice, got %T", dst)
	}
	slice := dv.Elem()
	elem := slice.Type().Elem()
	if elem.Kind() != reflect.Struct {
		return res, fmt.Errorf("trilha: BindCSV needs a slice of a struct, got %s", elem.Kind())
	}
	cols := csvColumns(elem)
	if len(cols) == 0 {
		return res, fmt.Errorf("trilha: BindCSV: %s has no exported field to read", elem)
	}

	br := bufio.NewReader(r)
	if bom, err := br.Peek(3); err == nil && string(bom) == bomUTF8 {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.Comma = rule.Separator
	if cr.Comma == 0 {
		cr.Comma = sniffComma(br)
	}
	// A row with one column too many is a bad row, not a bad file: it is
	// reported on its own line and the rest of the file is still read.
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	head, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return res, errors.New("the file is empty")
		}
		return res, fmt.Errorf("the file could not be read: %w", err)
	}
	at, warns := matchHeader(head, cols)
	res.Warnings = warns
	// A column no row can supply is one message at the top, not the same
	// message on each of ten thousand lines.
	for _, name := range requiredMissing(elem, cols, at) {
		res.Errors = append(res.Errors, CSVError{Line: 1, Column: name, Message: message("csvcolumn", "")})
	}
	if len(res.Errors) > 0 {
		return res, nil
	}

	form := make(map[string][]string, len(cols))
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if !errors.As(err, &pe) {
				return res, fmt.Errorf("the file could not be read: %w", err)
			}
			res.Errors = append(res.Errors, CSVError{Line: pe.Line, Message: message("csvline", "")})
			if len(res.Errors) >= maxCSVErrors {
				res.Warnings = append(res.Warnings, message("csvtoomany", strconv.Itoa(maxCSVErrors)))
				break
			}
			continue
		}
		if blankRow(rec) {
			continue
		}
		// The line as the person's editor numbers it, taken from the reader so
		// a quoted cell with a newline in it does not shift every message
		// after it by one.
		line, _ := cr.FieldPos(0)
		res.Rows++
		if res.Rows > max {
			return res, fmt.Errorf("the file has more than %d rows", max)
		}

		for k := range form {
			delete(form, k)
		}
		for i, col := range cols {
			if j := at[i]; j >= 0 && j < len(rec) {
				form[col.form] = []string{csvIn(rec[j])}
			}
		}
		row := reflect.New(elem)
		if err := validated(row.Interface(), row, form); err != nil {
			res.Errors = append(res.Errors, rowErrors(err, line, cols)...)
			if len(res.Errors) >= maxCSVErrors {
				res.Warnings = append(res.Warnings, message("csvtoomany", strconv.Itoa(maxCSVErrors)))
				break
			}
			continue
		}
		slice = reflect.Append(slice, row.Elem())
	}
	dv.Elem().Set(slice)
	return res, nil
}

// matchHeader pairs each field with the column that carries it, and names the
// headings nobody claimed.
func matchHeader(head []string, cols []csvColumn) (at []int, warns []string) {
	at = make([]int, len(cols))
	for i := range at {
		at[i] = -1
	}
	for j, cell := range head {
		want, taken := csvKey(cell), false
		for i, col := range cols {
			if at[i] < 0 && csvKey(col.head) == want {
				at[i], taken = j, true
				break
			}
		}
		if !taken && strings.TrimSpace(cell) != "" {
			warns = append(warns, message("csvunknown", strings.TrimSpace(cell)))
		}
	}
	return at, warns
}

// requiredMissing is the header check worth doing once: a required field with
// no column at all.
func requiredMissing(elem reflect.Type, cols []csvColumn, at []int) []string {
	var out []string
	for i, col := range cols {
		if at[i] >= 0 {
			continue
		}
		for _, rule := range strings.Split(elem.Field(col.index).Tag.Get("validate"), ",") {
			if strings.TrimSpace(rule) == "required" {
				out = append(out, col.head)
				break
			}
		}
	}
	return out
}

// rowErrors turns what the validator said about a row into cells of the file,
// in file order — a map's order would shuffle the list on every upload.
func rowErrors(err error, line int, cols []csvColumn) []CSVError {
	var fe FieldErrors
	if !errors.As(err, &fe) {
		return []CSVError{{Line: line, Message: err.Error()}}
	}
	out := make([]CSVError, 0, len(fe))
	for _, col := range cols {
		if msg, ok := fe[col.form]; ok {
			out = append(out, CSVError{Line: line, Column: col.head, Message: msg})
		}
	}
	// A whole-row rule (Validate on the struct) names something that is not a
	// column; it still belongs on the line it happened.
	for name, msg := range fe {
		if !knownField(cols, name) {
			out = append(out, CSVError{Line: line, Column: name, Message: msg})
		}
	}
	return out
}

func knownField(cols []csvColumn, form string) bool {
	for _, col := range cols {
		if col.form == form {
			return true
		}
	}
	return false
}

// sniffComma reads the header without consuming it and counts the candidates
// outside quotes. A file with no separator at all is one column, and a comma
// parses that as well as anything.
func sniffComma(br *bufio.Reader) rune {
	line, _ := br.Peek(4096)
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	best, count := ',', 0
	for _, sep := range []rune{',', ';', '\t', '|'} {
		if n := countOutsideQuotes(line, byte(sep)); n > count {
			best, count = sep, n
		}
	}
	return best
}

func countOutsideQuotes(s []byte, sep byte) int {
	n, quoted := 0, false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			quoted = !quoted
		case s[i] == sep && !quoted:
			n++
		}
	}
	return n
}

// csvKey is how two headings are compared: case and surrounding space are not
// a difference anybody meant.
func csvKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func blankRow(rec []string) bool {
	for _, cell := range rec {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// csvIn is the one conversion the form path cannot do for us: a date the way a
// Brazilian spreadsheet writes it. An HTML date input always sends ISO, so
// bind never had to know 08/09/2026 — a file exported from Excel does.
func csvIn(cell string) string {
	s := strings.TrimSpace(cell)
	if len(s) < 10 || s[2] != '/' || s[5] != '/' {
		return s
	}
	d, m, y := s[0:2], s[3:5], s[6:10]
	if !allDigits(d) || !allDigits(m) || !allDigits(y) {
		return s
	}
	iso := y + "-" + m + "-" + d
	if rest := strings.TrimSpace(s[10:]); rest != "" {
		return iso + "T" + rest
	}
	return iso
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
