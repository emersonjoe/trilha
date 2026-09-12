package aichat

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/chatdemo"
)

// Page is the runnable demo of the AI chat recipe (spec 136), pt.
func Page(c *trilha.Ctx) (h.Node, error) { return chatdemo.Page(c, "pt") }
