// Package sessao builds the session without OIDC and lends it to app/: the
// login checks the password against the users table, and what the session
// carries is what the API upstream wants in Authorization.
package sessao

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/examples/local-login/internal/usuarios"
)

// Flow is the session. It has no provider: nobody outside this app says who
// the person is.
var Flow = auth.Sessions(auth.Options{
	Store:      auth.NewMemoryStore(),
	Idle:       30 * time.Minute,
	LoginPath:  "/entrar",
	AfterLogin: "/painel",
})

// Entrar creates the session for a row of the users table.
func Entrar(c *trilha.Ctx, u usuarios.Usuario) error {
	return Flow.Login(c, &auth.User{
		Subject: u.ID, Email: u.Email, Name: u.Nome, Roles: []string{u.Papel},
		// The token of the API the upstream forwards to. It travels in the
		// session, never in the page.
		Extra: map[string]string{"api_token": u.Token},
	})
}

// Token is what Config.Upstreams injects. It reads the session from the cookie
// rather than from the request context: the proxy answers outside the
// middleware chain, so nobody put the user there.
func Token(c *trilha.Ctx) string {
	u, err := Flow.Session(c)
	if err != nil {
		return ""
	}
	return u.Extra["api_token"]
}
