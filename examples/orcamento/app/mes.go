package app

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
)

// mes reads ?mes=2026-09 (validated) or the default month.
func mes(c *trilha.Ctx) string {
	if m := c.Query("mes"); m != "" {
		if _, err := time.Parse("2006-01", m); err == nil {
			return m
		}
	}
	return plano.MesPadrao()
}
