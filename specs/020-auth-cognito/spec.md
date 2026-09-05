# Spec 020 — Atalho de provedor para o AWS Cognito

- **Issue**: [#41](https://github.com/emersonjoe/trilha/issues/41) — a lista de implementação
  está lá; aqui só o que mudou em relação a ela.
- **Branch**: `020-auth-cognito`
- **Versão**: 0.11.0

## Por quê

A 0.7.0 (spec 016) entregou OIDC com atalhos para Entra ID e Keycloak. O Cognito já
funcionava pelo `auth.OIDC` genérico, mas custava três coisas ao usuário: acertar o formato
do emissor na mão, descobrir que os grupos moram em `cognito:groups` e configurar
`Options.RoleClaims`, e — a que dói — descobrir do jeito difícil que o logout federado
simplesmente não acontece, porque o Cognito não publica `end_session_endpoint`. O `Logout`
voltava para o app em silêncio, como se tivesse encerrado a sessão no provedor.

## O que muda

```go
p := auth.Cognito("us-east-1", "us-east-1_ABC123", id, segredo, redirect)
p.LogoutDomain = "exemplo.auth.us-east-1.amazoncognito.com" // opcional
```

- **`auth.Cognito(region, userPoolID, clientID, clientSecret, redirectURL) *Provider`** monta
  o emissor `https://cognito-idp.<region>.amazonaws.com/<userPoolID>`.
- **Papéis** de `cognito:groups`, sem configuração (`auth/roles.go`).
- **`Provider.LogoutDomain`**: com ele, `Logout` redireciona para
  `<domínio>/logout?client_id=…&logout_uri=…` — não é RP-Initiated Logout, é o formato
  próprio da AWS, e a URL de retorno precisa estar nas *Allowed sign-out URLs* do app client.
  Sem ele, `Logout` apaga a sessão local, **registra no log** que foi só isso e volta para
  `AfterLogout`. Os outros provedores ignoram o campo.
- **`trilha audit`** conhece a posição do segredo em `Cognito(...)` (índice 3, como no
  `Keycloak`), senão um segredo literal passaria batido.

O que varia por provedor continua fora do fluxo: `flow.go` pergunta
`p.endSession(doc, postLogout) (url, motivo string)` e não sabe qual provedor é.

## Fora de escopo

- **Clerk**, que a issue pedia no mesmo pacote. A documentação pública descreve o
  `/.well-known/jwks.json` e um `id_token` com `org_id`, mas **não** um
  `/.well-known/openid-configuration` — que é de onde o `auth` tira todos os endereços — nem
  uma claim com o papel na organização, que seria o motivo de existir do atalho. Os exemplos
  de emissor da Clerk ainda terminam em `/`, e o `OIDC()` corta a barra final: se o documento
  declarar o emissor com ela, a descoberta falha por divergência. Nada disso se resolve sem
  uma instância real para conferir, e atalho escrito em cima de suposição é pior que atalho
  nenhum. Registrado na #41, que fica aberta.
- **Troca de token**: verificado na documentação da AWS que o `/oauth2/token` do Cognito
  aceita `client_secret_post`, que é o que o `exchange` já faz. Nenhuma mudança.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Cognito é OIDC; nada de SDK da AWS, nenhuma linha nova em `go.mod` |
| IV/VII — contrato e segurança | o segredo continua fora de log e de URL; as recusas de token existentes valem sem alteração para o provedor novo |
| VI — teste primeiro | `TestCognitoLogout` e `TestCognitoLogoutIsLocalWithoutDomain` escritos antes, falhando por símbolo inexistente |
| API pública | `Cognito` e `LogoutDomain` têm doc comment e uso em `examples/sso` |

## Tarefas

- [x] T001 Testes que falham: emissor, papéis de `cognito:groups`, URL de logout com e sem
      `LogoutDomain`, e o ponta a ponta que exige o aviso no log
- [x] T002 `Cognito`, `LogoutDomain`, `endSession` em `provider.go`; caso em `roles.go`;
      `Logout` sem conhecimento de provedor em `flow.go`; `secretArg` em `audit.go`
- [x] T003 `examples/sso`: `SSO_PROVIDER=cognito` com `SSO_REGION`, `SSO_USER_POOL_ID` e
      `SSO_LOGOUT_DOMAIN`, documentado no README do exemplo
- [x] T004 Capítulo e referência nas duas locales, incluindo o motivo de o Clerk ficar de fora
- [x] T005 `CHANGELOG.md` 0.11.0, `version`, item 22 do `ROADMAP.md`
- [x] T006 `make test` verde e `make release VERSION=0.11.0`

## Aceitação

- **SC-001** `auth.Cognito(...)` produz o emissor correto e traz os papéis sem
  `Options.RoleClaims`.
- **SC-002** Com `LogoutDomain`, o `/sair` leva ao `/logout` do domínio de managed login com
  `client_id` e `logout_uri`; sem ele, leva a `AfterLogout` **e deixa rastro no log**.
- **SC-003** Um `end_session_endpoint` anunciado por engano não muda o comportamento do
  Cognito — o atalho não cai no caminho padrão.
- **SC-004** `flow.go` não menciona nenhum provedor.
