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
	escreve := func(t *testing.T, arquivos map[string]string) *scan.Result {
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
		return res
	}
	const pagina = "package app\n\nimport (\n\t\"github.com/emersonjoe/trilha\"\n\t\"github.com/emersonjoe/trilha/h\"\n)\n\nfunc Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }\n"
	const escrita = "package acoes\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc POST(c *trilha.Ctx) error { return c.Text(200, \"ok\") }\n"
	const leitura = "package leitura\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc GET(c *trilha.Ctx) error { return c.JSON(200, nil) }\n"
	const kind = "package app\n\nimport \"github.com/emersonjoe/trilha\"\n\nvar Kind = trilha.KindPage\n"

	aberto := escreve(t, map[string]string{"app/page.go": pagina, "app/acoes/route.go": escrita})
	if got := openWrites(aberto); len(got) != 1 || got[0] != "/acoes" {
		t.Errorf("escrita aberta não apontada: %v", got)
	}
	coberto := escreve(t, map[string]string{"app/page.go": pagina, "app/kind.go": kind, "app/acoes/route.go": escrita})
	if got := openWrites(coberto); got != nil {
		t.Errorf("o kind.go acima devia calar o aviso: %v", got)
	}
	// Leitura não escreve nada, e um app que não serve página nenhuma é uma
	// API de verdade: nos dois casos o aviso seria ruído.
	so := escreve(t, map[string]string{"app/page.go": pagina, "app/coisas/route.go": leitura})
	if got := openWrites(so); got != nil {
		t.Errorf("GET não é escrita: %v", got)
	}
	api := escreve(t, map[string]string{"app/route.go": strings.Replace(escrita, "package acoes", "package app", 1)})
	if got := openWrites(api); got != nil {
		t.Errorf("app sem páginas é uma API: %v", got)
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
