package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

// Setup runs once before the server starts.
func Setup(a *trilha.App) error {
	a.Logger().Info("assistente: modelo", "base_url", ferramentas.Client.BaseURL, "model", ferramentas.Client.Model)
	// The chat streams for a while: allow long responses on /api/chat.
	return nil
}
