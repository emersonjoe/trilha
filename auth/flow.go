package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// flowTTL is how long the browser has to finish the round trip.
const flowTTL = 10 * time.Minute

// Options tunes the login flow and the session. The zero value is usable.
type Options struct {
	// Scopes requested at the provider (default: openid, profile, email).
	Scopes []string
	// Absolute is the maximum session lifetime (default 8h).
	Absolute time.Duration
	// Idle ends a session left alone for this long (default 30m); zero with
	// IdleOff disables the idle check.
	Idle time.Duration
	// IdleOff disables the idle timeout (kiosks, long forms).
	IdleOff bool
	// CookieName is the session cookie (default "trilha_session").
	CookieName string
	// LoginPath is where Require sends an anonymous browser (default "/entrar").
	LoginPath string
	// AfterLogin is where Callback lands when there is no next (default "/").
	AfterLogin string
	// AfterLogout is where Logout lands when the provider has no end session
	// endpoint (default "/").
	AfterLogout string
	// RoleClaims names extra claims to read roles from.
	RoleClaims []string
	// Store persists sessions; nil keeps them in the signed cookie.
	Store Store
}

// Auth is the configured login flow.
type Auth struct {
	p    *Provider
	opts Options
}

// New builds the flow. It performs no network call: discovery happens on the
// first login, so a provider that is down does not stop the app from starting.
func New(p *Provider, o Options) *Auth {
	if len(o.Scopes) == 0 {
		o.Scopes = []string{"openid", "profile", "email"}
	}
	if o.Absolute == 0 {
		o.Absolute = 8 * time.Hour
	}
	if o.Idle == 0 && !o.IdleOff {
		o.Idle = 30 * time.Minute
	}
	if o.IdleOff {
		o.Idle = 0
	}
	if o.CookieName == "" {
		o.CookieName = "trilha_session"
	}
	if o.LoginPath == "" {
		o.LoginPath = "/entrar"
	}
	if o.AfterLogin == "" {
		o.AfterLogin = "/"
	}
	if o.AfterLogout == "" {
		o.AfterLogout = "/"
	}
	return &Auth{p: p, opts: o}
}

// Provider returns the configured provider.
func (a *Auth) Provider() *Provider { return a.p }

// Start begins the login: it stores state, nonce and the PKCE verifier in
// signed cookies and redirects to the provider.
//
//	// app/entrar/route.go
//	func GET(c *trilha.Ctx) error { return sso.Start(c) }
func (a *Auth) Start(c *trilha.Ctx) error {
	doc, err := a.p.discover(c.Context())
	if err != nil {
		return a.fail(c, err)
	}
	state, nonce, verifier := randomID(), randomID(), randomID()+randomID()
	for name, value := range map[string]string{
		"trilha_oidc_state": state, "trilha_oidc_nonce": nonce, "trilha_oidc_verifier": verifier,
	} {
		if err := c.SetSigned(name, value, flowTTL); err != nil {
			return fmt.Errorf("auth: without TRILHA_SECRET the login cannot keep its state: %w", err)
		}
	}
	if next := safeNext(c.Query("next")); next != "" {
		if err := c.SetSigned("trilha_oidc_next", next, flowTTL); err != nil {
			return err
		}
	}
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"client_id":             {a.p.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {a.p.RedirectURL},
		"scope":                 {strings.Join(a.opts.Scopes, " ")},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
	}
	return c.Redirect(doc.Authorization + sep(doc.Authorization) + q.Encode())
}

// Callback finishes the login: it checks state, exchanges the code, validates
// the ID token and creates the session.
//
//	// app/entrar/retorno/route.go
//	func GET(c *trilha.Ctx) error { return sso.Callback(c) }
func (a *Auth) Callback(c *trilha.Ctx) error {
	defer func() {
		for _, n := range []string{"trilha_oidc_state", "trilha_oidc_nonce", "trilha_oidc_verifier", "trilha_oidc_next"} {
			c.ClearCookie(n)
		}
	}()
	if e := c.Query("error"); e != "" {
		// The description comes from the provider: log it, never render it.
		return a.fail(c, fmt.Errorf("provider refused the login: %s", e))
	}
	want, ok := c.Signed("trilha_oidc_state")
	if !ok {
		return a.fail(c, errors.New("login state missing or expired"))
	}
	if got := c.Query("state"); got == "" || got != want {
		return a.fail(c, errors.New("login state mismatch"))
	}
	verifier, ok := c.Signed("trilha_oidc_verifier")
	if !ok {
		return a.fail(c, errors.New("PKCE verifier missing"))
	}
	code := c.Query("code")
	if code == "" {
		return a.fail(c, errors.New("callback without code"))
	}
	tok, err := a.exchange(c.Context(), code, verifier)
	if err != nil {
		return a.fail(c, err)
	}
	nonce, _ := c.Signed("trilha_oidc_nonce")
	claims, err := a.p.verify(c.Context(), tok.IDToken, nonce)
	if err != nil {
		return a.fail(c, err)
	}
	u := &User{Subject: claims.Subject, Email: claims.Email, Name: claims.Name,
		Roles: a.p.roles(claims, a.opts.RoleClaims)}
	if u.Email == "" {
		if pref, ok := claims.All["preferred_username"].(string); ok {
			u.Email = pref
		}
	}
	if err := a.login(c, u); err != nil {
		return a.fail(c, err)
	}
	dest := a.opts.AfterLogin
	if next, ok := c.Signed("trilha_oidc_next"); ok {
		if s := safeNext(next); s != "" {
			dest = s
		}
	}
	return c.Redirect(dest)
}

// Logout drops the session and, when the provider supports it, ends the
// session at the provider too (RP-Initiated Logout).
//
//	// app/sair/route.go
//	func POST(c *trilha.Ctx) error { return sso.Logout(c) }
func (a *Auth) Logout(c *trilha.Ctx) error {
	a.clear(c)
	doc, err := a.p.discover(c.Context())
	if err != nil {
		return c.Redirect(a.opts.AfterLogout)
	}
	dest, why := a.p.endSession(doc, a.absolute(c, a.opts.AfterLogout))
	if why != "" {
		c.Log().Warn("auth: local logout only", "reason", why)
	}
	if dest == "" {
		return c.Redirect(a.opts.AfterLogout)
	}
	return c.Redirect(dest)
}

type tokenResponse struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

// exchange trades the authorization code for tokens. The client secret goes
// in the POST body, over TLS, and never into a log line or a URL.
func (a *Auth) exchange(ctx context.Context, code, verifier string) (*tokenResponse, error) {
	doc, err := a.p.discover(ctx)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {a.p.RedirectURL},
		"client_id":     {a.p.ClientID},
		"code_verifier": {verifier},
	}
	if a.p.ClientSecret != "" {
		form.Set("client_secret", a.p.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := a.p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("unreadable token response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK || tok.Error != "" {
		return nil, fmt.Errorf("provider refused the code exchange (HTTP %d, %s)", resp.StatusCode, tok.Error)
	}
	if tok.IDToken == "" {
		return nil, errors.New("response without id_token")
	}
	return &tok, nil
}

// fail records the security event and answers without detail.
func (a *Auth) fail(c *trilha.Ctx, err error) error {
	c.Log().Warn("auth: login refused", "err", err.Error())
	return &trilha.HTTPError{Code: http.StatusUnauthorized, Message: "could not authenticate"}
}

// safeNext accepts only a path inside this app: "//evil.com" and
// "https://evil.com" are open redirects waiting to happen.
func safeNext(s string) string {
	if s == "" || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || strings.Contains(s, "\\") {
		return ""
	}
	if u, err := url.Parse(s); err != nil || u.Host != "" || u.Scheme != "" {
		return ""
	}
	return s
}

// absolute turns a local path into an absolute URL for the provider.
func (a *Auth) absolute(c *trilha.Ctx, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if u, err := url.Parse(a.p.RedirectURL); err == nil && u.Host != "" {
		u.Path, u.RawQuery, u.Fragment = path, "", ""
		return u.String()
	}
	return path
}

func sep(u string) string {
	if strings.Contains(u, "?") {
		return "&"
	}
	return "?"
}

// randomID returns 128 bits of randomness, URL-safe.
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("auth: randomness source unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
