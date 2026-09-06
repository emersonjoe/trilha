package arquivos

import "github.com/emersonjoe/trilha"

func GET(c *trilha.Ctx) error { return c.Text(200, "x") }

func HEAD(c *trilha.Ctx) error { return c.Text(200, "") }
