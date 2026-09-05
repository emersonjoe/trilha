# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/);
versionamento semântico.

## Não lançado

## 0.6.0 — 2026-09-05

### Adicionado
- Observabilidade (spec 014): sondas `/_trilha/health/live` e `/_trilha/health/ready` com
  `App.Check` (prazo, execução em paralelo, cache e resposta `application/health+json`),
  `App.HealthReport`; registro de métricas `App.Metrics()` (`Counter`/`Gauge`/`Histogram`
  com rótulos e teto de cardinalidade) exposto no formato de texto do Prometheus quando
  `Observability.Metrics` está configurado; métricas do framework (requisições, latência,
  em voo, eventos de segurança, pânicos, runtime e `trilha_build_info`); `Ctx.TraceID` e
  `Ctx.Log` com propagação de `traceparent` (W3C Trace Context); `Config.Observability`
  com porteiro por token (`TRILHA_OBS_TOKEN`, comparação em tempo constante) ou rede
  confiável; três itens novos em `trilha audit`; capítulo e referência no site.

### Alterado
- O detalhe da saúde e as métricas ficam fechados por padrão fora de `dev`: sem token nem
  rede confiável, o endereço de métricas responde 401 e a saúde devolve só `status`
  (NIST SP 800-53 AU-9, OWASP API Security 2023 API8).

## 0.5.3 — 2026-09-05

### Corrigido
- Site: a demo "Formulário com CSRF em uma linha" não reagia ao envio — o `onclick="return
  false"` no próprio `<form>` cancelava todo clique dentro dele (spec 013). Agora o envio é
  interceptado por `tema.js` e mostra o fluxo `POST → 303 → GET /eventos/<slug>`; sem
  JavaScript o formulário apenas recarrega a página.
- Nenhum manipulador de evento inline no site nem nos exemplos (a CSP padrão do Trilha os
  bloqueia): resíduo `onchange=""` removido do exemplo de orçamento e teste que varre todas
  as páginas para impedir a volta.

### Adicionado
- Spec 012 (backlog documentado): reduzir o custo fixo por requisição medido na spec 011
  (CSP remontada a cada requisição, nonce sorteado mesmo em rotas de API, mapa de valores
  alocado sempre, log formatado mesmo quando descartado).

## 0.5.2 — 2026-09-05

### Adicionado
- Benchmarks (spec 011): módulo `bench/` comparando com a biblioteca padrão (página, JSON,
  estático, 200 rotas, middlewares), `make bench`/`make bench-results`, `bench/RESULTS.md`,
  página "Desempenho e comparação" no site e job de CI.

## 0.5.1 — 2026-09-05

### Adicionado
- Estatísticas (spec 010): site com contagem sem cookies via GoatCounter, ligada pela
  variável `SITE_ANALYTICS`; `scripts/traffic.sh` e workflow `traffic` (snapshot diário do
  tráfego do repositório na branch `stats`, opcional por `TRAFFIC_TOKEN`).

## 0.5.0 — 2026-09-05

### Adicionado
- Exemplos (spec 009): `examples/cadastro` (médio) e `examples/orcamento` (complexo), com
  READMEs e testes; capítulo "Exemplos" no site.
- `c.Bind(&struct)` (formulário ou JSON, structs aninhadas com prefixo), `trilha.FieldErrors`
  (422 com `fields` em APIs), `c.Render(code, node)` (página com layouts a partir de um POST);
  no kit: `ui.Errors`, `ui.InvalidIf`, `ui.SelectOptions`, `ui.Checked`.

## 0.4.0 — 2026-09-05

### Adicionado
- Kit de interface (spec 006): pacote `ui` (componentes, variantes, ícones Lucide), assets
  `public/ui.theme.css`/`ui.css`/`ui.js` copiados por `trilha new` e `trilha ui`, contrato de
  tema compatível com shadcn/ui v4; exemplos `blog` e `assistente` reestilizados; demos vivas
  no site. `h`: atributos `class` repetidos são fundidos em um só.

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
