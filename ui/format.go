package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// A date, a size, a duration and a count are not domain: they are the same in
// every application, and every application writes them again. Money is domain
// and stays out — the currency, where the symbol goes and how a negative reads
// are decisions nobody can make for somebody else.
//
// These take a *Ctx and not a package-level locale on purpose. Two applications
// in one process — which is what Ctx.Provide and the embedded-app work exist
// for — would otherwise share one language, and the second one to boot would
// silently change the first. Head, Flashes and DataTable take a Ctx for the
// same kind of reason.

// FormatOpt tunes a formatter.
type FormatOpt func(*formatOpts)

type formatOpts struct {
	relative bool
	dateOnly bool
	timeOnly bool
	decimals int
	hasDec   bool
}

// Relative writes "3 min ago" and keeps the absolute time in the title and in
// the datetime attribute.
//
// Nothing here keeps it up to date: the kit has no clock and does not want one.
// A screen that needs the number to keep moving puts the piece in a ui.Poll,
// which is a decision the page makes and pays for, once.
func Relative() FormatOpt { return func(o *formatOpts) { o.relative = true } }

// DateOnly drops the time of day.
func DateOnly() FormatOpt { return func(o *formatOpts) { o.dateOnly = true } }

// TimeOnly keeps only the time of day.
func TimeOnly() FormatOpt { return func(o *formatOpts) { o.timeOnly = true } }

// Decimals fixes how many decimal places a number shows.
func Decimals(n int) FormatOpt {
	return func(o *formatOpts) { o.decimals, o.hasDec = n, true }
}

func opts(list []FormatOpt) formatOpts {
	var o formatOpts
	for _, f := range list {
		if f != nil {
			f(&o)
		}
	}
	return o
}

// none is what an absent value looks like. It is a dash and not an empty cell
// because an empty cell reads as a bug, and it is not "01/01/0001", which is
// what a zero time.Time prints when nobody checks.
func none() h.Node { return h.Span(h.Class("ui-muted"), h.Text("—")) }

// Date renders a moment as <time>, in the app's zone and language.
//
//	ui.Date(c, doc.CreatedAt)                // 08/09/2026 12:04
//	ui.Date(c, doc.CreatedAt, ui.Relative()) // 3 min ago, absolute in the title
//	ui.Date(c, doc.CreatedAt, ui.DateOnly()) // 08/09/2026
//
// The zero time and a nil *time.Time render as a dash: a row with no date is a
// row with no date, not the first day of year one.
//
//	see: ui.Bytes, ui.Duration, ui.Number
func Date(c *trilha.Ctx, v any, options ...FormatOpt) h.Node {
	t, ok := asTime(v)
	if !ok || t.IsZero() {
		return none()
	}
	o := opts(options)
	loc, lang := zoneOf(c), langOf(c)
	local := t.In(loc)
	full := local.Format(layoutFor(lang, false, false))

	text := full
	switch {
	case o.relative:
		text = relativeText(lang, time.Since(t))
	case o.dateOnly || o.timeOnly:
		text = local.Format(layoutFor(lang, o.dateOnly, o.timeOnly))
	}
	attrs := []h.Node{h.Attr("datetime", t.Format(time.RFC3339))}
	if text != full {
		attrs = append(attrs, h.Attr("title", full))
	}
	return h.Time(append(attrs, h.Text(text))...)
}

// Bytes renders a size in base 10 — kB, MB, GB — which is what the file manager
// of whoever is reading already shows them. The exact number stays in the
// title, because "1.4 MB" is the answer to "how big" and not to "how many
// bytes".
func Bytes(c *trilha.Ctx, n int64) h.Node {
	if n <= 0 {
		return none()
	}
	lang := langOf(c)
	units := []string{"kB", "MB", "GB", "TB", "PB"}
	if n < 1000 {
		return h.Span(h.Text(decimal(lang, float64(n), 0) + " B"))
	}
	value, unit := float64(n), ""
	for _, u := range units {
		value /= 1000
		unit = u
		if value < 1000 {
			break
		}
	}
	places := 1
	if value >= 100 {
		places = 0
	}
	return h.Span(h.Attr("title", decimal(lang, float64(n), 0)+" B"),
		h.Text(decimal(lang, value, places)+" "+unit))
}

// Duration renders an elapsed time the way a person says it: "2 min 13 s",
// "1 h 5 min", "340 ms".
func Duration(c *trilha.Ctx, d time.Duration) h.Node {
	if d <= 0 {
		return none()
	}
	lang := langOf(c)
	switch {
	case d < time.Second:
		return h.Span(h.Text(strconv.FormatInt(d.Milliseconds(), 10) + " ms"))
	case d < time.Minute:
		return h.Span(h.Text(decimal(lang, d.Seconds(), 1) + " s"))
	case d < time.Hour:
		m := int(d / time.Minute)
		s := int(d/time.Second) % 60
		if s == 0 {
			return h.Span(h.Textf("%d min", m))
		}
		return h.Span(h.Textf("%d min %d s", m, s))
	default:
		hh := int(d / time.Hour)
		m := int(d/time.Minute) % 60
		if m == 0 {
			return h.Span(h.Textf("%d h", hh))
		}
		return h.Span(h.Textf("%d h %d min", hh, m))
	}
}

// Number renders a count or a measure with the thousands separator of the
// language: 12.345 in pt-BR, 12,345 in English.
//
//	ui.Number(c, total)                   // 12,345
//	ui.Number(c, price, ui.Decimals(2))   // 1,234.56
//
// Money is not here. The currency, where the symbol goes and how a negative
// reads are the application's to decide; ui.Number(c, v, ui.Decimals(2)) with
// the symbol written beside it is the whole recipe.
//
//	see: ui.Date, ui.Bytes, ui.Duration
func Number(c *trilha.Ctx, v any, options ...FormatOpt) h.Node {
	o := opts(options)
	f, ok := asFloat(v)
	if !ok {
		return none()
	}
	places := 0
	if o.hasDec {
		places = o.decimals
	}
	return h.Span(h.Text(decimal(langOf(c), f, places)))
}

// ---- the two decisions everything above shares ----------------------------

func langOf(c *trilha.Ctx) string {
	if c == nil {
		return "en"
	}
	if strings.EqualFold(c.Locale(), "pt-BR") || strings.EqualFold(c.Locale(), "pt") {
		return "pt-BR"
	}
	return "en"
}

func zoneOf(c *trilha.Ctx) *time.Location {
	if c == nil {
		return time.UTC
	}
	return c.Location()
}

// layoutFor is three layouts per language and no more. A format string in a
// page is what this exists to remove, so offering a fourth would be inviting
// the fifth.
func layoutFor(lang string, dateOnly, timeOnly bool) string {
	if lang == "pt-BR" {
		switch {
		case dateOnly:
			return "02/01/2006"
		case timeOnly:
			return "15:04"
		}
		return "02/01/2006 15:04"
	}
	switch {
	case dateOnly:
		return "Jan 2, 2006"
	case timeOnly:
		return "3:04 PM"
	}
	return "Jan 2, 2006 3:04 PM"
}

// decimal writes a number with the separators of the language.
func decimal(lang string, v float64, places int) string {
	s := strconv.FormatFloat(v, 'f', places, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")

	group, point := ",", "."
	if lang == "pt-BR" {
		group, point = ".", ","
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(group)
		}
		b.WriteRune(r)
	}
	out := b.String()
	if frac != "" {
		out += point + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

// relativeText is the coarse answer, which is the useful one: nobody reading a
// list needs "2 minutes and 13 seconds ago".
func relativeText(lang string, d time.Duration) string {
	future := d < 0
	if future {
		d = -d
	}
	n, unit := 0, ""
	switch {
	case d < time.Minute:
		if lang == "pt-BR" {
			return "agora"
		}
		return "just now"
	case d < time.Hour:
		n, unit = int(d/time.Minute), "min"
	case d < 24*time.Hour:
		n, unit = int(d/time.Hour), "h"
	case d < 30*24*time.Hour:
		n, unit = int(d/(24*time.Hour)), "d"
	case d < 365*24*time.Hour:
		n, unit = int(d/(30*24*time.Hour)), "mo"
	default:
		n, unit = int(d/(365*24*time.Hour)), "y"
	}
	if lang == "pt-BR" {
		pt := map[string]string{"min": "min", "h": "h", "d": "d", "mo": "mês", "y": "ano"}
		if future {
			return fmt.Sprintf("em %d %s", n, pt[unit])
		}
		return fmt.Sprintf("há %d %s", n, pt[unit])
	}
	if future {
		return fmt.Sprintf("in %d%s", n, unit)
	}
	return fmt.Sprintf("%d%s ago", n, unit)
}

func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	}
	return time.Time{}, false
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
