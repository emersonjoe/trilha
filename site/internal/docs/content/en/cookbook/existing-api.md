---
title: An app in front of an existing API
description: Forwarding /api/ to a service that already exists, with a login of your own.
---

The shape of most migrations: the users are yours — e-mail, password hash, role — and the
API stays where it is. Two pieces of Trilha meet here, and they meet because the proxy has
to carry what the login stored.

`examples/local-login` is this recipe, whole and running.

## 1. The session, without a provider

`auth.Sessions` is the same `*Auth` the OIDC flow uses, without a provider. `Require()`,
`RequireRole()`, the `Store`, the rotation of the identifier and the idle window are the
code that was already there; what changes is who says the person is who they claim to be.

```go
var Flow = auth.Sessions(auth.Options{
	Store:      auth.NewMemoryStore(),
	Idle:       30 * time.Minute,
	LoginPath:  "/entrar",
	AfterLogin: "/painel",
})
```

`User.Extra` is what this session carries and OIDC has no claim for — the token the API
expects, the tenant, the plan. It travels where the rest of the session travels, so keep it
small and never put a password in it:

```go
func Entrar(c *trilha.Ctx, u usuarios.Usuario) error {
	return Flow.Login(c, &auth.User{
		Subject: u.ID, Email: u.Email, Name: u.Nome, Roles: []string{u.Papel},
		// The token of the API the upstream forwards to. It travels in the
		// session, never in the page.
		Extra: map[string]string{"api_token": u.Token},
	})
}
```

## 2. The password, checked against your own table

`auth.CheckPBKDF2` reads the format Django and `hashlib` write —
`pbkdf2_sha256$iterations$salt$hash` — so the users table that already exists stays valid,
hashes and all. `auth.HashPBKDF2` writes a new one at 600,000 iterations.

Answer the same thing for a wrong e-mail and for a wrong password, and take the same time:
saying which of the two was wrong tells an attacker who has an account.

```go
func (s *Store) Verify(email, senha string) (Usuario, error) {
	s.mu.RLock()
	u, ok := s.rows[strings.ToLower(strings.TrimSpace(email))]
	s.mu.RUnlock()
	if !ok {
		auth.CheckPBKDF2(semUsuario, senha)
		return Usuario{}, ErrCredencial
	}
	if !auth.CheckPBKDF2(u.Hash, senha) {
		return Usuario{}, ErrCredencial
	}
	return u, nil
}
```

The handler is a form like any other:

```go
func POST(c *trilha.Ctx) error {
	email, senha := c.Form("email"), c.Form("senha")
	u, err := trilha.Use[*usuarios.Store](c).Verify(email, senha)
	if err != nil {
		return c.Render(422, formulario(c, email, "E-mail ou senha inválidos."))
	}
	return sessao.Entrar(c, u)
}
```

## 3. The API, on the same origin

```go
func Config(cfg *trilha.Config) {
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
```

The proxy answers outside the middleware chain, so nobody put the user in the request
context: read the session from the cookie.

```go
func Token(c *trilha.Ctx) string {
	u, err := Flow.Session(c)
	if err != nil {
		return ""
	}
	return u.Extra["api_token"]
}
```

## What you get for free

- `POST /api/*` requires the CSRF token, like every other write of the app.
- The browser's own `Authorization` never reaches the API — only the session's.
- A 200 MB upload and a PDF coming back both stream, past `MaxBodyBytes` and the write
  deadline.
- A `route.go` you write at `/api/documents` answers before the proxy, so the API moves to
  Go one endpoint at a time and the front never notices.

`trilha audit` will tell you if the target is plain `http://` to another host, if the
upstream injects nothing in an app that has a login, or if the login has no rate limit.

See [Upstreams](/reference/upstreams) and [Auth](/reference/auth) for the details.
