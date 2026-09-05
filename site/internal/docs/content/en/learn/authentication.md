---
title: Autenticação com Entra ID e Keycloak
description: Login OpenID Connect com PKCE, sessão assinada, papéis e logout federado, sem dependência externa e sem senha no seu banco.
---

Quase todo app interno chega no mesmo ponto: alguém pergunta "dá para entrar com a conta da
empresa?". A resposta é OpenID Connect — o Entra ID (antigo Azure AD) e o Keycloak falam o
mesmo protocolo, e o pacote `auth` implementa o lado do app com a biblioteca padrão.

A vantagem não é só comodidade. Senha que você não guarda é senha que você não vaza; MFA,
bloqueio por tentativa e política de rotação passam a ser problema do provedor, que tem um
time cuidando disso. O que sobra para o app é o pedaço que ninguém pode terceirizar:
validar o token direito e manter a sessão em ordem.

## O fluxo, em três rotas

O código de autorização com PKCE tem quatro passos: o app manda o navegador ao provedor, a
pessoa se autentica lá, o provedor devolve um código para uma rota sua, e o app troca esse
código por um `id_token` — essa última troca acontece servidor a servidor, não passa pelo
navegador. No `app/`, isso são três arquivos:

```go
// app/entrar/route.go
var Kind = trilha.KindPage
func GET(c *trilha.Ctx) error { return sso.Start(c) }

// app/entrar/retorno/route.go
var Kind = trilha.KindPage
func GET(c *trilha.Ctx) error { return sso.Callback(c) }

// app/sair/route.go
var Kind = trilha.KindPage
func POST(c *trilha.Ctx) error { return sso.Logout(c) }
```

`sso` aqui é um pacote seu, de umas 30 linhas, que lê o ambiente e guarda o `*auth.Auth`
(veja `examples/sso/internal/sso`). O `auth` não registra rota nenhuma: quem decide os
endereços é o `app/`, como em todo o resto do framework.

## Configurar o provedor

```go
p := auth.EntraID(os.Getenv("SSO_TENANT"), id, segredo, "https://app.exemplo/entrar/retorno")
// ou
p := auth.Keycloak("https://kc.exemplo", "producao", id, segredo, redirect)
// ou qualquer provedor conforme, pelo emissor:
p := auth.OIDC("https://accounts.exemplo/", id, segredo, redirect)

flow := auth.New(p, auth.Options{LoginPath: "/entrar", AfterLogin: "/painel"})
```

Nada aí faz chamada de rede: a descoberta (`/.well-known/openid-configuration`) acontece no
primeiro login e fica em cache por uma hora. Um provedor fora do ar não impede o app de
subir — impede só de entrar, que é o comportamento honesto.

O segredo do cliente **nunca** vai no código. `trilha audit` reclama se encontrar um
literal na posição dele, e um segredo que foi para o git precisa ser rotacionado no
provedor, não apenas removido do arquivo.

## Proteger uma parte do app

É um `middleware.go`, igual a qualquer outro:

```go
// app/painel/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.Require(c, next) }

// app/painel/relatorio/middleware.go — exige papel, não só sessão
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.RequireAdmin(c, next) }
```

Abaixo do middleware a página lê `flow.User(c)` e confia: o `*auth.User` está lá, com
`Subject`, `Email`, `Name` e `Roles`.

Duas respostas diferentes para duas situações diferentes:

- **anônimo**: navegador vai para `/entrar?next=/painel`; qualquer outro cliente recebe
  **401**. Redirecionar uma chamada de API para um formulário HTML só produz um erro de
  parsing difícil de entender do outro lado.
- **logado, mas sem o papel**: **403**. Mandar para o login quem já está logado cria um
  laço — a pessoa entra de novo, volta, e leva 401 outra vez.

## Onde moram os papéis

Cada provedor guarda em um lugar, e o `auth` já sabe onde procurar:

| Provedor | Lê de |
|---|---|
| Entra ID | `roles` (app roles), `groups`, `wids` |
| Keycloak | `realm_access.roles` e `resource_access[seu-cliente].roles` |
| Genérico | `roles`, `groups` |

Papéis do Keycloak que pertencem a **outro** cliente não entram: quem é `admin` no cliente
de contabilidade não vira `admin` no seu. Se a sua instalação usa outro nome de claim,
acrescente-o em `Options.RoleClaims`.

## A sessão

Depois do login, o `id_token` cumpriu o papel dele e é descartado. O que fica é um cookie
assinado com `TRILHA_SECRET`, `HttpOnly`, `SameSite=Lax` e `Secure` sob HTTPS, contendo o
essencial: identificador, nome, e-mail, papéis e prazos.

```go
auth.Options{
	Absolute: 8 * time.Hour,  // prazo máximo, contado do login
	Idle:     30 * time.Minute, // some depois de parado
	Store:    auth.NewMemoryStore(), // opcional: revogação imediata
}
```

Sem `Store`, a sessão é apátrida: vale em qualquer réplica e não precisa de banco, mas só
termina de verdade quando vence. Com um `Store`, o cookie carrega um identificador e o
logout apaga o registro na hora — é o que você quer se precisa desligar alguém agora. O
identificador muda a cada login, então um cookie plantado antes não vira sessão válida.

## O que o `auth` recusa

Cada item aqui é um ataque conhecido, e todos têm teste na suíte:

- **`alg` do token**: a lista é fixa (RS256/384/512, ES256/384). Ler o algoritmo do token e
  obedecer é como bibliotecas de JWT se quebram — `alg: none` passa, ou uma chave pública
  RSA vira segredo de HMAC.
- **`state`**: sem ele o retorno pode ser forjado por outro site (CSRF no login).
- **`nonce`**: amarra o `id_token` a *este* pedido, contra replay.
- **PKCE (S256)**: um código roubado no meio do caminho não serve sem o verificador.
- **`iss`, `aud`, `exp`, `nbf`**: um token legítimo, mas emitido para outro app ou por outro
  tenant, não vale aqui. A tolerância de relógio é de 60 segundos.
- **`next`**: só caminho do próprio app. `//evil.exemplo` e `https://evil.exemplo` viram
  `/` — redirecionamento aberto é o jeito clássico de dar credibilidade a um phishing.
- **Chave desconhecida**: o JWKS é buscado de novo quando o provedor rotaciona a chave, mas
  no máximo uma vez por minuto, para que um token forjado não vire uma requisição de rede
  por requisição HTTP.

Toda recusa vira `SecurityEvent` do tipo `auth` e entra em `trilha_security_events_total`,
o contador do [capítulo de observabilidade](/aprender/observabilidade).

## Desafio

Seu app precisa de uma rota que só funcione para quem entrou **nos últimos cinco minutos** —
uma reautenticação recente antes de uma operação sensível, como trocar a chave de API.

:::solucao
```go
func recente(c *trilha.Ctx, next trilha.Next) error {
	u := flow.User(c)
	if u == nil || time.Since(u.IssuedAt) > 5*time.Minute {
		return trilha.RedirectCode("/entrar?next="+url.QueryEscape(c.Request().URL.Path), 302)
	}
	return next()
}
```
`IssuedAt` é o momento do login, e `Start` cria uma sessão nova a cada volta pelo provedor —
então quem já estava logado só precisa passar de novo pela tela do provedor, que costuma
aceitar sem pedir senha outra vez. Para forçar a digitação, acrescente `prompt=login` aos
parâmetros de autorização.
:::
