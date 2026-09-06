---
title: Um app na frente de uma API existente
description: Encaminhar /api/ para um serviço que já existe, com um login próprio.
---

A forma da maioria das migrações: os usuários são seus — e-mail, hash de senha, papel — e a
API continua onde está. Duas peças do Trilha se encontram aqui, e se encontram porque o
proxy precisa levar o que o login guardou.

O `examples/local-login` é esta receita, inteira e rodando.

## 1. A sessão, sem provedor

`auth.Sessions` é o mesmo `*Auth` do fluxo OIDC, sem provedor. `Require()`, `RequireRole()`,
a `Store`, a rotação do identificador e a janela idle são o código que já estava lá; o que
muda é quem diz que a pessoa é quem diz ser.

```go
var Flow = auth.Sessions(auth.Options{
	Store:      auth.NewMemoryStore(),
	Idle:       30 * time.Minute,
	LoginPath:  "/entrar",
	AfterLogin: "/painel",
})
```

`User.Extra` é o que esta sessão carrega e para o que o OIDC não tem claim — o token que a
API espera, o tenant, o plano. Ele viaja por onde o resto da sessão viaja, então deixe-o
pequeno e nunca ponha uma senha ali:

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

## 2. A senha, conferida na sua própria tabela

`auth.CheckPBKDF2` lê o formato que o Django e o `hashlib` gravam —
`pbkdf2_sha256$iteracoes$sal$hash` — então a tabela de usuários que já existe continua
valendo, hashes e tudo. `auth.HashPBKDF2` grava um novo com 600 mil iterações.

Responda a mesma coisa para e-mail errado e para senha errada, e demore o mesmo tanto:
dizer qual dos dois estava errado conta a um atacante quem tem conta.

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

O handler é um formulário como outro qualquer:

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

## 3. A API, na mesma origem

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

O proxy responde fora da cadeia de middleware, então ninguém pôs o usuário no contexto da
requisição: leia a sessão do cookie.

```go
func Token(c *trilha.Ctx) string {
	u, err := Flow.Session(c)
	if err != nil {
		return ""
	}
	return u.Extra["api_token"]
}
```

## O que vem de graça

- `POST /api/*` exige o token do CSRF, como toda escrita do app.
- O `Authorization` do próprio navegador nunca chega à API — só o da sessão.
- Um upload de 200 MB e um PDF voltando passam em stream, por cima do `MaxBodyBytes` e do
  write deadline.
- Um `route.go` que você escreva em `/api/documentos` responde antes do proxy, então a API
  vai para Go endpoint a endpoint e o front não percebe.

O `trilha audit` avisa se o alvo é `http://` puro para outro host, se o upstream não injeta
nada num app com login, ou se o login está sem limite de taxa.

Veja [Upstreams](/pt/referencia/upstreams) e [Auth](/pt/referencia/auth) para os detalhes.
