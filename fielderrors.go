package trilha

import (
	"sort"
	"strings"
)

// FieldErrors collects validation messages by form field. Returned from a
// handler it answers 422: JSON with a "fields" object on API routes, or the
// error page on pages — though a form usually re-renders itself with
// Ctx.Render and shows each message next to its field.
type FieldErrors map[string]string

// Add records a message for field (the first one wins).
func (e FieldErrors) Add(field, msg string) {
	if _, ok := e[field]; !ok {
		e[field] = msg
	}
}

// Has reports whether field has an error.
func (e FieldErrors) Has(field string) bool { _, ok := e[field]; return ok }

// Get returns the message for field, or "".
func (e FieldErrors) Get(field string) string { return e[field] }

// Any reports whether there is at least one error.
func (e FieldErrors) Any() bool { return len(e) > 0 }

// Error joins the messages, sorted by field, for logs and API responses.
func (e FieldErrors) Error() string {
	if len(e) == 0 {
		return "trilha: validation failed"
	}
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("trilha: validation failed:")
	for _, k := range keys {
		b.WriteString(" " + k + ": " + e[k] + ";")
	}
	return strings.TrimSuffix(b.String(), ";")
}

// OrNil returns e as an error, or nil when empty, for `return errs.OrNil()`.
func (e FieldErrors) OrNil() error {
	if len(e) == 0 {
		return nil
	}
	return e
}
