// Package cidades serves the dependent select: GET /api/cidades?uf=SP.
package cidades

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
)

// GET returns the cities of a state as a JSON array.
func GET(c *trilha.Ctx) error {
	list, ok := clientes.Cidades[c.Query("uf")]
	if !ok {
		return trilha.Errorf(http.StatusNotFound, "UF desconhecida")
	}
	c.Header("Cache-Control", "public, max-age=3600")
	return c.JSON(http.StatusOK, list)
}
