// Package busca serves the combobox: GET /cidades/busca?q=camp&uf=SP answers
// the options themselves, as HTML. There is no JSON to agree on and no client
// to keep in sync — the browser only shows what came.
package busca

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
	"github.com/emersonjoe/trilha/ui"
)

// GET answers at most ten cities of the chosen state.
func GET(c *trilha.Ctx) error {
	achadas := clientes.BuscarCidades(c.Query("uf"), c.Query("q"), 10)
	return c.HTML(http.StatusOK, ui.ComboboxOptions(achadas, func(nome string) (string, string) {
		return nome, nome
	}))
}
