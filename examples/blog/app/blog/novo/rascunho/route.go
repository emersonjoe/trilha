// Package rascunho is the other side of the editor island: what the island
// posts while someone is still writing.
package rascunho

import (
	"net/http"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// rascunho is what the island sends. The rules are the same as the form's, so
// what the island hears back is what the form would have shown.
type rascunho struct {
	Titulo string `json:"titulo" validate:"required,min=3,max=80"`
	Corpo  string `json:"corpo" validate:"required"`
}

// salvo is what it hears back when the draft is good.
type salvo struct {
	Palavras int    `json:"palavras"`
	SalvoEm  string `json:"salvoEm"`
}

// POST takes the draft the island sends. It is a route.go, so it is an API
// route: CSRF applies, and the token the island carries is the one c.Island
// wrote into the element.
func POST(c *trilha.Ctx) error {
	var in rascunho
	if err := c.BindJSON(&in); err != nil {
		// FieldErrors becomes a 422 with the fields inside, which is exactly
		// what island.post turns into IslandInvalid.fields.
		return err
	}
	// A draft is not a post: this example keeps nothing. What matters here is
	// the round trip, and that the answer has the shape of an answer.
	return c.JSON(http.StatusOK, salvo{
		Palavras: len(strings.Fields(in.Corpo)),
		SalvoEm:  time.Now().UTC().Format(time.RFC3339),
	})
}
