# Implementation Plan: Segurança por padrão

**Branch**: `004-seguranca` | **Spec**: [spec.md](spec.md)

## Constitution Check
| Princípio | Atende |
|---|---|
| II stdlib | `crypto/hmac`, `crypto/sha256`, `net/netip`, `time`; token bucket próprio |
| IV contrato | nada muda nas assinaturas; novos métodos em `Ctx` e `Config` |
| VI teste primeiro | `security_test.go`, `ratelimit_test.go`, `signed_test.go` antes do código |
| VII segurança | é a própria feature; padrões endurecidos, opt-out explícito |

## Design
- `security.go`: `Security` struct + `applySecurityHeaders(c)` chamado em `wrap` e `fallback`; nonce em `Ctx` (16 bytes base64). CSP padrão:
  `default-src 'self'; script-src 'self' 'nonce-{n}'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'` + `CSPExtra` (mapa diretiva → origens).
  HSTS `max-age=31536000; includeSubDomains` só quando `c.isSecure()` (TLS ou proxy confiável com `X-Forwarded-Proto: https`).
- `proxy.go`: `TrustedProxies` parse com `netip.ParsePrefix`; `c.ClientIP()`; `isSecure` passa a exigir proxy confiável.
- `ratelimit.go`: `limiter` com `sync.Mutex` + mapa ip → bucket{tokens, last}; varredura a cada minuto; `Limit(rps, burst)` devolve `MiddlewareFunc`; global aplicado em `wrap` antes dos middlewares.
- `signed.go`: `Signer{keys [][]byte}`; formato `valor|exp|base64(hmac)`; `SetSigned/Signed`; `NewSignerFromEnv`; `New` chama e em prod sem chave → `Fatal`-like error retornado por `App.Err()`? Simpler: `ConfigFromEnv` lê; `New` em Prod sem secret → panic com mensagem clara (documentado); em Dev gera e loga aviso.
- `events.go`: `SecurityEvent{Kind, IP, Path, RequestID, Status}`; `a.security(c, kind, status)` chamado em handleError para 401/403/413/429/panic e em checkCSRF.
- `trilha.go`: `Config.Timeouts` (Read 15s, Write 60s, Idle 120s, MaxHeaderBytes 64 KiB).
- `cmd/trilha/audit.go`.
- Docs: `site/.../aprender/seguranca.md`, `referencia/seguranca.md`; `SECURITY.md`; CI govulncheck.
