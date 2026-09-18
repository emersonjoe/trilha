package trilha

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// An application with one language per process has Config.Locale and needs
// nothing else. An application that answers the same URL in five languages
// needs the language to be a property of the request: who is asking, what
// they picked the last time and what their browser says. Config.Locales
// turns Ctx.Locale into that negotiation, in one fixed order, and the whole
// kit — ui.Date, ui.Relative, the labels, c.CSV, c.T — reads it.

// localeCookie remembers the choice made with ?lang= so that the next page,
// which carries no query, is still in the language the person picked.
const localeCookie = "trilha_lang"

// localeYear is how long that cookie lives: a preference is not a session.
const localeYear = 365 * 24 * time.Hour

// Locale is the language of this request, and what the kit's formatters
// write in: "en", "pt-BR", "ht".
//
// With Config.Locales empty it is Config.Locale, one language for the whole
// process ("en" when nothing was configured). With Config.Locales set it is
// negotiated once per request, in this order, and the first supported answer
// wins:
//
//  1. Config.LocaleOf — the preference the application stored, usually in the
//     session (auth.Auth.LocaleOf);
//  2. ?lang= in the URL, which is also written to the trilha_lang cookie so
//     the choice survives the next click;
//  3. the trilha_lang cookie;
//  4. the Accept-Language header, by q-value and by base language ("pt"
//     matches "pt-BR", "fr-CA" matches "fr");
//  5. Config.Locales[0], the default.
//
// "Supported" means listed in Config.Locales, compared without case and by
// base language; what comes back is always the spelling that was configured,
// so a page never has to normalise it.
func (c *Ctx) Locale() string {
	if c.localeDone {
		return c.locale
	}
	c.localeDone = true
	c.locale = c.negotiateLocale()
	return c.locale
}

// SetLocale forces the language of this request, for a handler that has
// decided on its own — a preview of the site in another language, a page
// under /pt. It overrides the negotiation for everything below it and writes
// no cookie: what to remember is the application's call.
func (c *Ctx) SetLocale(l string) {
	c.localeDone = true
	c.locale = l
}

// negotiateLocale is Locale's first read; Locale caches it because a page
// with fifty dates would otherwise parse Accept-Language fifty times.
func (c *Ctx) negotiateLocale() string {
	cfg := &c.app.cfg
	if len(cfg.Locales) == 0 {
		// One language for the process, which is what every app had before
		// Locales existed. The empty Config.Locale has always meant English.
		if cfg.Locale == "" {
			return "en"
		}
		return cfg.Locale
	}
	if cfg.LocaleOf != nil {
		if l, ok := supportedLocale(cfg.Locales, cfg.LocaleOf(c)); ok {
			return l
		}
	}
	if l, ok := supportedLocale(cfg.Locales, c.Query("lang")); ok {
		c.SetCookie(&http.Cookie{
			Name: localeCookie, Value: l, Path: "/",
			MaxAge: int(localeYear / time.Second),
			// The preference is not a credential, but it is read by nobody
			// but the server: HttpOnly costs nothing and keeps it out of a
			// script that gets injected.
			HttpOnly: true, Secure: c.isSecure(), SameSite: http.SameSiteLaxMode,
		})
		return l
	}
	if ck, err := c.Cookie(localeCookie); err == nil {
		if l, ok := supportedLocale(cfg.Locales, ck.Value); ok {
			return l
		}
	}
	if l, ok := acceptLocale(cfg.Locales, c.r.Header.Get("Accept-Language")); ok {
		return l
	}
	return cfg.Locales[0]
}

// supportedLocale answers whether want is one of the offered locales, and
// with which spelling. An exact match wins over a base-language one, so
// "pt-BR" picks pt-BR and not the pt-PT that happens to come first.
func supportedLocale(offered []string, want string) (string, bool) {
	want = strings.TrimSpace(want)
	if want == "" {
		return "", false
	}
	for _, l := range offered {
		if strings.EqualFold(l, want) {
			return l, true
		}
	}
	base := baseLocale(want)
	for _, l := range offered {
		if strings.EqualFold(baseLocale(l), base) {
			return l, true
		}
	}
	return "", false
}

// baseLocale is the language without the region: "pt-BR" is "pt".
func baseLocale(l string) string {
	if i := strings.IndexAny(l, "-_"); i > 0 {
		return l[:i]
	}
	return l
}

// acceptLocale reads Accept-Language — "ht;q=0.9, fr-CA, en;q=0.2" — and
// returns the offered locale the browser wants most. A tag without q counts
// as 1, an unparseable q as 0, and "*" is ignored: it means "anything", and
// anything is the default we would fall back to anyway.
func acceptLocale(offered []string, header string) (string, bool) {
	if strings.TrimSpace(header) == "" {
		return "", false
	}
	type pref struct {
		tag string
		q   float64
		i   int
	}
	var prefs []pref
	for i, part := range strings.Split(header, ",") {
		tag, rest, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "*" {
			continue
		}
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(rest), "q="); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				f = 0
			}
			q = f
		}
		if q <= 0 {
			continue
		}
		prefs = append(prefs, pref{tag: tag, q: q, i: i})
	}
	// Stable by q, then by the order the browser wrote them: two tags with
	// the same weight are as the header lists them.
	sort.SliceStable(prefs, func(a, b int) bool { return prefs[a].q > prefs[b].q })
	for _, p := range prefs {
		if l, ok := supportedLocale(offered, p.tag); ok {
			return l, true
		}
	}
	return "", false
}

// defaultLocale is the first of Config.Locales, or Config.Locale when the
// app never listed any.
func (a *App) defaultLocale() string {
	if len(a.cfg.Locales) > 0 {
		return a.cfg.Locales[0]
	}
	if a.cfg.Locale == "" {
		return "en"
	}
	return a.cfg.Locale
}
