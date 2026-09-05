package manifest

import "github.com/emersonjoe/trilha"

func GET(c *trilha.Ctx) error { return c.JSON(200, map[string]string{"name": "x"}) }
