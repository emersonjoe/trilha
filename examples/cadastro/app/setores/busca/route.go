// Package setoresbusca is the other way into a hierarchy: quem sabe o código e
// não sabe onde ele mora.
package setoresbusca

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/setores"
	"github.com/emersonjoe/trilha/ui"
)

// GET /setores/busca?q=finan
//
// openapi:query q string  código ou nome, em qualquer parte da árvore
// openapi:tag setores
func GET(c *trilha.Ctx) error {
	achados := setores.Buscar(c.Query("q"))
	nos := make([]ui.TreeNode, 0, len(achados))
	for _, s := range achados {
		// Achatado e com o caminho: um resultado fora de contexto só quer
		// dizer alguma coisa com a ancestralidade ao lado.
		nos = append(nos, ui.TreeNode{
			Value: s.Codigo,
			Label: setores.Rotulo(s),
			Leaf:  true,
			Path:  setores.Caminho(s.Codigo),
		})
	}
	return c.HTML(http.StatusOK, ui.TreeItems(nos, ui.TreeOpts{Name: "setor"}))
}
