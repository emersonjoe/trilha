package coisas

import "github.com/emersonjoe/trilha"

func GET(c *trilha.Ctx) error { return c.JSON(200, nil) }
