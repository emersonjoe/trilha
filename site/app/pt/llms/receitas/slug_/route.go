package ptreceitasllms

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// GET responde o llms.txt de uma receita: o pacote com preço e a página da
// receita na íntegra. O slug vem com .txt, porque o arquivo é um arquivo.
func GET(c *trilha.Ctx) error {
	slug := strings.TrimSuffix(c.Param("slug"), ".txt")
	out, ok := docs.LLMsRecipe("pt", c.Base(), slug)
	if !ok {
		return trilha.ErrNotFound
	}
	return c.Text(200, out)
}
