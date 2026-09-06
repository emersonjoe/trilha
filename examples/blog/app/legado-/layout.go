// Package legado is the shell of an app that already existed in
// html/template, with the new screens written in h — the halfway state of a
// migration, where the shell is old and the inside is new. The folder name
// ends with "-", so the group adds no URL segment.
package legado

import (
	"embed"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/tmpl"
)

//go:embed casca.html
var files embed.FS

// The shell is prepared once, at package load: html/template only clones a set
// that has not executed yet.
var casca = tmpl.Wrap(tmpl.Must(files, "*.html"), "casca", "conteudo")

type dados struct{ Titulo, Nonce, CSRF string }

// pagina builds the template data from the *http.Request alone — which is all a
// renderer that does not know the *Ctx receives.
func pagina(r *http.Request) dados {
	return dados{
		Titulo: "Área migrada",
		Nonce:  trilha.NonceFrom(r),
		CSRF:   trilha.CSRFTokenFrom(r),
	}
}

// Layout puts the h body inside the old shell.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return casca.Node(pagina(c.Request()), children), nil
}
