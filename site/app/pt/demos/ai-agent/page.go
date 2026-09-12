package aiagent

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/agentdemo"
)

// Page is the runnable demo of the AI agent recipe (spec 137), pt.
func Page(c *trilha.Ctx) (h.Node, error) { return agentdemo.Page(c, "pt") }
