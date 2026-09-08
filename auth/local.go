package auth

import (
	"errors"

	"github.com/emersonjoe/trilha"
)

// errNoProvider is what the OIDC half answers in an app that has no provider.
var errNoProvider = errors.New("auth: this Auth was built by Sessions, which has no provider: use Login after checking the password yourself")

// Sessions builds the same flow without a provider, for an app whose users are
// a table of its own: e-mail, password hash and role. Everything around the
// session — Require, RequireRole, Optional, User, the Store, the rotation of
// the identifier, the idle window and the redirect carrying next — is the code
// the OIDC login already uses; what changes is who says the person is who they
// claim to be.
//
//	sessions := auth.Sessions(auth.Options{Store: store, LoginPath: "/entrar"})
func Sessions(o Options) *Auth { return New(nil, o) }

// Login creates the session for a user the app has already authenticated, and
// redirects — to the next it was asked for, or to AfterLogin. Checking the
// password is the app's job: only the app knows where its users live.
//
//	func POST(c *trilha.Ctx) error {
//		u, err := users.Verify(c.Context(), c.Form("email"), c.Form("password"))
//		if err != nil {
//			return c.Render(422, form(trilha.FieldErrors{"password": "wrong e-mail or password"}))
//		}
//		return sessions.Login(c, &auth.User{Subject: u.ID, Email: u.Email,
//			Roles: []string{u.Role}, Extra: map[string]string{"api_token": u.JWT}})
//	}
func (a *Auth) Login(c *trilha.Ctx, u *User) error {
	if u == nil || u.Subject == "" {
		return errors.New("auth: Login needs a User with a Subject")
	}
	if a.opts.OnLogin != nil {
		if err := a.opts.OnLogin(c, u); err != nil {
			return err
		}
	}
	if err := a.login(c, u); err != nil {
		return err
	}
	dest := a.opts.AfterLogin
	if s := safeNext(c.Query("next")); s != "" {
		dest = s
	} else if s := safeNext(c.Form("next")); s != "" {
		dest = s
	}
	return c.Redirect(dest)
}

// RequireFunc guards a subtree with a rule the app writes. RequireRole answers
// the common question; a matrix of module and level, a tenant, an owner of the
// record are all the same shape and none of them fits in a list of role names.
//
//	middleware.go: var Middleware = sessions.RequireFunc(func(u *auth.User, c *trilha.Ctx) bool {
//		return u.Extra["tenant"] == c.Param("tenant")
//	})
//
// Anonymous never reaches the predicate: it is sent to the login first, as
// Require does.
func (a *Auth) RequireFunc(pred func(*User, *trilha.Ctx) bool) trilha.MiddlewareFunc {
	return func(c *trilha.Ctx, next trilha.Next) error {
		u, err := a.Session(c)
		if err != nil {
			return a.challenge(c)
		}
		if pred == nil || !pred(u, c) {
			return trilha.Errorf(403, "forbidden")
		}
		remember(c, u)
		return next()
	}
}
