---
title: Segurança
description: O que o Trilha protege por padrão, como ajustar, e o que continua sendo responsabilidade sua.
---

O Trilha segue duas referências: o **NIST Cybersecurity Framework 2.0** (as funções
Identificar, Proteger, Detectar, Responder, Recuperar e Governar) e o **OWASP ASVS 4.0**
nível 2. Um framework web só consegue *proteger* e *detectar*; o restante é trabalho de quem
opera o app, e este capítulo diz exatamente onde termina um e começa o outro.

## O que já vem ligado

| Controle | Padrão | NIST CSF 2.0 | OWASP ASVS |
|---|---|---|---|
| Escape de HTML (`h`) e escape contextual (`tmpl`) | sempre | PR.DS | V5.3 |
| `Content-Security-Policy` com nonce por requisição | ligado | PR.PS | V14.4 |
| `Strict-Transport-Security` | ligado em HTTPS | PR.DS | V9.1 |
| `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy` | ligados | PR.PS | V14.4 |
| CSRF por *double-submit cookie* em formulários | ligado | PR.AA | V4.2 |
| Limite do corpo da requisição (1 MiB) | ligado | PR.IR | V13.1 |
| Timeouts de leitura, escrita e ociosidade; limite de cabeçalhos | ligados | PR.IR | V13.1 |
| Estáticos restritos a `public/` | sempre | PR.DS | V12.3 |
| Erros opacos em produção; sem stack, sem caminho | ligado | PR.DS | V7.4 |
| Logs estruturados sem corpo nem cookies, com `request_id` | sempre | DE.CM | V7.1 |
| Eventos de segurança (CSRF, 401/403, 413, 429, panic) no log | sempre | DE.AE | V7.2 |
| Cookies assinados (`SetSigned`/`Signed`) | com `TRILHA_SECRET` | PR.AA | V3.4 |
| Limite de taxa por cliente | opcional | PR.IR | V11.1 |
| Proxies confiáveis (`X-Forwarded-*`) | opcional | PR.AA | V14.1 |

## CSP e scripts inline

A política padrão só permite scripts do próprio site ou com o **nonce** da requisição. Um
`<script>` inline precisa dele:

```go
h.Script(trilha.NonceAttr(c), h.Raw(`document.body.dataset.pronto = "1"`))
```

O script de recarga do `trilha dev` já usa o nonce. Para liberar uma origem externa (fontes,
CDN de imagens) sem reescrever a política, acrescente em `app/setup.go`:

```go
func Setup(a *trilha.App) error {
	a.Security().CSPExtra = map[string][]string{
		"style-src": {"https://fonts.googleapis.com"},
		"font-src":  {"https://fonts.gstatic.com"},
	}
	return nil
}
```

Para uma política totalmente sua, defina `a.Security().CSP` (a string pode conter
`{nonce}`); para desligar um cabeçalho, atribua `trilha.Off`.

## Atrás de um proxy

Se o app roda atrás de nginx, Caddy ou um balanceador, o `RemoteAddr` é o proxy. Diga ao
Trilha em quem confiar para que `X-Forwarded-For` e `X-Forwarded-Proto` valham:

```bash
TRILHA_TRUSTED_PROXIES=10.0.0.0/8,127.0.0.1
```

Só então `c.ClientIP()` devolve o cliente real, o HSTS é enviado e o limite de taxa conta por
cliente e não por proxy. Sem essa variável, cabeçalhos `X-Forwarded-*` são ignorados, o que
é o comportamento seguro.

## Sessão com cookie assinado

Um cookie assinado não pode ser forjado nem alterado, e vence sozinho:

```go
// no POST do login
if err := c.SetSigned("sessao", usuario.ID, 8*time.Hour); err != nil {
	return err
}

// no middleware da área restrita
id, ok := c.Signed("sessao")
if !ok {
	return trilha.RedirectCode("/entrar", 302)
}
```

A chave vem de `TRILHA_SECRET` (32 bytes ou mais; `openssl rand -base64 32`). Em
desenvolvimento o `trilha dev` gera uma chave efêmera por sessão. Em produção sem a
variável, `SetSigned` devolve erro e registra um aviso dizendo qual cookie, em qual rota —
uma vez por cookie, e só para quem de fato assina algum: um aviso em toda subida de um app
que tem sessão própria é o que ensina a equipe a não ler aviso. Para trocar a chave sem
derrubar sessões, coloque a antiga em `TRILHA_SECRET_PREVIOUS` até que expirem.

:::atencao
O cookie assinado garante integridade, não sigilo: o valor é legível por quem tem o cookie.
Guarde nele um identificador, nunca dados sensíveis.
:::

## Limite de taxa

Global, em `app/setup.go`, ou por subárvore, em um `middleware.go`:

```go
// app/api/middleware.go
var limit = trilha.Limit(5, 20) // 5 req/s por cliente, rajada de 20

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return limit(c, next)
}
```

A resposta é 429 com `Retry-After`. O contador vive na memória do processo: com várias
réplicas, cada uma conta a sua parte.

## Detectar e responder

Cada bloqueio gera uma linha `security` no log, com `kind`, `ip`, `path` e `request_id`, e
chama `Config.OnSecurityEvent` se você definir um. É o gancho para contar tentativas,
alertar ou bloquear um IP no firewall.

Antes de publicar, rode:

```bash
trilha audit
```

Ele confere `TRILHA_SECRET`, proxies, `trilha_gen.go`, versão do Go, `go vet` e
`govulncheck`, e sai com erro se houver item crítico.

## O que continua sendo seu

- **Autenticação e autorização**: quem é o usuário e o que pode fazer. O Trilha dá o
  cookie assinado e o middleware; a regra de negócio é sua.
- **Dados em repouso**: criptografia do banco, backups, retenção.
- **TLS**: termine no proxy ou use um certificado no próprio `http.Server` via
  `a.Handler()`.
- **Segredos**: só em variáveis de ambiente ou em um cofre; nunca no repositório.
- **Governar, Identificar, Recuperar**: inventário, classificação de dados, plano de
  resposta e restauração são processos da organização. O `SECURITY.md` do projeto descreve
  como relatar vulnerabilidades do framework.

## Desafio

Faça a área `/painel` do seu app exigir sessão assinada, com um limite de 10 tentativas por
minuto no formulário de login, e registre em `OnSecurityEvent` quantos bloqueios ocorreram.

:::solucao
```go
// app/entrar/middleware.go
var limit = trilha.Limit(10.0/60, 10)

func Middleware(c *trilha.Ctx, next trilha.Next) error { return limit(c, next) }

// app/setup.go
var bloqueios atomic.Int64

func Setup(a *trilha.App) error {
	a.Config().OnSecurityEvent = func(e trilha.SecurityEvent) {
		if e.Kind == "rate" { bloqueios.Add(1) }
	}
	return nil
}
```

`a.Config()` dá acesso à configuração dentro de `Setup`; o limitador do login usa
`trilha.Limit` com 10 fichas por minuto.
:::
