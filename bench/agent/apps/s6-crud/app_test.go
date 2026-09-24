package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/emersonjoe/trilha"
)

// client builds the app the way the server does and drives it in memory: no
// port, no browser, and the cookies of the session kept between requests.
func client(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
}

func TestHomeResponde(t *testing.T) {
	c := client(t)
	c.Get("/").WantStatus(200).WantContains("Início")
}
