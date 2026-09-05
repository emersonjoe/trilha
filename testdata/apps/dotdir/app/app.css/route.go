// A folder with a dot serves a fixed path with an extension; the package
// name cannot be "app.css", so it is declared differently and imported by alias.
package appcss

import "github.com/emersonjoe/trilha"

var Kind = trilha.KindPage

func GET(c *trilha.Ctx) error { return c.Text(200, "body{}") }
