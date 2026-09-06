---
title: Segurança
description: Configuração completa de cabeçalhos, proxies, limite de taxa, cookies assinados e eventos.
---

## Config.Security

| Campo | Padrão | Cabeçalho |
|---|---|---|
| `CSP` | política com nonce (abaixo) | `Content-Security-Policy` |
| `CSPExtra map[string][]string` | — | acrescenta origens a diretivas da política padrão |
| `HSTS` | `max-age=31536000; includeSubDomains` (só em HTTPS) | `Strict-Transport-Security` |
| `PermissionsPolicy` | `camera=(), microphone=(), geolocation=(), payment=(), usb=()` | `Permissions-Policy` |
| `COOP` | `same-origin` | `Cross-Origin-Opener-Policy` |
| `FrameOptions` | `DENY` | `X-Frame-Options` |
| `Referrer` | `strict-origin-when-cross-origin` | `Referrer-Policy` |

`trilha.Off` em qualquer campo remove o cabeçalho. `X-Content-Type-Options: nosniff` é
sempre enviado. Política padrão:

```text
default-src 'self'; script-src 'self' 'nonce-…'; style-src 'self' 'unsafe-inline';
img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';
base-uri 'self'; form-action 'self'
```

`c.Nonce()` devolve o nonce da requisição; `trilha.NonceAttr(c)` o coloca em um `h.Script`.
Ajuste em `Setup` por `a.Security()`.

## Proxies confiáveis

`Config.TrustedProxies []string` (CIDR ou IP) ou `TRILHA_TRUSTED_PROXIES=a,b`. Efeitos quando
o peer é confiável: `c.ClientIP()` lê `X-Forwarded-For` (o IP mais à direita que não seja
proxy), `X-Forwarded-Proto: https` liga HSTS e marca cookies como `Secure`.

## Hosts permitidos

`Config.AllowedHosts []string` ou `TRILHA_ALLOWED_HOSTS=a,b`. A requisição cujo `Host` não
está na lista é respondida com 400 antes do roteador, das sondas e do CORS, e emite um evento
`host`. Lista vazia = sem conferência.

| Padrão | Libera | Não libera |
|---|---|---|
| `exemplo.com` | `exemplo.com`, `exemplo.com:8443`, `EXEMPLO.com.` | `sub.exemplo.com` |
| `*.exemplo.com` | `app.exemplo.com` | `exemplo.com`, `a.b.exemplo.com` |

Em `Dev`, `localhost`, `127.0.0.1` e `::1` passam sempre. O que se compara é o host que o app
recebe — atrás de um proxy que reescreve o `Host`, liste o que o proxy manda.

## Limite de taxa

`Config.RateLimit{RPS float64, Burst int}` aplica um *token bucket* por `ClientIP` antes dos
middlewares. `trilha.Limit(rps, burst) MiddlewareFunc` cria um limitador independente para
uma subárvore. Resposta: 429 com `Retry-After` (segundos) e evento `rate`.
`trilha.ErrRateLimited` pode ser devolvido por um handler para o mesmo efeito.

## Cookies assinados

| Símbolo | Descrição |
|---|---|
| `c.SetSigned(nome, valor, ttl) error` | grava cookie `valor|expira|hmac` com `HttpOnly`, `SameSite=Lax`, `Secure` em HTTPS; `ErrNoSecret` sem chave |
| `c.Signed(nome) (string, bool)` | lê e verifica assinatura e prazo |
| `c.ClearCookie(nome)` | expira um cookie |
| `trilha.NewSigner(chaves...)`, `Sign`, `Verify` | o assinador (HMAC-SHA256) para uso direto |
| `Config.Secret`, `Config.PreviousSecret` | `TRILHA_SECRET`, `TRILHA_SECRET_PREVIOUS` (base64 ou texto, ≥ 32 bytes) |

Sem segredo: em `dev` uma chave efêmera é gerada (o `trilha dev` mantém uma por sessão); em
`prod` o app avisa no log e `SetSigned` devolve `ErrNoSecret`.

## Timeouts

`Config.Timeouts{ReadHeader 10s, Read 30s, Write 60s, Idle 120s, MaxHeaderBytes 64 KiB}`.
Para respostas longas (SSE, download), chame `c.NoWriteDeadline()` antes de escrever.

## Eventos de segurança

```go
type SecurityEvent struct {
	Kind      string // csrf | auth | body | host | rate | panic
	Status    int
	Method    string
	Path      string
	IP        string
	RequestID string
}
```

Registrados com `slog.Warn("security", ...)` e entregues a `Config.OnSecurityEvent`, uma vez
por requisição.

## `trilha audit`

Verifica: `TRILHA_SECRET`, `TRILHA_TRUSTED_PROXIES`, `trilha_gen.go` atualizado, versão do
Go, `.gitignore`, `go vet` e `govulncheck` (`--no-vuln` para pular). Código de saída 1 com
item crítico.
