# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/);
versionamento semântico.

## Não lançado

### Adicionado
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
