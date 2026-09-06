// Package report exports the items as CSV. The directory has a dot, so the
// package name differs from it.
package report

import (
	"net/http"

	"github.com/emersonjoe/trilha"
)

// GET writes one line per item.
//
// openapi:query mes string  competence, in AAAA-MM
// openapi:tag report
func GET(c *trilha.Ctx) error {
	if c.Query("mes") == "" {
		return trilha.Errorf(http.StatusBadRequest, "mes is required")
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Writer().WriteHeader(http.StatusOK)
	return nil
}
