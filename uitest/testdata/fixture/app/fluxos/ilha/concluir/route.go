// Package concluir is where the island of /fluxos/ilha/salto posts: it leaves
// a message and redirects, as a route.go of an app would.
package concluir

import "github.com/emersonjoe/trilha"

// POST finishes and sends the browser to the result.
func POST(c *trilha.Ctx) error {
	c.Flash("success", "Finished.")
	return c.Redirect("/fluxos/ilha/feito")
}
