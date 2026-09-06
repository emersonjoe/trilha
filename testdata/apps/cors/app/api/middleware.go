package api

import "github.com/emersonjoe/trilha"

func MiddlewareOPTIONS(c *trilha.Ctx, next trilha.Next) error { return next() }
