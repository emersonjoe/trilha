package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
)

// Setup seeds the chart of accounts, budgets and entries.
func Setup(a *trilha.App) error {
	plano.Seed()
	return nil
}
