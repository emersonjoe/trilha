package cookbook

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func PWAInvite(c *trilha.Ctx) h.Node {
	return ui.InstallApp(c, ui.InstallAppOpts{Help: "/install"})
}
