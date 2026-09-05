package trilha

import (
	"crypto/rand"
	"encoding/base64"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// Off disables a security header when assigned to its field.
const Off = "off"

// Security configures the hardening headers sent with every response. The
// zero value means "defaults"; set a field to Off to drop that header.
type Security struct {
	// CSP is the Content-Security-Policy. Empty = default policy with a
	// per-request nonce for scripts; the text may contain {nonce}.
	CSP string
	// CSPExtra adds sources to directives of the default policy, e.g.
	// {"style-src": {"https://fonts.googleapis.com"}}.
	CSPExtra map[string][]string
	// HSTS is sent only over HTTPS (TLS or a trusted proxy saying so).
	HSTS string
	// PermissionsPolicy restricts browser features.
	PermissionsPolicy string
	// COOP is Cross-Origin-Opener-Policy.
	COOP string
	// FrameOptions is X-Frame-Options.
	FrameOptions string
	// Referrer is Referrer-Policy.
	Referrer string
}

const (
	defaultHSTS        = "max-age=31536000; includeSubDomains"
	defaultPermissions = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"
	defaultCOOP        = "same-origin"
	defaultFrame       = "DENY"
	defaultReferrer    = "strict-origin-when-cross-origin"
)

// defaultCSP lists the default policy directives in order.
var defaultCSP = [][2]string{
	{"default-src", "'self'"},
	{"script-src", "'self' 'nonce-{nonce}'"},
	{"style-src", "'self' 'unsafe-inline'"},
	{"img-src", "'self' data:"},
	{"font-src", "'self'"},
	{"connect-src", "'self'"},
	{"frame-ancestors", "'none'"},
	{"base-uri", "'self'"},
	{"form-action", "'self'"},
}

func pick(v, def string) string {
	switch v {
	case "":
		return def
	case Off:
		return ""
	}
	return v
}

// csp renders the policy for one request.
func (s *Security) csp(nonce string) string {
	if s.CSP == Off {
		return ""
	}
	if s.CSP != "" {
		return strings.ReplaceAll(s.CSP, "{nonce}", nonce)
	}
	seen := map[string]bool{}
	parts := make([]string, 0, len(defaultCSP)+len(s.CSPExtra))
	for _, d := range defaultCSP {
		seen[d[0]] = true
		v := d[1]
		if extra := s.CSPExtra[d[0]]; len(extra) > 0 {
			v += " " + strings.Join(extra, " ")
		}
		parts = append(parts, d[0]+" "+v)
	}
	var extraDirs []string
	for d := range s.CSPExtra {
		if !seen[d] {
			extraDirs = append(extraDirs, d)
		}
	}
	sort.Strings(extraDirs)
	for _, d := range extraDirs {
		parts = append(parts, d+" "+strings.Join(s.CSPExtra[d], " "))
	}
	return strings.ReplaceAll(strings.Join(parts, "; "), "{nonce}", nonce)
}

// applySecurity writes the hardening headers for a request.
func (a *App) applySecurity(c *Ctx) {
	h := c.w.Header()
	s := &a.cfg.Security
	h.Set("X-Content-Type-Options", "nosniff")
	if v := pick(s.FrameOptions, defaultFrame); v != "" {
		h.Set("X-Frame-Options", v)
	}
	if v := pick(s.Referrer, defaultReferrer); v != "" {
		h.Set("Referrer-Policy", v)
	}
	if v := pick(s.PermissionsPolicy, defaultPermissions); v != "" {
		h.Set("Permissions-Policy", v)
	}
	if v := pick(s.COOP, defaultCOOP); v != "" {
		h.Set("Cross-Origin-Opener-Policy", v)
	}
	if v := s.csp(c.Nonce()); v != "" {
		h.Set("Content-Security-Policy", v)
	}
	if c.isSecure() {
		if v := pick(s.HSTS, defaultHSTS); v != "" {
			h.Set("Strict-Transport-Security", v)
		}
	}
}

// Security returns the security settings for adjustment in Setup.
func (a *App) Security() *Security { return &a.cfg.Security }

// Nonce returns the per-request CSP nonce for inline scripts.
func (c *Ctx) Nonce() string {
	if c.nonce == "" {
		var b [16]byte
		_, _ = rand.Read(b[:])
		c.nonce = base64.RawStdEncoding.EncodeToString(b[:])
	}
	return c.nonce
}

// NonceAttr renders the nonce attribute for an inline <script>:
// h.Script(trilha.NonceAttr(c), h.Raw(js)).
func NonceAttr(c *Ctx) h.Node { return h.Attr("nonce", c.Nonce()) }
