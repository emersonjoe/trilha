---
title: Sessions
description: Login with a signed cookie, the current user in a middleware, a flash message that survives one redirect — and nothing stored on the server.
---

A session is two decisions: what proves who you are, and where that proof is kept. Trilha
answers the first — a cookie signed with the app's secret, which the browser cannot forge —
and leaves the second to you. This recipe keeps nothing on the server: the cookie carries the
user id, and every request reads the user from the database.

That costs one indexed query per request and buys something worth more: disabling an account
takes effect now, not when the cookie expires.

## Logging in

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

`SetSigned` writes the cookie with `HttpOnly`, `SameSite=Lax` and `Secure` outside dev, and
signs the value with the app's `Secret`. The value is not encrypted and does not need to be:
it is the visitor's own id.

```go
// SessionCookie carries the user id, signed by the app's secret. What is
// inside it is not secret — it is the id, and anyone may read their own —
// but it cannot be changed without the key.
const SessionCookie = "session"
```

The password check is the one thing the framework will not do for you, and neither will the
standard library:

```go
// CheckPassword compares a password with the stored hash. The standard
// library has no password hash worth using, so this is where your app plugs
// in bcrypt or argon2; the default refuses everyone, which is the safe way
// to notice it was never wired.
var CheckPassword = func(hash, password string) bool { return false }

```

In your app, that variable points at `bcrypt.CompareHashAndPassword` or
`argon2.IDKey`. Here it refuses everyone, so that forgetting to wire it fails closed.

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

Two details earn their lines. The single error means the login page cannot be used to find out
which e-mails have accounts. The `dummyHash` means it cannot be used by timing either:

```go
// dummyHash keeps the comparison cost the same for an e-mail that does not
// exist: without it, the time the answer takes says which e-mails are real.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
```

And the redirect after login goes through a check, because `?next=` is the classic open
redirect — a login page that sends people to another site after they type their password:

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

## The current user

The middleware runs for every route in the folder it lives in and below, so `app/middleware.go`
covers the whole app:

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

It refuses nobody on purpose. A page that requires a login says so itself, and a page that only
greets by name works either way:

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

The zero `User` means "nobody", so a page can ask without checking twice. Reading the user is
one query, and it is the query that gives the session its teeth:

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

:::note
Want the session in a store instead? Change `UserByID` to read your store and keep everything
else. The cookie still carries an opaque id; what changes is where the id is looked up.
:::

## Logging out

```go
// Logout clears the cookie. Nothing is stored on the server, so there is
// nothing else to forget.
func Logout(c *trilha.Ctx) error {
	c.ClearCookie(SessionCookie)
	return c.Redirect("/")
}
```

There is nothing else to forget, which is the advantage of a stateless session — and its
limit: a stolen cookie stays valid until it expires. If you need to revoke one, you need the
store.

## Flash

The message that has to survive a redirect and then disappear:

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

Signed, so nobody can put text of their own on your page by editing a cookie. Read once, so a
reload does not show it again.

:::tip
Testing this needs no HTTP client: `trilha.WithSigned("session", "42")` writes a valid session
in a test request, and `res.Cookie("session")` is how you prove a logout cleared it. See
[Testing](/learn/testing).
:::
