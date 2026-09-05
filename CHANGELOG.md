# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/);
versionamento semântico.

## Não lançado

## 0.3.0 — 2026-09-05

### Adicionado
- Issues #10–#14 (spec 008): `Config.DevReload` e `TRILHA_DEV_RELOAD=off` desligam o
  script de recarga em dev; `Route.Kind` (`KindAuto`/`KindPage`/`KindAPI`) e
  `var Kind = trilha.KindPage` em `route.go`; `App.OnShutdown`, `Timeouts.Shutdown` e
  `func Shutdown(a *trilha.App) error` opcional em `setup.go`; o gerador omite `main()`
  quando o pacote já tem um; pastas com ponto no nome (`app.css/` → `/app.css`) documentadas
  e testadas.

### Corrigido
- `not_found.go`/`error.go`/`page.go` que escrevem a resposta e devolvem `(nil, nil)` não
  recebem um segundo documento em cima (#11).
- Rota de `route.go` acessada por navegador (`Accept: text/html`, fora de `/api/`) recebe a
  página de erro em HTML em vez de JSON (#12).

## 0.2.0 — 2026-09-05

### Adicionado
- Adoção (spec 007, issues #6–#9): `func Config(cfg *trilha.Config)` opcional em `setup.go`,
  chamada antes de `trilha.New`; campos derivados (`Logger`, `Secret`, `RateLimit`,
  `TrustedProxies`) reaplicados ao começar a servir, então mudanças em `Setup` valem;
  `trilha.NoTimeout`; `Config.StaticCacheControl` e `Config.StaticHeaders`;
  `Ctx.SetContext` e `Ctx.SetRequest`.
- IA (spec 005): pacote `ai` (cliente OpenAI-compatível com `Chat`/`Stream`, `Tool`/`Typed`,
  `Agent` com `Run`/`RunStream`, handoffs, `AsTool`, `Parallel`, `Chain`) e `ai/mcp` (cliente
  e servidor MCP por stdio e Streamable HTTP); `c.Stream()` para Server-Sent Events;
  exemplo `examples/assistente`.
- Segurança (spec 004): `Config.Security` (CSP com nonce por requisição, HSTS, Permissions-Policy,
  COOP), `Config.TrustedProxies` e `c.ClientIP()`, `Config.RateLimit` e `trilha.Limit`, cookies
  assinados (`c.SetSigned`/`c.Signed`, `TRILHA_SECRET`/`TRILHA_SECRET_PREVIOUS`), `Config.Timeouts`,
  eventos de segurança (`Config.OnSecurityEvent`), `c.Nonce()`/`trilha.NonceAttr`, `c.NoWriteDeadline()`,
  `a.Config()`, e o comando `trilha audit`.
- `trilha export` e `App.Export`: exportação de rotas estáticas para HTML, com `404.html`
  e cópia de `public/`; `App.AddExportPath` para rotas dinâmicas; `Ctx.Base()` e
  `TRILHA_BASE_PATH` para sites em subcaminho.
- `trilha.Run(app)`: o `main` gerado passa a chamá-lo (serve ou exporta).
- Site de documentação em `site/`, construído com o próprio Trilha e publicado no GitHub Pages.
- Arquivos de comunidade: CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, GOVERNANCE,
  templates de issue e PR, CODEOWNERS, Dependabot.

### Alterado
- `X-Forwarded-Proto`/`X-Forwarded-For` só são considerados vindos de `TrustedProxies`
  (antes, `X-Forwarded-Proto: https` de qualquer origem marcava cookies como `Secure`).
- O arquivo gerado chama `trilha.Run(newApp())` em vez de `ListenAndServe` direto
  (regenerar com `trilha gen`).

## 0.1.0 — 2026-09-05

### Adicionado
- Roteamento por arquivos em `app/`: `page.go`, `route.go`, `layout.go`, `middleware.go`,
  `not_found.go`, `error.go`, `setup.go`; segmentos `nome_`, catch-all `nome__`, grupos `nome-`.
- Runtime: `App`, `Ctx`, erros como valores, CSRF, cabeçalhos de segurança, limite de corpo,
  estáticos embutidos, logs `slog`.
- DSL `h` e adaptador `tmpl` para `html/template`.
- CLI: `new`, `gen`, `dev` (proxy + recarga por SSE, sem rebuild para `public/`), `build`, `routes`.
- Exemplo `examples/blog` e suíte de testes (unitários, golden, integração, e2e).
