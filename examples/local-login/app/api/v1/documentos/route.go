// Package apiv1documentos is what a key can read.
package apiv1documentos

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// GET answers the list. The caller is a key, so the trail says so by itself.
//
// openapi:tag documentos
func GET(c *trilha.Ctx) error {
	u := sessao.Chaves.User(c)
	c.Audit("documentos.listou", "todos")
	return c.JSON(http.StatusOK, map[string]any{
		"chave":      u.Name,
		"escopos":    u.Roles,
		"documentos": []string{"contrato-2026.pdf", "nota-fiscal-9.pdf"},
	})
}
