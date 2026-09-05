package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Setup lê a configuração do provedor uma vez, antes de servir.
func Setup(a *trilha.App) error {
	sso.Configure()
	a.Values()["site"] = "Trilha SSO"
	return nil
}
