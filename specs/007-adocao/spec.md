# Feature Specification: Atrito de adoção (issues #5–#9)

**Feature Branch**: `007-adocao` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "dê prioridade para resolver essas issues" — #5, #6, #7, #8, #9, relatadas ao
adotar o Trilha num app Go existente (~76 rotas, PWA com upload e estáticos versionados).

## User Scenarios & Testing

### US1 - `go get @latest` traz a API documentada (#5, P1)
Cortar **v0.2.0** na `main` após estas correções e passar a marcar tag a cada spec fechada
(regra registrada em GOVERNANCE.md e na memória do projeto).
**Acceptance**: `go get github.com/emersonjoe/trilha@latest` resolve `v0.2.0`; release no
GitHub com notas; `trilha version` mostra a versão.

### US2 - Config antes do `New` e sem campos mortos (#6, P1)
Duas frentes: (a) `setup.go` pode exportar `func Config(cfg *trilha.Config)`, chamada pelo
arquivo gerado **antes** de `trilha.New(cfg)`; (b) os campos derivados em `New` (`Logger`,
`Secret`/`PreviousSecret`, `RateLimit`, `TrustedProxies`) são reaplicados quando o servidor
começa (`ListenAndServe`, `Handler`, `Export`), de modo que mudanças em `Setup` também valem.
**Acceptance**: app com `Config` que troca o `Logger` loga pelo logger novo; `Setup` que
liga `RateLimit` produz 429; doc de `App.Config` lista os campos e quando cada um é lido.

### US3 - "Sem timeout" expressável (#7, P2)
`trilha.NoTimeout` (`-1`) desliga um limite; zero continua "padrão".
**Acceptance**: `Timeouts{Read: NoTimeout}` vira `ReadTimeout: 0` no `http.Server`; doc
explica `Write` × streaming (`Ctx.Stream`/`NoWriteDeadline`).

### US4 - Cabeçalhos de estáticos configuráveis (#8, P2)
`Config.StaticCacheControl` (string) substitui o padrão de produção; `Config.StaticHeaders
func(name string, hdr http.Header)` roda por arquivo depois dos padrões e pode mudar qualquer
cabeçalho (immutable para fontes com hash, `Cross-Origin-Resource-Policy`, etc.).
**Acceptance**: teste com os dois; em dev continua `no-cache` salvo se `StaticHeaders` mudar.

### US5 - Middleware passa valores a código stdlib (#9, P1)
`Ctx.SetContext(ctx)` e `Ctx.SetRequest(r)`.
**Acceptance**: middleware que põe o nonce no contexto; handler lendo `c.Request().Context()`
enxerga; o `Ctx` não guarda nada derivado de `c.r` que fique obsoleto.

## Requirements
- FR-001 Nenhuma quebra de API; só adições. Goldens do gerador regravados (`Config`).
- FR-002 `examples/blog` exercita `Config` (cache imutável dos estáticos) e o teste cobre.
- FR-003 Docs: referência App (tabela de campos × quando são lidos), Ctx, Convenções
  (`setup.go` com `Config`), CHANGELOG, README (tabela de convenções).
- FR-004 Fechar as issues com comentário apontando commit e versão.
