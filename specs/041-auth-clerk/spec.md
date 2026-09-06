# Spec 041 — Atalho `auth.Clerk`

- **Issue**: [#41](https://github.com/emersonjoe/trilha/issues/41) — a issue é a fonte do
  escopo.
- **Branch**: `041-auth-clerk`
- **Versão**: 0.31.0

## Por quê

A metade do Cognito saiu na 0.11.0 (spec 020). O Clerk ficou aberto porque a documentação
pública não respondia três perguntas, e atalho escrito em cima de suposição erra em silêncio
na casa de quem usa. As três foram conferidas agora contra um documento de descoberta real
(`https://clerk.clerk.com/.well-known/openid-configuration`, a própria instância de produção
da Clerk); o resultado está registrado na issue, para não ser conferido uma terceira vez:

1. **Existe `/.well-known/openid-configuration`**, completo: `authorization_endpoint`,
   `token_endpoint`, `userinfo_endpoint` e `jwks_uri`. A descoberta do `auth` funciona sem
   nenhuma adaptação.
2. **O `issuer` do documento não tem barra final** (`https://clerk.clerk.com`), então o
   `strings.TrimSuffix(issuer, "/")` do `OIDC()` casa exatamente — o medo registrado na issue
   vinha dos exemplos da documentação, não do documento.
3. **Não há claim de papel.** O `claims_supported` é `aud, exp, email, family_name, name,
   preferred_username, sub, iss, iat, email_verified, given_name, picture, org_id`: tem a
   organização, não tem o papel nela. E não há `end_session_endpoint`, com
   `backchannel_logout_supported` e `frontchannel_logout_supported` ambos `false`.

Ou seja: o atalho vale metade do que valem os outros três — monta o emissor e diz a verdade
sobre o logout —, e a outra metade (papéis sem configuração) **não existe do lado do Clerk**.
Escrever isso é melhor que deixar a pessoa descobrir sozinha, desde que a documentação diga
exatamente isso em vez de fingir paridade.

## O que muda

```go
// Clerk configures a Clerk instance by its Frontend API URL.
func Clerk(frontendAPI, clientID, clientSecret, redirectURL string) *Provider
```

`frontendAPI` é a *Frontend API URL* do painel: `verb-noun-00.clerk.accounts.dev` em
desenvolvimento, `clerk.seu-dominio.com` em produção. O construtor aceita as quatro grafias
(com e sem `https://`, com e sem barra final) e produz sempre o mesmo emissor, porque é aí que
a pessoa erra.

`kind` novo (`clerkProvider`) com duas consequências, e nenhuma delas no fluxo:

- **Papéis**: nada específico. O `id_token` do Clerk não carrega papel, então o atalho cai no
  mesmo par genérico (`roles`, `groups`) do `OIDC` — quem tiver uma claim configurada na
  instância aponta com `Options.RoleClaims`. A tabela da referência diz isso na linha do
  Clerk, com essas palavras.
- **Logout**: o `endSession` devolve o motivo em vez de silêncio. Sem `end_session_endpoint`
  o `Logout` já apagava a sessão local e voltava para o app; agora ele **avisa no log** que a
  sessão do Clerk continua de pé, como já fazia para o Cognito sem `LogoutDomain`.

## Fora de escopo

- **Ler a organização do Clerk** (`org_id`, `org_slug`) para dentro de `User`. Organização não
  é papel, e `Claims.All` já entrega tudo para quem quiser.
- **A API de organizações do Clerk.** Buscar papel por HTTP depois do login é outro assunto
  (chave de servidor, cache, invalidação) e não cabe num construtor.
- **Google e GitHub**, pelo motivo já registrado no ROADMAP.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o Clerk é OIDC conforme; nenhum SDK, nenhum arquivo novo |
| VI — teste primeiro | `auth/auth_test.go` cobre as quatro grafias do emissor, os papéis e o logout antes da implementação |
| VII — segurança por padrão | o segredo continua fora de log e de URL; `cmd/trilha/audit.go` passa a reconhecer `auth.Clerk` no check de segredo literal |
| — sem código de provedor no fluxo | `flow.go` não muda: o que varia mora em `provider.go` (emissor, motivo do logout) |

## Tarefas

- [x] T001 Teste que falha em `auth/auth_test.go`: `TestClerkIssuer` (quatro grafias, mesmo
      emissor), caso do Clerk em `TestRolesPerProvider` (sem claim de papel; `RoleClaims`
      resolve) e `TestClerkLogout` (sem `end_session_endpoint` o destino é local e o motivo
      cita o Clerk).
- [x] T002 `auth/provider.go`: `Clerk`, `clerkProvider`, motivo no `endSession`.
- [x] T003 `cmd/trilha/audit.go`: `Clerk` na lista de `authCalls` (o índice do segredo já é o
      padrão, 2) e teste do audit com um `auth.Clerk` de segredo literal.
- [x] T004 Documentação nas duas locales: linha na tabela de `reference/auth` e
      `referencia/auth`; no capítulo, a seção "Other providers"/"Outros provedores" troca o
      parágrafo do que ficou em aberto pela seção do Clerk, com o passo a passo do painel.
- [x] T005 `CHANGELOG.md` (0.31.0), `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`.
- [x] T006 `make test` verde e `make release VERSION=0.31.0 ISSUES="41"`.

## Aceitação

- **SC-001** `auth.Clerk("verb-noun-00.clerk.accounts.dev", …)`,
  `"https://verb-noun-00.clerk.accounts.dev"`, `"…dev/"` e `"https://…dev/"` produzem
  `Issuer == "https://verb-noun-00.clerk.accounts.dev"` — verificado no teste.
- **SC-002** Um provedor Clerk cujo documento não anuncia `end_session_endpoint` faz logout
  local e o motivo devolvido pelo `endSession` cita o Clerk — verificado no teste.
- **SC-003** `trilha audit` acusa segredo literal em `auth.Clerk("…", id, "s3cret", cb)`.
- **SC-004** `auth/flow.go` não muda (o `git diff` da release não o toca).
