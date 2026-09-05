package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Analytics describes the cookie-less page counter enabled for the site.
type Analytics struct {
	Provider     string // goatcounter
	Code         string // account code
	ScriptSrc    string
	ScriptOrigin string
	CountURL     string
	CountOrigin  string
}

var codeRe = regexp.MustCompile(`^[a-z0-9-]{2,64}$`)

// ParseAnalytics reads "goatcounter:<code>". Only providers that work without
// cookies and without personal data are accepted.
func ParseAnalytics(s string) (*Analytics, error) {
	provider, code, _ := strings.Cut(strings.TrimSpace(s), ":")
	switch provider {
	case "goatcounter":
		if !codeRe.MatchString(code) {
			return nil, fmt.Errorf("SITE_ANALYTICS: invalid GoatCounter code %q", code)
		}
		return &Analytics{Provider: provider, Code: code,
			ScriptSrc: "https://gc.zgo.to/count.js", ScriptOrigin: "https://gc.zgo.to",
			CountURL: "https://" + code + ".goatcounter.com/count", CountOrigin: "https://" + code + ".goatcounter.com"}, nil
	default:
		return nil, fmt.Errorf("SITE_ANALYTICS: unknown provider %q (use goatcounter:<code>)", provider)
	}
}

func analyticsOf(c *trilha.Ctx) *Analytics {
	an, _ := c.App().Values()["analytics"].(*Analytics)
	return an
}

// AnalyticsScript renders the counter tag when enabled (put it in <head>).
func AnalyticsScript(c *trilha.Ctx) h.Node {
	an := analyticsOf(c)
	if an == nil {
		return h.Nil
	}
	return h.Script(h.Data("goatcounter", an.CountURL), h.Async(), h.Src(an.ScriptSrc))
}

// AnalyticsNote is the privacy note for the footer.
func AnalyticsNote(c *trilha.Ctx) h.Node {
	an := analyticsOf(c)
	if an == nil {
		return h.Nil
	}
	return h.P(h.Small(h.Text(T(c, "analytics.1")),
		h.A(h.Href("https://www.goatcounter.com"), h.Rel("noopener"), h.Text("GoatCounter")), h.Text(T(c, "analytics.2")),
		h.A(h.Href(an.CountOrigin), h.Rel("noopener"), h.Text(T(c, "analytics.public"))), h.Text(".")))
}
