# Implementation Plan: 172 — Redirect seguido, formulário na região e URL declarada

**Spec**: `spec.md` | **Issues**: #291, #292, #293

## Constitution Check

- [x] I — sem convenção nova em `app/`; o fixture do `uitest` usa as convenções existentes.
- [x] II — só biblioteca padrão no runtime; JavaScript do kit sem dependência.
- [x] IV — API aditiva: `RedirectReload` (função e método), `Ctx.PushURL`, `Ctx.ReplaceURL`,
  `RedirectError.Reload`, `ui.PushHistory`, `ui.Follow`, `ui.NoFollow`; `api/current.txt` e
  catálogo do `uidoc` atualizados.
- [x] VI — teste primeiro: servidor (`follow_test.go`) e navegador (oito cenários) reprovaram
  antes do código; jornada e risco na spec; catálogo atualizado.
- [x] VII — só mesma origem nos cabeçalhos seguidos; `localPath` no servidor.
- [x] `make test`, `make test-ui` com `UITEST_BROWSERS=all` e `make security` (na release).

## Estrutura

- Raiz: `render.go` (cabeçalhos, `Trilha-Reload`), `flash.go` (modo seguir), `errors.go`
  (`RedirectReload`), `ctx.go` (`RedirectReload`, `PushURL`, `ReplaceURL`).
- `ui/`: `ui.go` (atributos), `assets/ui.js` (`follow`, `follows`, `record`, flash guardado),
  `assets/ui.nav.js` (submit, redirect do link, `navigate`/`navRegion`), `assets/ui.upload.js`.
- `bench/agent/measure.go`: `snapshotTree`/`countWritten`.
- `uitest/`: fixture `/fluxos/itens/*`, `/fluxos/semregiao`, `internal/itens`; cenários.

## Orçamentos

`ui.js` 36 KB (era 32) e `ui.nav.js` 10 KB (era 6), com o motivo no teste de tamanho.
