// Package setoresnos answers the children of one node. É a fonte da árvore: o
// ui.Tree pede ?parent=100.1 na primeira vez que alguém abre aquele ramo.
package setoresnos

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/setores"
	"github.com/emersonjoe/trilha/ui"
)

// GET /setores/nos?parent=100.1
//
// openapi:query parent string  o nó cujos filhos se quer; vazio traz as raízes
// openapi:tag setores
func GET(c *trilha.Ctx) error {
	filhos := setores.Filhos(c.Query("parent"))
	return c.HTML(http.StatusOK, ui.TreeItems(no(filhos), ui.TreeOpts{Name: "setor"}))
}

// no converte o domínio no que a árvore desenha. É a única tradução, e ela
// mora do lado da tela: o pacote setores não conhece o kit.
func no(lista []setores.Setor) []ui.TreeNode {
	out := make([]ui.TreeNode, 0, len(lista))
	for _, s := range lista {
		out = append(out, ui.TreeNode{
			Value: s.Codigo,
			Label: setores.Rotulo(s),
			Leaf:  setores.Folha(s.Codigo),
		})
	}
	return out
}
