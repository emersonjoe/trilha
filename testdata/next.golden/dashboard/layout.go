package dashboard

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Layout wraps /dashboard, ported from app/dashboard/layout.tsx.
//
// Source: 4 lines, server component.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return children, nil
}
