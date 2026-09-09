package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/scan"
)

// A auditoria precisa enxergar o segredo literal mesmo quando os outros
// argumentos são chamadas com parênteses e vírgulas dentro.
func TestAuthCallsSeparaArgumentos(t *testing.T) {
	src := `
	p := auth.EntraID(os.Getenv("SSO_TENANT"), id, "s3cret", "https://app/cb")
	q := auth.Keycloak(base, realm, os.Getenv("ID"), os.Getenv("SEGREDO"), "http://app/cb")
	r := auth.OIDC(iss, id, secret, "http://localhost:3000/cb")
	s := auth.Cognito(regiao, pool, id, "s3cret", "https://app/cb")
	u := auth.Clerk(frontend, id, "s3cret", "https://app/cb")
	`
	calls := authCalls(src)
	if len(calls) != 5 {
		t.Fatalf("achou %d chamadas: %+v", len(calls), calls)
	}
	byName := map[string]authCall{}
	for _, c := range calls {
		byName[c.name] = c
	}
	entra := byName["EntraID"]
	if got := entra.args[0]; got != `os.Getenv("SSO_TENANT")` {
		t.Errorf("argumento partido ao meio: %q", got)
	}
	if got := entra.args[secretArg("EntraID")]; got != `"s3cret"` {
		t.Errorf("segredo não localizado: %q", got)
	}
	if got := byName["Keycloak"].args[secretArg("Keycloak")]; got != `os.Getenv("SEGREDO")` {
		t.Errorf("posição do segredo do Keycloak: %q", got)
	}
	if n := len(byName["OIDC"].args); n != 4 {
		t.Errorf("OIDC com %d argumentos", n)
	}
	// A 0.11.0 ensinou o secretArg a achar o segredo do Cognito, mas o
	// authCalls não procurava a chamada: a checagem nunca rodava.
	if got := byName["Cognito"].args[secretArg("Cognito")]; got != `"s3cret"` {
		t.Errorf("posição do segredo do Cognito: %q", got)
	}
	if got := byName["Clerk"].args[secretArg("Clerk")]; got != `"s3cret"` {
		t.Errorf("posição do segredo do Clerk: %q", got)
	}
}

// TestAuditoriaDeHost: sem AllowedHosts a auditoria avisa; com o campo no
// fonte ou com a variável de ambiente, o item passa.
func TestAuditoriaDeHost(t *testing.T) {
	acha := func(t *testing.T, cs []check) check {
		t.Helper()
		for _, c := range cs {
			if strings.Contains(c.title, "AllowedHosts") {
				return c
			}
		}
		t.Fatal("a auditoria não olhou o AllowedHosts")
		return check{}
	}
	semCampo := t.TempDir()
	if err := os.WriteFile(filepath.Join(semCampo, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	comCampo := t.TempDir()
	src := "package main\n\nvar cfg = trilha.Config{AllowedHosts: []string{\"exemplo.com\"}}\n"
	if err := os.WriteFile(filepath.Join(comCampo, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := acha(t, runAudit(&project{Root: semCampo}, false)); got.level != "warn" {
		t.Errorf("sem o campo o nível é %q, queria warn", got.level)
	}
	if got := acha(t, runAudit(&project{Root: comCampo}, false)); got.level != "ok" {
		t.Errorf("com o campo o nível é %q", got.level)
	}
	t.Setenv("TRILHA_ALLOWED_HOSTS", "exemplo.com")
	if got := acha(t, runAudit(&project{Root: semCampo}, false)); got.level != "ok" {
		t.Errorf("com a variável de ambiente o nível é %q", got.level)
	}
}

// #77 — o segredo ausente só é crítico no app que assina alguma coisa. Num app
// que nunca chama SetSigned, exigir a variável é ensinar a guardar um segredo
// que não protege nada — e o check parava antes do openapi por causa dele.
func TestAuditoriaDoSegredoOlhaOCodigo(t *testing.T) {
	acha := func(t *testing.T, cs []check) check {
		t.Helper()
		for _, c := range cs {
			if strings.Contains(c.title, "TRILHA_SECRET") {
				return c
			}
		}
		t.Fatal("a auditoria não olhou o TRILHA_SECRET")
		return check{}
	}
	escreve := func(t *testing.T, src string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	semAssinatura := escreve(t, "package main\n\nfunc main() {}\n")
	comAssinatura := escreve(t, "package main\n\nfunc h(c *trilha.Ctx) error { return c.SetSigned(\"s\", \"1\", 0) }\n")
	t.Setenv("TRILHA_SECRET", "")

	if got := acha(t, runAudit(&project{Root: semAssinatura}, false)); got.level != "warn" {
		t.Errorf("num app que não assina nada o nível é %q, queria warn", got.level)
	}
	if got := acha(t, runAudit(&project{Root: comAssinatura}, false)); got.level != "critical" {
		t.Errorf("num app que chama SetSigned o nível é %q, queria critical", got.level)
	}
	// Definido e curto continua crítico nos dois: quem definiu quis usar.
	t.Setenv("TRILHA_SECRET", "curto")
	if got := acha(t, runAudit(&project{Root: semAssinatura}, false)); got.level != "critical" {
		t.Errorf("segredo curto demais é %q", got.level)
	}
}

// Spec 055 (#43): uma escrita que mora num route.go nasce API e não confere o
// token. Num app que também serve páginas isso é quase sempre engano — o mesmo
// formulário, movido de page.go para route.go, passa a aceitar POST de outro
// site sem que nada avise. A auditoria avisa; a herança do Kind cala.
func TestAuditoriaAvisaEscritaSemCSRF(t *testing.T) {
	escreve := func(t *testing.T, arquivos map[string]string) (*project, *scan.Result) {
		t.Helper()
		dir := t.TempDir()
		for nome, src := range arquivos {
			caminho := filepath.Join(dir, filepath.FromSlash(nome))
			if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(caminho, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		res, err := scan.Scan(dir, "exemplo.com/x")
		if err != nil {
			t.Fatal(err)
		}
		return &project{Root: dir, Module: "exemplo.com/x"}, res
	}
	const pagina = "package app\n\nimport (\n\t\"github.com/emersonjoe/trilha\"\n\t\"github.com/emersonjoe/trilha/h\"\n)\n\nfunc Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }\n"
	const escrita = "package acoes\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc POST(c *trilha.Ctx) error { return c.Text(200, \"ok\") }\n"
	const leitura = "package leitura\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc GET(c *trilha.Ctx) error { return c.JSON(200, nil) }\n"
	const kind = "package app\n\nimport \"github.com/emersonjoe/trilha\"\n\nvar Kind = trilha.KindPage\n"

	p, aberto := escreve(t, map[string]string{"app/page.go": pagina, "app/acoes/route.go": escrita})
	if got := openWrites(p, aberto); len(got) != 1 || got[0] != "/acoes" {
		t.Errorf("escrita aberta não apontada: %v", got)
	}
	p, coberto := escreve(t, map[string]string{"app/page.go": pagina, "app/kind.go": kind, "app/acoes/route.go": escrita})
	if got := openWrites(p, coberto); got != nil {
		t.Errorf("o kind.go acima devia calar o aviso: %v", got)
	}
	// Leitura não escreve nada, e um app que não serve página nenhuma é uma
	// API de verdade: nos dois casos o aviso seria ruído.
	p, so := escreve(t, map[string]string{"app/page.go": pagina, "app/coisas/route.go": leitura})
	if got := openWrites(p, so); got != nil {
		t.Errorf("GET não é escrita: %v", got)
	}
	p, api := escreve(t, map[string]string{"app/route.go": strings.Replace(escrita, "package acoes", "package app", 1)})
	if got := openWrites(p, api); got != nil {
		t.Errorf("app sem páginas é uma API: %v", got)
	}
	// Spec 066: a rota que responde a uma ilha continua sendo API — os erros
	// dela são problem+json — mas pede o token na mão, e isso é uma resposta
	// ao aviso, não um caso a mais dele.
	const guarda = "package acoes\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error { return trilha.RequireCSRF(c, next) }\n"
	p, ilha := escreve(t, map[string]string{"app/page.go": pagina, "app/acoes/route.go": escrita, "app/acoes/middleware.go": guarda})
	if got := openWrites(p, ilha); got != nil {
		t.Errorf("RequireCSRF é a resposta ao aviso: %v", got)
	}
}

// The metrics item is about the endpoint Observability publishes, not about
// every field named Metrics. cache.Options has one with the same name that
// only picks the registry the counters go to, and it made the reference app
// report a critical that was never true.
func TestMetricsItemLooksAtTheEndpoint(t *testing.T) {
	item := func(t *testing.T, cs []check) check {
		t.Helper()
		for _, c := range cs {
			if strings.Contains(strings.ToLower(c.title), "metrics") {
				return c
			}
		}
		t.Fatal("the audit never looked at the metrics")
		return check{}
	}
	proj := func(t *testing.T, src string) *project {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return &project{Root: root}
	}
	t.Setenv("TRILHA_METRICS", "")
	t.Setenv("TRILHA_OBS_TOKEN", "")
	t.Setenv("TRILHA_OBS_TRUSTED", "")

	cases := []struct {
		name  string
		src   string
		level string
	}{
		{"cache option", "package main\n\nvar c = cache.New(cache.Options{Name: \"posts\", MaxEntries: 500, Metrics: a.Metrics()})\n", "ok"},
		{"assignment", "package main\n\nfunc f() { cfg.Observability.Metrics = \"/_trilha/metrics\" }\n", "critical"},
		{"literal", "package main\n\nvar o = trilha.Observability{Metrics: \"/_trilha/metrics\"}\n", "critical"},
		{"literal with the type elided", "package main\n\nvar c = trilha.Config{Observability: {Metrics: \"/_trilha/metrics\"}}\n", "critical"},
		{"another observability field", "package main\n\nfunc f() { cfg.Observability.CacheFor = 2 * time.Second }\n", "ok"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := item(t, runAudit(proj(t, c.src), false))
			if got.level != c.level {
				t.Errorf("level %q (%s), want %q", got.level, got.title, c.level)
			}
		})
	}

	// The environment variable is what the runtime reads into the field, so
	// it turns the item on even in a source that never mentions observability.
	quiet := proj(t, "package main\n")
	if got := item(t, runAudit(quiet, false)); got.level != "ok" {
		t.Errorf("with no endpoint the level is %q", got.level)
	}
	t.Setenv("TRILHA_METRICS", "/_trilha/metrics")
	if got := item(t, runAudit(quiet, false)); got.level != "critical" {
		t.Errorf("with TRILHA_METRICS the level is %q", got.level)
	}
	t.Setenv("TRILHA_OBS_TOKEN", strings.Repeat("t", 32))
	if got := item(t, runAudit(quiet, false)); got.level != "ok" {
		t.Errorf("with a token the level is %q", got.level)
	}
}

// SC-016 — o alvo em http puro é o único dos três que é crítico: a credencial
// da sessão atravessa a rede legível. O localhost é o laço de desenvolvimento
// e não conta.
func TestAuditoriaOlhaOsUpstreams(t *testing.T) {
	casos := []struct {
		nome  string
		src   string
		plain []string
	}{
		{
			nome:  "http para fora",
			src:   `cfg.Upstreams = map[string]trilha.Upstream{"/api/": {Target: "http://api.interno:8801", Headers: func(){}}}`,
			plain: []string{"http://api.interno:8801"},
		},
		{
			nome: "http na própria máquina",
			src:  `cfg.Upstreams = map[string]trilha.Upstream{"/api/": {Target: "http://localhost:8801", Headers: func(){}}}`,
		},
		{
			nome: "https",
			src:  `cfg.Upstreams = map[string]trilha.Upstream{"/api/": {Target: "https://api.exemplo.com", Headers: func(){}}}`,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := plainTargets(c.src)
			if len(got) != len(c.plain) {
				t.Fatalf("%v, queria %v", got, c.plain)
			}
			for i := range got {
				if got[i] != c.plain[i] {
					t.Fatalf("%v, queria %v", got, c.plain)
				}
			}
		})
	}
	// Um upstream sem Headers nenhum num app que exige login: ou é uma
	// credencial esquecida, ou merece um comentário dizendo que é de propósito.
	const comLogin = "\nvar Middleware = sso.Require()\n"
	const upstream = `cfg.Upstreams = map[string]trilha.Upstream{"/api/": {Target: "https://api.exemplo.com"}}`
	const comHeaders = `cfg.Upstreams = map[string]trilha.Upstream{"/api/": {Target: "https://x", Headers: nil}}`
	for _, c := range []struct {
		nome string
		src  string
		quer bool
	}{
		{"sem credencial num app com login", upstream + comLogin, true},
		{"com Headers", comHeaders + comLogin, false},
		{"sem login nenhum", upstream, false},
		{"sem upstream", comLogin, false},
	} {
		if got := upstreamWithoutCredential(c.src); got != c.quer {
			t.Errorf("%s: %v", c.nome, got)
		}
	}

	// Um login que ninguém limita é uma máquina de adivinhar senha.
	const login = "return sessoes.Login(c, u)"
	for _, c := range []struct {
		nome string
		src  string
		env  bool
		quer bool
	}{
		{"login sem limite", login, false, true},
		{"login com RateLimit no código", login + "\ncfg.RateLimit = trilha.RateLimit{RPS: 1}", false, false},
		{"login com limite no ambiente", login, true, false},
		{"sem login", "cfg.Addr = \":3000\"", false, false},
	} {
		if got := loginWithoutLimit(c.src, c.env); got != c.quer {
			t.Errorf("%s: %v", c.nome, got)
		}
	}
}

func TestAuditoriaOlhaOStreamAberto(t *testing.T) {
	casos := []struct {
		nome string
		src  string
		quer bool
	}{
		{"sem live", `h.Div(ui.Poll("6s", "/status"))`, false},
		{"live sem sessão", `h.Body(ui.Live("/events"))`, true},
		{"live com rota protegida", `h.Body(ui.Live("/events"))` + "\n" + `func Middleware(c *trilha.Ctx, next trilha.Next) error { return flow.Require()(c, next) }`, false},
		{"live com papel", `ui.Live("/events")` + "\n" + `flow.RequireRole("admin")`, false},
	}
	for _, c := range casos {
		if got := liveWithoutAuth(c.src); got != c.quer {
			t.Errorf("%s: liveWithoutAuth = %v", c.nome, got)
		}
	}
}

// Spec 066: JavaScript that came from outside and nobody recorded. The file is
// served to every visitor, so where it came from is a fact the repository
// should hold, not a thing to remember.
func TestAuditoriaOlhaOVendorSemLock(t *testing.T) {
	root := t.TempDir()
	p := &project{Root: root, Module: "example.com/x"}
	if got := unpinnedVendor(p); got != nil {
		t.Fatalf("a project without public/vendor has no finding: %v", got)
	}
	dir := filepath.Join(root, "public", "vendor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"preact.js", "htm.js"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("//\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := unpinnedVendor(p)
	if len(got) != 2 || got[0] != "public/vendor/htm.js" {
		t.Fatalf("both files are unpinned, in order: %v", got)
	}
	lock := "# trilha vendor. name version sha256 file url\npreact 10.19.3 abc public/vendor/preact.js https://esm.sh/preact@10.19.3\n"
	if err := os.WriteFile(filepath.Join(root, vendorLock), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	got = unpinnedVendor(p)
	if len(got) != 1 || got[0] != "public/vendor/htm.js" {
		t.Fatalf("only the one nobody pinned: %v", got)
	}
}

// Spec 070: o iframe escrito à mão. A armadilha não está onde parece — quem
// recusa ser enquadrado é a resposta lá dentro —, então o aviso tem de dizer
// isso, e sumir quando a app usa o ui.Preview.
func TestAuditoriaDoIframeEscritoAMao(t *testing.T) {
	tem := func(cs []check) bool {
		for _, c := range cs {
			if strings.Contains(c.title, "iframe") || strings.Contains(c.title, "IFrame") {
				return true
			}
		}
		return false
	}
	escreve := func(t *testing.T, src string) *project {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return &project{Root: dir}
	}
	t.Setenv("TRILHA_SECRET", strings.Repeat("k", 40))

	mao := escreve(t, "package main\n\nvar x = h.Iframe(h.Src(\"/arquivo\"))\n")
	if !tem(runAudit(mao, false)) {
		t.Fatal("o iframe à mão devia virar aviso")
	}
	kit := escreve(t, "package main\n\nvar x = ui.Preview(c, \"/arquivo\", ui.PreviewOpts{})\n")
	if tem(runAudit(kit, false)) {
		t.Fatal("quem usa o ui.Preview não devia ser avisado")
	}
}

// O modo dev do trilha/mail escreve .eml numa pasta e diz onde. Em produção
// isso é o convite que nunca sai, e ninguém descobre até alguém perguntar.
func TestAuditoriaDeEmail(t *testing.T) {
	acha := func(cs []check) (check, bool) {
		for _, c := range cs {
			if strings.Contains(strings.ToLower(c.title), "mail") {
				return c, true
			}
		}
		return check{}, false
	}
	escreve := func(t *testing.T, src string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	manda := escreve(t, "package main\n\nvar Mail = mail.New(mail.FromEnv())\n")
	nao := escreve(t, "package main\n")

	t.Setenv("TRILHA_MAIL_URL", "")
	if got, ok := acha(runAudit(&project{Root: manda}, false)); !ok || got.level != "warn" {
		t.Errorf("app que manda e-mail sem servidor: %+v (achou: %v)", got, ok)
	}
	// Quem não manda e-mail não precisa ouvir sobre e-mail.
	if _, ok := acha(runAudit(&project{Root: nao}, false)); ok {
		t.Error("avisou sobre e-mail um app que não manda nenhum")
	}
	t.Setenv("TRILHA_MAIL_URL", "smtp://smtp.org.br:587")
	if got, ok := acha(runAudit(&project{Root: manda}, false)); !ok || got.level != "ok" {
		t.Errorf("com servidor: %+v", got)
	}
}

// #118 — proxy sem Timeout. O padrão é trinta segundos, e trinta segundos por
// requisição pendurada é o que derruba o app inteiro quando a API do outro
// lado fica lenta — e a falha chega como "nosso app caiu", que manda todo
// mundo procurar no lugar errado.
func TestAuditoriaDeUpstreamSemTimeout(t *testing.T) {
	acha := func(cs []check) (check, bool) {
		for _, c := range cs {
			if strings.Contains(strings.ToLower(c.title), "timeout") {
				return c, true
			}
		}
		return check{}, false
	}
	escreve := func(t *testing.T, src string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	sem := escreve(t, "package main\n\nvar cfg = trilha.Config{Upstreams: map[string]trilha.Upstream{\"/api/\": {Target: \"https://api.exemplo\"}}}\n")
	com := escreve(t, "package main\n\nvar cfg = trilha.Config{Upstreams: map[string]trilha.Upstream{\"/api/\": {Target: \"https://api.exemplo\", Timeout: 5 * time.Second}}}\n")
	nenhum := escreve(t, "package main\n")

	if got, ok := acha(runAudit(&project{Root: sem}, false)); !ok || got.level != "warn" {
		t.Errorf("proxy sem Timeout: %+v (achou: %v)", got, ok)
	}
	if _, ok := acha(runAudit(&project{Root: com}, false)); ok {
		t.Error("avisou de um proxy que tem Timeout")
	}
	// Quem não faz proxy não ouve sobre proxy.
	if _, ok := acha(runAudit(&project{Root: nenhum}, false)); ok {
		t.Error("avisou um app que não tem upstream nenhum")
	}
}
