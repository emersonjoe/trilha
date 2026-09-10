package assistant

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/assistantdemo"
)

// Page is the runnable ui.Assistant demo (spec 118), en.
func Page(c *trilha.Ctx) (h.Node, error) { return assistantdemo.Page(c, "en") }
