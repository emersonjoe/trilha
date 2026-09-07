package health

import (
	"net/http"

	"github.com/emersonjoe/trilha"
)

// /api/health, ported from app/api/health/route.ts.
//
// Source: 2 lines, server component.
func GET(c *trilha.Ctx) error {
	return trilha.Errorf(http.StatusNotImplemented, "GET /api/health has not been ported yet")
}
