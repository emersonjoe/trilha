# Feature Specification: Issues #10–#14 (dev reload, páginas de erro, kind de rota, main/shutdown, pastas com ponto)

**Feature Branch**: `008-issues-10-14` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "resolva essas issues" — https://github.com/emersonjoe/trilha/issues (#10 a #14),
relatadas na migração incremental de um app Next.js para o Trilha.

## User Scenarios & Testing

### US1 - Desligar a injeção do live reload (#10, P1)
`Config.DevReload string`: `trilha.Off` desliga a injeção do `<script>` de recarga em Dev;
`TRILHA_DEV_RELOAD=off` faz o mesmo pelo ambiente (sem tocar no código do app). Stack
traces e `no-cache` continuam ligados em Dev.
**Acceptance**: em Dev com `DevReload: Off` o HTML não contém `/_trilha/events`; com a
variável de ambiente idem; sem nada, contém.

### US2 - Página de erro que já respondeu não ganha segundo corpo (#11, P1)
`writeHTML` não escreve se a resposta já começou; `not_found.go`/`error.go` (e `page.go`)
que devolvem `(nil, nil)` depois de escreverem a resposta são tratados como "já respondido".
Se devolvem `nil` **sem** escrever nada, vale a página simples do framework (404/500), e no
caso de `page.go`, 204 (comportamento já documentado para handlers que não escrevem).
**Acceptance**: `NotFound` que chama `http.NotFound` responde `text/plain` 404 sem HTML
anexado e sem `superfluous WriteHeader` no log; `Error` idem; `Page` que escreve e devolve
nil não recebe documento em cima.

### US3 - `route.go` que serve HTML recebe erro em HTML (#12, P1)
Duas regras, da implícita à explícita:
1. Desempate por requisição: rota de `route.go` cujo `Accept` contém `text/html` e não
   contém `application/json`, fora de `/api/`, recebe a página de erro em HTML. `fetch`
   sem `Accept` (`*/*`), `curl` e clientes JSON continuam recebendo JSON.
2. Explícito: `route.go` pode exportar `var Kind = trilha.KindPage` (ou `KindAPI`); o
   gerador passa `Kind` na `Route` e ele vence o desempate. `KindPage` também liga a
   verificação de CSRF nos métodos com corpo, como em `page.go`.
**Acceptance**: `route.go` com `GET` que devolve `Errorf(403)`: navegador (`Accept:
text/html`) recebe HTML 403; `Accept: application/json` recebe JSON; `Kind = KindAPI`
força JSON mesmo para navegador; `Kind = KindPage` exige CSRF no `POST`.

### US4 - `main` próprio, desligamento e `public/` opcional (#13, P1)
- `public/` já é opcional desde a 0.2.0 (`//go:embed` só quando a pasta tem arquivos);
  documentar.
- `setup.go` pode exportar `func Shutdown(a *trilha.App) error`, chamado após o
  `Shutdown` do servidor; `a.OnShutdown(fn)` registra ganchos por código; `Timeouts.Shutdown`
  define a espera (padrão 5 s).
- Se o pacote `main` do projeto já tem `func main()` em outro arquivo, o gerador omite o
  `main` (o dev sobe o binário do mesmo jeito). Não precisa de flag nem de estado: a
  decisão é derivada da árvore, como tudo no gerador.
**Acceptance**: golden/teste do gerador com `main.go` próprio não contém `func main`;
`Shutdown` aparece no gerado quando existe; teste de `ListenAndServe` chama os ganchos.

### US5 - Pasta com ponto no nome (#14, P2)
Suportado oficialmente: `app.css/route.go` → `/app.css`; o pacote pode ter outro nome
(`package appcss`) porque o gerador importa com alias. Teste no scanner e no gerador,
documentação em Convenções e em Páginas e rotas.

## Requirements
- FR-001 Sem quebra de API; só adições (`Config.DevReload`, `Timeouts.Shutdown`,
  `App.OnShutdown`, `Route.Kind`, `RouteKind`/`KindAuto`/`KindPage`/`KindAPI`).
- FR-002 Goldens do gerador regravados quando o template muda; determinismo mantido.
- FR-003 CHANGELOG, docs (App e Config, Erros, Convenções, Desenvolvimento e produção,
  CLI) e fechamento das issues com comentário apontando o commit.
