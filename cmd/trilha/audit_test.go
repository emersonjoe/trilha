package main

import "testing"

// A auditoria precisa enxergar o segredo literal mesmo quando os outros
// argumentos são chamadas com parênteses e vírgulas dentro.
func TestAuthCallsSeparaArgumentos(t *testing.T) {
	src := `
	p := auth.EntraID(os.Getenv("SSO_TENANT"), id, "s3cret", "https://app/cb")
	q := auth.Keycloak(base, realm, os.Getenv("ID"), os.Getenv("SEGREDO"), "http://app/cb")
	r := auth.OIDC(iss, id, secret, "http://localhost:3000/cb")
	`
	calls := authCalls(src)
	if len(calls) != 3 {
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
}
