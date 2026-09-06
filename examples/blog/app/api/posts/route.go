// Package posts exposes the JSON API at /api/posts.
package posts

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// GET lists posts.
//
// Every /api route goes through the rate limit in app/api/middleware.go, and
// a middleware is not something the generator can read: that 429 is only in
// the document because this line puts it there.
//
// openapi:response 429
func GET(c *trilha.Ctx) error {
	return c.JSON(http.StatusOK, trilha.Use[*posts.Store](c).All())
}

// POST creates a post from JSON {"title": "...", "body": "..."}.
//
// openapi:response 429
func POST(c *trilha.Ctx) error {
	var in struct {
		Title string `json:"title" validate:"required,max=80"`
		Body  string `json:"body"`
	}
	// The same tags the form uses: BindJSON answers 422 with {"fields": ...}.
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	// Um erro que o cliente precisa entender sem ler documentação nenhuma: o
	// type aponta para a página que explica, e o membro de extensão diz qual
	// slug bateu. É isso que o problem+json compra.
	st := trilha.Use[*posts.Store](c)
	slug := posts.Slugify(in.Title)
	if _, existe := st.Get(slug); existe {
		return &trilha.Problem{
			Type:   "https://trilha.dev/probs/slug-em-uso",
			Title:  "Slug já existe",
			Status: http.StatusConflict,
			Detail: "Já existe um post com esse título.",
			Extra:  map[string]any{"slug": slug},
		}
	}
	p := st.Create(in.Title, in.Body)
	c.Header("Location", "/api/posts/"+p.Slug)
	return c.JSON(http.StatusCreated, p)
}
