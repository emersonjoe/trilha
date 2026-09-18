package api

import "github.com/emersonjoe/trilha"

// A JSON answer is not a navigation: the flag belongs to a page.
var Offline = true

func GET(c *trilha.Ctx) error { return c.JSON(200, nil) }
