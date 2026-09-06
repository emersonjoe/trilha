package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A auditoria precisa enxergar o segredo literal mesmo quando os outros
// argumentos são chamadas com parênteses e vírgulas dentro.
func TestAuthCallsSeparaArgumentos(t *testing.T) {
	src := `
	p := auth.EntraID(os.Getenv("SSO_TENANT"), id, "s3cret", "https://app/cb")
	q := auth.Keycloak(base, realm, os.Getenv("ID"), os.Getenv("SEGREDO"), "http://app/cb")
	r := auth.OIDC(iss, id, secret, "http://localhost:3000/cb")
	s := auth.Cognito(regiao, pool, id, "s3cret", "https://app/cb")
	`
	calls := authCalls(src)
	if len(calls) != 4 {
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
