// A route.go that declares its own Kind still wins over the branch.
package propria

import "github.com/emersonjoe/trilha"

var Kind = trilha.KindPage

func POST(c *trilha.Ctx) error { return c.Text(200, "ok") }
