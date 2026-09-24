package app

import (
	"github.com/emersonjoe/trilha"
)

// Setup runs once, before the first request.
func Setup(a *trilha.App) error {
	// trilha:add login
	trilha.Provide(a, usuarios.New(a.Logger()))
	return nil
}
