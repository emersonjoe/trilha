package app

import (
	"github.com/emersonjoe/trilha"

	"example.com/s8-conserto/app/contas"
)

// Setup runs once, before the first request: it puts the dependencies where
// the pages find them with trilha.Use[T](c), and registers what has to be
// answering for the app to call itself ready.
func Setup(a *trilha.App) error {
	// The app speaks Portuguese; validation messages follow the locale.
	a.Config().Locales = []string{"pt-BR"}
	trilha.Provide(a, &contas.Store{})
	return nil
}
