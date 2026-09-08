// Package sessao builds the session without OIDC and lends it to app/: the
// login checks the password against the users table, and what the session
// carries is what the API upstream wants in Authorization.
package sessao

import (
	"sync"
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

// Policy is the permission matrix of this app, declared once as data. Two
// modules and three roles is small, and it is the point: the same three lines
// answer the middleware, the button and the administration screen, instead of
// each one growing its own `if u.Papel == …`.
var Policy = auth.Policy{
	Modules: []string{"relatorios", "usuarios"},
	Levels:  auth.Levels{"ver", "editar", "administrar"},
	Roles: map[string]auth.Grants{
		"admin":    auth.All("administrar"),
		"analista": {"relatorios": "editar"},
		"leitor":   {"relatorios": "ver"},
	},
}

// Pode is what a page asks before it draws a button. Hiding is cosmetic: the
// rule that holds is the middleware, because a hidden button is still an
// address somebody can type.
func Pode(c *trilha.Ctx, modulo, nivel string) bool {
	return Policy.Can(Flow.User(c), modulo, nivel)
}

// Exige guards a folder or one method of a route.
//
//	var Middleware = sessao.Exige("relatorios", "ver")
//	var MiddlewarePOST = sessao.Exige("relatorios", "editar")
func Exige(modulo, nivel string) trilha.MiddlewareFunc {
	return Flow.RequirePolicy(Policy, modulo, nivel)
}

// BindMatriz reads what the grid posted.
func BindMatriz(c *trilha.Ctx) (map[string]auth.Grants, error) {
	return auth.BindPolicy(c, Policy)
}

// SalvarMatriz swaps the roles of the policy in memory. A real app hands
// auth.PolicyStore a table and calls auth.PolicyFrom again after saving; the
// snapshot is deliberate, so that one request cannot answer twice — allowed at
// the middleware, denied at the button.
func SalvarMatriz(papeis map[string]auth.Grants) {
	mu.Lock()
	defer mu.Unlock()
	Policy.Roles = papeis
}

var mu sync.Mutex
