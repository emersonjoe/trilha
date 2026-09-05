// Package sso monta o fluxo OpenID Connect a partir do ambiente e expõe
// funções finas para as rotas em app/. Nenhum segredo mora no código: tudo
// vem de variáveis de ambiente.
package sso

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
)

var (
	flow      *auth.Auth
	adminRole = "admin"
	// motivo explica por que o login está indisponível, quando está.
	motivo string
)

// Configure lê o ambiente. Sem configuração o app continua de pé e as rotas
// protegidas explicam o que falta — é o que se quer num exemplo.
func Configure() {
	id, secret := os.Getenv("SSO_CLIENT_ID"), os.Getenv("SSO_CLIENT_SECRET")
	redirect := os.Getenv("SSO_REDIRECT_URL")
	if r := os.Getenv("SSO_ADMIN_ROLE"); r != "" {
		adminRole = r
	}
	if id == "" || redirect == "" {
		motivo = "defina SSO_CLIENT_ID, SSO_CLIENT_SECRET e SSO_REDIRECT_URL"
		return
	}
	var p *auth.Provider
	switch strings.ToLower(os.Getenv("SSO_PROVIDER")) {
	case "entra", "entraid", "azure":
		tenant := os.Getenv("SSO_TENANT")
		if tenant == "" {
			motivo = "SSO_PROVIDER=entra exige SSO_TENANT (o id do diretório)"
			return
		}
		p = auth.EntraID(tenant, id, secret, redirect)
	case "keycloak":
		base, realm := os.Getenv("SSO_URL"), os.Getenv("SSO_REALM")
		if base == "" || realm == "" {
			motivo = "SSO_PROVIDER=keycloak exige SSO_URL e SSO_REALM"
			return
		}
		p = auth.Keycloak(base, realm, id, secret, redirect)
	case "cognito":
		region, pool := os.Getenv("SSO_REGION"), os.Getenv("SSO_USER_POOL_ID")
		if region == "" || pool == "" {
			motivo = "SSO_PROVIDER=cognito exige SSO_REGION e SSO_USER_POOL_ID"
			return
		}
		p = auth.Cognito(region, pool, id, secret, redirect)
		// O Cognito não publica end_session_endpoint: sem o domínio de managed
		// login o /sair apaga a sessão local e para por aí.
		p.LogoutDomain = os.Getenv("SSO_LOGOUT_DOMAIN")
	default:
		issuer := os.Getenv("SSO_ISSUER")
		if issuer == "" {
			motivo = "defina SSO_PROVIDER (entra|keycloak|cognito) ou SSO_ISSUER"
			return
		}
		p = auth.OIDC(issuer, id, secret, redirect)
	}
	flow = auth.New(p, auth.Options{
		LoginPath:  "/entrar",
		AfterLogin: "/painel",
		// Papéis fora do lugar padrão do provedor entram por aqui.
		RoleClaims: split(os.Getenv("SSO_ROLE_CLAIMS")),
	})
	motivo = ""
}

// Configurado diz se há provedor. As páginas usam isso para trocar o texto.
func Configurado() bool { return flow != nil }

// Motivo é a explicação de o que falta configurar.
func Motivo() string { return motivo }

// AdminRole é o papel exigido em /painel/relatorio.
func AdminRole() string { return adminRole }

// Start, Callback e Logout são o fluxo; as rotas só as encaminham.
func Start(c *trilha.Ctx) error    { return with(c, func() error { return flow.Start(c) }) }
func Callback(c *trilha.Ctx) error { return with(c, func() error { return flow.Callback(c) }) }
func Logout(c *trilha.Ctx) error {
	if flow == nil {
		return c.Redirect("/")
	}
	return flow.Logout(c)
}

// Require e RequireAdmin são os middlewares que app/ exporta.
func Require(c *trilha.Ctx, next trilha.Next) error {
	if flow == nil {
		return indisponivel(c)
	}
	return flow.Require()(c, next)
}

func RequireAdmin(c *trilha.Ctx, next trilha.Next) error {
	if flow == nil {
		return indisponivel(c)
	}
	return flow.RequireRole(adminRole)(c, next)
}

// User devolve quem está logado, ou nil.
func User(c *trilha.Ctx) *auth.User {
	if flow == nil {
		return nil
	}
	return flow.User(c)
}

func with(c *trilha.Ctx, fn func() error) error {
	if flow == nil {
		return indisponivel(c)
	}
	return fn()
}

// indisponivel responde à falta de configuração. Um 5xx numa página vira a
// tela genérica de erro, que não explica nada: para o navegador é melhor
// voltar à home, que diz exatamente o que falta. Uma chamada de API, essa
// sim, merece o 503.
func indisponivel(c *trilha.Ctx) error {
	if strings.Contains(c.Request().Header.Get("Accept"), "text/html") {
		return trilha.RedirectCode("/", http.StatusSeeOther)
	}
	return trilha.Errorf(http.StatusServiceUnavailable, "login não configurado: %s", motivo)
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Descricao resume a configuração ativa, para a página inicial.
func Descricao() string {
	if flow == nil {
		return "não configurado"
	}
	return fmt.Sprintf("%s (cliente %s)", flow.Provider().Issuer, flow.Provider().ClientID)
}
