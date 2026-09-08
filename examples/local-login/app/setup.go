// Package app is an app whose users are its own and whose API is somewhere
// else: it logs in against a table with PBKDF2 hashes, and forwards /api/ to
// the service that already exists, injecting the session's credential.
package app

import (
	"net/http"
	"os"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/examples/local-login/internal/usuarios"
)

// Config declares the upstream before the app is built. API_URL is where the
// service lives; with the variable empty there is no proxy and /api/ is a 404,
// so the example still runs alone.
func Config(cfg *trilha.Config) {
	// Where the trail of who did what goes. Without this it would go to the
	// log, which is enough to grep and enough to ship with.
	cfg.Audit = sessao.Auditoria
	if api := os.Getenv("API_URL"); api != "" {
		cfg.Upstreams = map[string]trilha.Upstream{
			"/api/": {
				Target: api,
				Headers: func(c *trilha.Ctx, hdr http.Header) {
					if tok := sessao.Token(c); tok != "" {
						hdr.Set("Authorization", "Bearer "+tok)
					}
				},
			},
		}
	}
}

// Setup puts the users table where the pages find it.
func Setup(a *trilha.App) error {
	trilha.Provide(a, usuarios.New())
	return nil
}
