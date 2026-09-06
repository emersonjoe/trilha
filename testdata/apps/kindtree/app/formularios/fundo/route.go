package fundo

import "github.com/emersonjoe/trilha"

func POST(c *trilha.Ctx) error { return c.Text(200, "ok") }
