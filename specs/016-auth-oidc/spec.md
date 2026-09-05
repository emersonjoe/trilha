# Feature Specification: Autenticação — sessão, RBAC e OIDC (Entra ID e Keycloak)

**Feature Branch**: `016-auth-oidc` | **Created**: 2026-09-05 | **Status**: Implementada (v0.7.0)
**Input**: "adicione integração com entra ids - com keycloak" (issue #40, roadmap §9)

## O problema

O Trilha já tem as peças de segurança de uma sessão (cookie assinado, CSRF, limite de taxa,
cookies `HttpOnly`/`Secure`/`SameSite`), mas quem escreve um app corporativo precisa hoje
implementar OpenID Connect à mão: descoberta, PKCE, `state`, `nonce`, troca de código,
validação de assinatura com JWKS, rotação de chave, extração de papéis. É exatamente o tipo
de código em que um erro pequeno vira uma falha de autenticação inteira — e é o mesmo código
em todo projeto.

## Decisões de escopo

- **OIDC, não "login social genérico".** O alvo declarado é Microsoft Entra ID e Keycloak,
  os dois provedores que o usuário usa; Google e qualquer emissor conforme funcionam pelo
  mesmo caminho, porque tudo vem da descoberta. GitHub (OAuth2 puro, sem `id_token`) fica
  para uma issue separada.
- **Zero dependências** (princípio II): JWS `RS256`/`ES256`, JWKS e o fluxo inteiro saem de
  `crypto/rsa`, `crypto/ecdsa`, `encoding/json` e `net/http`.
- **O framework não escolhe o provedor.** `auth.OIDC(issuer, ...)` é a base; `auth.EntraID`
  e `auth.Keycloak` são atalhos que só montam o *issuer* e sabem onde cada um guarda papéis.
- **Sessão sem banco por padrão.** O estado vai num cookie assinado (a mesma chave
  `TRILHA_SECRET`), o que preserva "um binário, sem infraestrutura". Quem precisar de
  revogação imediata liga um `Store`.
- **As rotas continuam vindo de arquivos** (princípio I): o pacote entrega *handlers*, e o
  app os expõe em `app/entrar/route.go`, `app/entrar/retorno/route.go` e `app/sair/route.go`.

## User Scenarios & Testing

### US1 - Entrar com a conta da empresa (P1)
Como pessoa usuária, clico em "Entrar", vou para o provedor, autentico e volto autenticada.
**Acceptance**: `GET /entrar` redireciona para o `authorization_endpoint` com
`response_type=code`, `scope=openid ...`, `state`, `nonce` e `code_challenge` (S256);
o retorno troca o código, valida o `id_token` e cria a sessão; o `state` e o `nonce` são
conferidos e descartados. `state` inválido ou ausente → 400 sem criar sessão.

### US2 - Confiar no token, não no que voltou pela URL (P1)
**Acceptance**: o `id_token` é validado contra a chave do JWKS indicada pelo `kid`, com
`iss`, `aud`, `exp`, `nbf` (com tolerância de relógio de 60 s) e `nonce`. Assinatura errada,
algoritmo `none`, `aud` de outro cliente, token expirado ou `nonce` diferente → falha, sem
sessão, com evento de segurança registrado. Chave rodada no provedor → o JWKS é rebuscado
uma vez (com limite de frequência) e a validação passa.

### US3 - Sessão que expira e que roda (P1)
**Acceptance**: a sessão tem prazo absoluto (8 h por padrão) e por inatividade (30 min);
o identificador roda no login (contra fixação de sessão); `POST /sair` apaga o cookie e,
quando o provedor suporta, redireciona para o `end_session_endpoint` (RP-Initiated Logout).

### US4 - Autorização por papel (P1)
**Acceptance**: `auth.Require()` exige sessão; `auth.RequireRole("financeiro")` exige papel.
Sem sessão, uma página redireciona para `/entrar?next=...` e uma rota de API responde 401;
com sessão e sem papel, 403. Os papéis saem de `roles` e `groups` (Entra ID) e de
`realm_access.roles` e `resource_access.<client>.roles` (Keycloak).

### US5 - Configurar sem ler o código do framework (P2)
**Acceptance**: `auth.EntraID(tenant, clientID, secret, redirect)` e
`auth.Keycloak(baseURL, realm, clientID, secret, redirect)` bastam; tudo o mais vem da
descoberta. O capítulo do site traz o passo a passo de registro nos dois provedores,
incluindo quais URIs de retorno cadastrar e onde ficam os papéis.

## Requirements

- **FR-001** Nenhuma dependência externa; a suíte roda offline contra um provedor falso.
- **FR-002** Segredo do cliente nunca aparece em log, em URL ou em mensagem de erro.
- **FR-003** Cookies de fluxo (`state`, `nonce`, verificador PKCE) são assinados, `HttpOnly`,
  `SameSite=Lax`, com validade de 10 minutos, e são apagados no retorno.
- **FR-004** O cookie de sessão é assinado, `HttpOnly`, `Secure` (quando a requisição é
  HTTPS, direta ou por proxy confiável) e `SameSite=Lax`.
- **FR-005** `id_token` sem `kid` conhecido, com algoritmo simétrico ou `none`, é recusado.
- **FR-006** O parâmetro `next` do login só aceita caminho relativo do próprio app (nada de
  redirecionamento aberto).
- **FR-007** Toda falha de autenticação emite `SecurityEvent` do tipo `auth` e é contada em
  `trilha_security_events_total` (spec 014).
- **FR-008** Documentação em pt-BR (capítulo + referência) e exemplo executável.

## Fora de escopo

- Login com usuário e senha guardados pelo app (o Trilha não vira provedor de identidade).
- GitHub e outros OAuth2 sem `id_token` (issue separada).
- SAML, mTLS, WebAuthn.
- Revogação distribuída entre réplicas: o `Store` opcional dá o gancho, a implementação
  distribuída é do app.
