---
title: Sessões
description: Login com cookie assinado, o usuário atual num middleware, uma mensagem flash que sobrevive a um redirecionamento — e nada guardado no servidor.
---

Uma sessão são duas decisões: o que prova quem você é, e onde essa prova fica. Trilha responde
a primeira — um cookie assinado com o segredo do app, que o navegador não consegue forjar — e
deixa a segunda com você. Esta receita não guarda nada no servidor: o cookie carrega o id do
usuário, e toda requisição lê o usuário do banco.

Isso custa uma consulta indexada por requisição e compra algo que vale mais: desativar uma
conta passa a valer agora, não quando o cookie expirar.

## Entrando

```go
// Login answers the form. The session is written before the redirect,
// because a Set-Cookie on a 303 still reaches the browser.
func Login(c *trilha.Ctx) error {
	u, err := Authenticate(c.Context(), c.Form("email"), c.Form("password"))
	if err != nil {
		return trilha.FieldErrors{"email": "wrong e-mail or password"}
	}
	if err := c.SetSigned(SessionCookie, strconv.FormatInt(u.ID, 10), SessionTTL); err != nil {
		return err
	}
	return c.Redirect(safeNext(c.Form("next")))
}
```

`SetSigned` escreve o cookie com `HttpOnly`, `SameSite=Lax` e `Secure` fora de dev, e assina o
valor com o `Secret` do app. O valor não é criptografado e não precisa ser: ele é o id do
próprio visitante.

```go
// SessionCookie carries the user id, signed by the app's secret. What is
// inside it is not secret — it is the id, and anyone may read their own —
// but it cannot be changed without the key.
const SessionCookie = "session"
```

A conferência da senha é a única coisa que o framework não vai fazer por você — e a biblioteca
padrão também não:

```go
// CheckPassword compares a password with the stored hash. The standard
// library has no password hash worth using, so this is where your app plugs
// in bcrypt or argon2; the default refuses everyone, which is the safe way
// to notice it was never wired.
var CheckPassword = func(hash, password string) bool { return false }

```

No seu app, essa variável aponta para `bcrypt.CompareHashAndPassword` ou `argon2.IDKey`. Aqui
ela recusa todo mundo, para que esquecer de ligá-la falhe fechado.

```go
// Authenticate reads the user and checks the password. One error for "no
// such e-mail" and for "wrong password": telling them apart hands an
// attacker a list of who has an account.
func Authenticate(ctx context.Context, email, password string) (User, error) {
	var u User
	err := DB.QueryRowContext(ctx, `SELECT id, email, password_hash FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.Hash)
	if err != nil {
		u.Hash = dummyHash
	}
	if !CheckPassword(u.Hash, password) || err != nil {
		return User{}, ErrBadCredentials
	}
	return u, nil
}
```

Dois detalhes pagam as suas linhas. O erro único faz com que a página de login não sirva para
descobrir quais e-mails têm conta. O `dummyHash` faz com que ela também não sirva por tempo:

```go
// dummyHash keeps the comparison cost the same for an e-mail that does not
// exist: without it, the time the answer takes says which e-mails are real.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
```

E o redirecionamento depois do login passa por uma checagem, porque `?next=` é o open redirect
clássico — uma página de login que manda a pessoa para outro site depois de ela digitar a
senha:

```go
// safeNext refuses a destination that leaves the site: ?next= is how an
// open redirect gets into a login page.
func safeNext(next string) string {
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return u.Path
}
```

## O usuário atual

O middleware roda para toda rota da pasta em que ele mora e abaixo dela, então
`app/middleware.go` cobre o app inteiro:

```go
// WithUser reads the session and puts the user in the request. It refuses
// nobody: a page that requires a login says so itself, and a page that only
// greets by name works either way.
func WithUser(c *trilha.Ctx, next trilha.Next) error {
	if id, ok := c.Signed(SessionCookie); ok {
		if u, err := UserByID(c.Context(), id); err == nil {
			c.Set(UserKey, u)
		}
	}
	return next()
}
```

Ele não recusa ninguém, de propósito. Uma página que exige login diz isso por conta própria, e
uma página que só cumprimenta pelo nome funciona dos dois jeitos:

```go
// RequireUser sends anyone the middleware did not recognise to the login
// page, remembering where they were going.
func RequireUser(c *trilha.Ctx, next trilha.Next) error {
	if _, ok := c.Get(UserKey).(User); !ok {
		return c.Redirect("/login?next=" + url.QueryEscape(c.Request().URL.Path))
	}
	return next()
}
```

```go
// CurrentUser is what a handler calls. The zero User means nobody is
// logged in, so a page can ask without checking twice.
func CurrentUser(c *trilha.Ctx) User {
	u, _ := c.Get(UserKey).(User)
	return u
}
```

O `User` zerado quer dizer "ninguém", então uma página pode perguntar sem conferir duas vezes.
Ler o usuário é uma consulta, e é a consulta que dá dentes à sessão:

```go
// UserByID reads the user the session points at, on every request. That is
// one indexed query for the ability to disable an account and have it take
// effect now, instead of when the cookie expires.
func UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := DB.QueryRowContext(ctx, `SELECT id, email, password_hash FROM users WHERE id = $1 AND active`, id).
		Scan(&u.ID, &u.Email, &u.Hash)
	return u, err
}
```

:::nota
Quer a sessão num store? Troque o `UserByID` para ler o seu store e mantenha todo o resto. O
cookie continua carregando um id opaco; o que muda é onde esse id é procurado.
:::

## Saindo

```go
// Logout clears the cookie. Nothing is stored on the server, so there is
// nothing else to forget.
func Logout(c *trilha.Ctx) error {
	c.ClearCookie(SessionCookie)
	return c.Redirect("/")
}
```

Não há mais nada para esquecer, o que é a vantagem de uma sessão sem estado — e o limite dela:
um cookie roubado vale até expirar. Se você precisa revogar um, precisa do store.

## Flash

A mensagem que tem que sobreviver a um redirecionamento e depois sumir:

```go
// Flash writes the message the next page will show.
func Flash(c *trilha.Ctx, msg string) error {
	return c.SetSigned(FlashCookie, msg, 5*time.Minute)
}
```

```go
// TakeFlash reads the message and clears it, so a reload does not show it
// again.
func TakeFlash(c *trilha.Ctx) string {
	msg, ok := c.Signed(FlashCookie)
	if !ok {
		return ""
	}
	c.ClearCookie(FlashCookie)
	return msg
}
```

Assinada, para que ninguém coloque um texto próprio na sua página editando um cookie. Lida uma
vez, para que um recarregamento não a mostre de novo.

:::dica
Testar isso não precisa de cliente HTTP: `trilha.WithSigned("session", "42")` escreve uma
sessão válida numa requisição de teste, e `res.Cookie("session")` é como você prova que o
logout limpou o cookie. Veja [Testes](/pt/aprender/testes).
:::
