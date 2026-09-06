package api

import "github.com/emersonjoe/trilha"

func GET(c *trilha.Ctx) error { return c.JSON(200, "ok") }

// OPTIONS answers the preflight by hand: the file wins over any policy.
func OPTIONS(c *trilha.Ctx) error { return c.Text(204, "") }
