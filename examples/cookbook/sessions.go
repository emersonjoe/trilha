package cookbook

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// SessionCookie carries the user id, signed by the app's secret. What is
// inside it is not secret — it is the id, and anyone may read their own —
// but it cannot be changed without the key.
const SessionCookie = "session"

// SessionTTL is how long a session lasts without logging in again.
const SessionTTL = 12 * time.Hour

// ErrBadCredentials is the only answer a failed login gives.
var ErrBadCredentials = errors.New("wrong e-mail or password")

// User is the row the session points at.
type User struct {
	ID    int64
	Email string
	Hash  string
}

// CheckPassword compares a password with the stored hash. The standard
// library has no password hash worth using, so this is where your app plugs
// in bcrypt or argon2; the default refuses everyone, which is the safe way
// to notice it was never wired.
var CheckPassword = func(hash, password string) bool { return false }

// dummyHash keeps the comparison cost the same for an e-mail that does not
// exist: without it, the time the answer takes says which e-mails are real.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

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

// Logout clears the cookie. Nothing is stored on the server, so there is
// nothing else to forget.
func Logout(c *trilha.Ctx) error {
	c.ClearCookie(SessionCookie)
	return c.Redirect("/")
}

// safeNext refuses a destination that leaves the site: ?next= is how an
// open redirect gets into a login page.
func safeNext(next string) string {
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return u.Path
}

// UserKey is where the middleware leaves the user for the handlers.
const UserKey = "user"

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

// RequireUser sends anyone the middleware did not recognise to the login
// page, remembering where they were going.
func RequireUser(c *trilha.Ctx, next trilha.Next) error {
	if _, ok := c.Get(UserKey).(User); !ok {
		return c.Redirect("/login?next=" + url.QueryEscape(c.Request().URL.Path))
	}
	return next()
}

// CurrentUser is what a handler calls. The zero User means nobody is
// logged in, so a page can ask without checking twice.
func CurrentUser(c *trilha.Ctx) User {
	u, _ := c.Get(UserKey).(User)
	return u
}

// UserByID reads the user the session points at, on every request. That is
// one indexed query for the ability to disable an account and have it take
// effect now, instead of when the cookie expires.
func UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := DB.QueryRowContext(ctx, `SELECT id, email, password_hash FROM users WHERE id = $1 AND active`, id).
		Scan(&u.ID, &u.Email, &u.Hash)
	return u, err
}

// FlashCookie holds a message that survives exactly one redirect.
const FlashCookie = "flash"

// Flash writes the message the next page will show.
func Flash(c *trilha.Ctx, msg string) error {
	return c.SetSigned(FlashCookie, msg, 5*time.Minute)
}

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
