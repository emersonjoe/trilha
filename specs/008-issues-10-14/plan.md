# Implementation Plan: 008-issues-10-14

## Constitution Check
| Princípio | Como |
|---|---|
| I Convenção | `Shutdown` e `Kind` são exports opcionais nos arquivos já convencionados (`setup.go`, `route.go`); `main` próprio é detectado, não configurado |
| II Só stdlib | sem deps |
| III Geração explícita | `HasMain`, `ShutdownFunc` e `Kind` vêm do scanner (`go/ast`), gerador determinístico |
| IV Contrato | `(nil, nil)` após escrever = já respondido; assinaturas inalteradas |
| V Dev/prod | `DevReload` só muda a injeção; stack/no-cache seguem em Dev |
| VI Teste primeiro | testes em render/serve (#11, #12), trilha (#10, #13), scan/gen (#13, #14) |
| VII Segurança | `KindPage` liga CSRF; desempate por `Accept` não muda CSRF (rota de `route.go` segue sem CSRF salvo `CSRFForAPI`) |

## Design
- `render.go`: `writeHTML` retorna cedo se `c.w.wrote`; `renderPage` pula layouts com nó nil;
  `renderNotFound`/`renderErrorPage` tratam `(nil, nil)`.
- `serve.go`: `kindOf(r)` honra `r.Kind`; `wantsHTML(req)`; aplicado em `wrap` e no 405.
- `trilha.go`: `DevReload`, `TRILHA_DEV_RELOAD`, `Timeouts.Shutdown`, `OnShutdown`, ganchos em ordem inversa após `srv.Shutdown`, erros com `errors.Join`.
- `scan`: `ShutdownFunc *Ref`, `HasMain bool` (parse dos `.go` da raiz, exceto `trilha_gen.go`, procurando `func main()` em `package main`), `Route.HasKind bool` (var/const `Kind` exportado em `route.go`).
- `gen`: template com `OnShutdown`, `Kind: alias.Kind`, `main` condicional.
- testdata: `apps/custom_main` (main.go + app/page.go + setup.go com Shutdown), `apps/dotdir` (app/app.css/route.go `package appcss`, com `var Kind = trilha.KindPage`).
