# Plano — spec 056

## Arquivos

| Arquivo | O quê |
|---|---|
| `trilha.go` | `Config.Upstreams map[string]Upstream` |
| `upstream.go` (novo) | `type Upstream`, `parseUpstreams`, `serveUpstream` |
| `serve.go` | chamada no `fallback`, entre o 405 e o redirect de barra |
| `upstream_test.go` (novo) | SC-001 a SC-009, com `httptest.Server` de upstream |
| `auth/session.go` | `User.Extra`; `Login`/`Logout` exportados |
| `auth/flow.go` | `Sessions(Options)`; `Start`/`Callback`/`Logout` sem provedor; `OnLogin` |
| `auth/middleware.go` | `RequireFunc(pred)` |
| `auth/pbkdf2.go` (novo) | `HashPBKDF2`, `CheckPBKDF2`, `PBKDF2` |
| `auth/local_test.go` (novo) | SC-010 a SC-015 |
| `cmd/trilha/audit.go`, `i18n.go`, `audit_test.go` | SC-016 |
| `examples/local-login/**` | SC-017 |
| docs en+pt, `SECURITY-MODEL.md`, `CHANGELOG.md`, `api/current.txt` | SC-018 |

## Ordem

O runtime primeiro (o exemplo depende dele), depois o `auth`, depois o exemplo que usa
os dois, depois auditoria e documentação. `make test` por bloco, não por arquivo.

## Riscos

- **O proxy no fallback pega o 404 de todo mundo.** Mitigado por prefixo explícito: só
  entra quando o caminho começa com um prefixo declarado.
- **`ReverseProxy` escreve 502 em HTML por padrão.** `ErrorHandler` próprio devolve
  `problem+json` pelo caminho do framework.
- **Cookie de sessão vazando para o upstream.** O `Cookie` do cliente é repassado (a API
  pode ter sessão própria); o que não é repassado é o `Authorization`. Documentado.
