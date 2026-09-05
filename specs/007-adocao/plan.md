# Implementation Plan: 007-adocao

Constitution Check: I (nova função opcional `Config` em `setup.go`, com exemplo e teste),
II (stdlib), III (gerador emite `app.Config(&cfg)`, golden atualizado), IV (Ctx ganha dois
métodos), V (sem impacto), VI (testes antes), VII (nada afrouxa: `NoTimeout` é opt-in).

- `ctx.go`: `SetContext`, `SetRequest`.
- `trilha.go`: `NoTimeout`; `or()`; `applyConfig()` extraído de `New` e chamado também em
  `Handler`, `ListenAndServe`, `Export`; campos `StaticCacheControl`, `StaticHeaders`; doc de
  `Config()`.
- `static.go`: usa os campos.
- `internal/scan`: `Result.ConfigFunc` (opcional, em `setup.go`); `internal/gen`: chama antes
  de `New`. `make golden`.
- `examples/blog/app/setup.go`: `Config` com `StaticCacheControl`; teste.
- Docs e CHANGELOG; tag `v0.2.0`; fechar issues.
