package documents

import (
	"net/http"

	"github.com/emersonjoe/trilha"
)

// /api/documents, ported from app/api/documents/route.ts.
//
// Source: 15 lines, server component.
func GET(c *trilha.Ctx) error {
	return trilha.Errorf(http.StatusNotImplemented, "GET /api/documents has not been ported yet")
}

func POST(c *trilha.Ctx) error {
	return trilha.Errorf(http.StatusNotImplemented, "POST /api/documents has not been ported yet")
}

func DELETE(c *trilha.Ctx) error {
	return trilha.Errorf(http.StatusNotImplemented, "DELETE /api/documents has not been ported yet")
}
