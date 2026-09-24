package recipesllms

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// GET answers one recipe's llms.txt: the priced pack and the recipe's page
// verbatim. The slug carries .txt, because the file is a file.
func GET(c *trilha.Ctx) error {
	slug := strings.TrimSuffix(c.Param("slug"), ".txt")
	out, ok := docs.LLMsRecipe("en", c.Base(), slug)
	if !ok {
		return trilha.ErrNotFound
	}
	return c.Text(200, out)
}
