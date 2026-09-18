package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// localeApp answers every request with the locale it negotiated.
func localeApp(cfg Config) *App {
	cfg.Logger = quiet()
	a := New(cfg)
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(200, c.Locale()) },
	}})
	return a
}

// #270 — o locale é do pedido, não do processo: a preferência gravada, o
// `?lang`, o cookie que ele deixou, o Accept-Language e o padrão, nessa ordem.
func TestLocaleNegotiation(t *testing.T) {
	locales := []string{"pt-BR", "ht", "fr", "en"}
	cases := []struct {
		name   string
		cfg    Config
		url    string
		header string
		cookie string
		want   string
	}{
		{name: "default when nothing says otherwise", cfg: Config{Locales: locales}, url: "/", want: "pt-BR"},
		{name: "accept-language by q-value", cfg: Config{Locales: locales}, url: "/",
			header: "de;q=0.9, ht;q=0.3, fr;q=0.8", want: "fr"},
		{name: "accept-language by base language", cfg: Config{Locales: locales}, url: "/",
			header: "fr-CA", want: "fr"},
		{name: "accept-language ignores what is not offered", cfg: Config{Locales: locales}, url: "/",
			header: "de, es", want: "pt-BR"},
		{name: "cookie beats the header", cfg: Config{Locales: locales}, url: "/",
			header: "fr", cookie: "ht", want: "ht"},
		{name: "query beats the cookie", cfg: Config{Locales: locales}, url: "/?lang=en",
			header: "fr", cookie: "ht", want: "en"},
		{name: "query is matched without case", cfg: Config{Locales: locales}, url: "/?lang=PT-br", want: "pt-BR"},
		{name: "the session preference beats everything",
			cfg:  Config{Locales: locales, LocaleOf: func(*Ctx) string { return "fr" }},
			url:  "/?lang=en",
			want: "fr"},
		{name: "an empty preference does not win",
			cfg:  Config{Locales: locales, LocaleOf: func(*Ctx) string { return "" }},
			url:  "/?lang=en",
			want: "en"},
		{name: "a preference nobody offers does not win",
			cfg:    Config{Locales: locales, LocaleOf: func(*Ctx) string { return "de" }},
			url:    "/",
			header: "ht",
			want:   "ht"},
		{name: "without Locales it is Config.Locale", cfg: Config{Locale: "pt-BR"}, url: "/?lang=en",
			header: "fr", want: "pt-BR"},
		{name: "without Locales and without Locale it is English", cfg: Config{}, url: "/",
			header: "fr", want: "en"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			if tc.header != "" {
				req.Header.Set("Accept-Language", tc.header)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: localeCookie, Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			localeApp(tc.cfg).Handler().ServeHTTP(rec, req)
			if got := rec.Body.String(); got != tc.want {
				t.Fatalf("locale = %q, want %q", got, tc.want)
			}
		})
	}
}

// A escolha feita em `?lang` tem que valer para o clique seguinte, que não
// carrega query nenhuma: ela volta num cookie.
func TestLocaleQueryIsRemembered(t *testing.T) {
	rec := httptest.NewRecorder()
	a := localeApp(Config{Locales: []string{"pt-BR", "ht"}})
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/?lang=ht", nil))
	var ck *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == localeCookie {
			ck = c
		}
	}
	if ck == nil {
		t.Fatalf("?lang did not leave a cookie: %v", rec.Result().Cookies())
	}
	if ck.Value != "ht" || ck.Path != "/" || !ck.HttpOnly || ck.MaxAge < 300*24*3600 {
		t.Fatalf("cookie da preferência: %+v", ck)
	}
	// E o clique seguinte, sem query, continua em crioulo.
	next := httptest.NewRequest("GET", "/", nil)
	next.AddCookie(ck)
	rec2 := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec2, next)
	if rec2.Body.String() != "ht" {
		t.Fatalf("o cookie não foi lido: %q", rec2.Body.String())
	}
}

// Um handler que decide sozinho manda em tudo que vem depois dele, e não
// grava cookie nenhum.
func TestSetLocale(t *testing.T) {
	a := New(Config{Locales: []string{"en", "fr"}, Logger: quiet()})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error {
			c.SetLocale("fr")
			return c.Text(200, c.Locale())
		},
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Body.String() != "fr" {
		t.Fatalf("SetLocale não valeu: %q", rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("SetLocale gravou cookie: %v", rec.Result().Cookies())
	}
}

// O cabeçalho é lido uma vez por requisição: o resultado fica no Ctx.
func TestLocaleIsResolvedOnce(t *testing.T) {
	calls := 0
	a := New(Config{Locales: []string{"en", "fr"}, Logger: quiet(),
		LocaleOf: func(*Ctx) string { calls++; return "fr" }})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error {
			return c.Text(200, c.Locale()+c.Locale()+c.Locale())
		},
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if !strings.HasPrefix(rec.Body.String(), "fr") || calls != 1 {
		t.Fatalf("resolveu %d vezes: %q", calls, rec.Body.String())
	}
}
