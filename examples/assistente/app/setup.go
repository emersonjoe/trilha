package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/assistente/internal/config"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

// Setup runs once before the server starts.
func Setup(a *trilha.App) error {
	// A seção sobe com o app: sem store ela vive em memória e o log diz isso
	// uma vez — configuração que esquece no restart é surpresa ruim de ter em
	// produção, e o aviso é mais barato que descobrir depois.
	if err := config.Cfg.Bind(a, nil); err != nil {
		return err
	}
	a.Logger().Info("assistente: modelo", "base_url", ferramentas.Client.BaseURL, "model", ferramentas.Client.Model)
	// The chat streams for a while: allow long responses on /api/chat.
	return nil
}
