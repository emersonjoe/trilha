# Tasks: Grupos de rota, adaptador html/template e recarga de estáticos

## Phase 1: US1 — Grupos de rota (P1)

- [x] T001 [US1] Árvores `testdata/apps/groups`, `err_group_dup`, `err_group_dynamic` e testes de tabela em `internal/scan/scan_test.go` (padrões sem grupo, ordem de layouts, `E_DUPLICATE_ROUTE`, `E_BAD_SEGMENT`)
- [x] T002 [US1] Implementar grupo no scanner (`parseSegment` kind group, `patternOf`, duplicatas)
- [x] T003 [US1] Golden do gerador para `groups` (`make golden`) e coluna ORIGEM no `trilha routes`
- [x] T004 [US1] Exemplo: `app/marketing-/{layout.go,precos,sobre}` e `app/painel-/{layout.go,middleware.go,painel}`; regenerar `trilha_gen.go`
- [x] T005 [US1] Integração em `examples/blog/blog_test.go`: cenários 1, 2, 4, 6

## Phase 2: US2 — html/template (P2)

- [x] T006 [US2] Testes `tmpl/tmpl_test.go`: render dentro de nó, escape, template inexistente → erro, `Must` com fs
- [x] T007 [US2] Implementar `tmpl/tmpl.go`
- [x] T008 [US2] Exemplo: `app/painel-/relatorio/{page.go,relatorio.html}` com `embed`; integração cenários 1–3

## Phase 3: US3 — estáticos sem rebuild (P3)

- [x] T009 [US3] Testes do watcher: lote só `public/` → `StaticOnly`; misto → rebuild; `public/` novo → rebuild
- [x] T010 [US3] Implementar `Change` no watcher e o atalho no servidor
- [x] T011 [US3] Estender `scripts/measure-reload.sh` com a medição de CSS (< 0,5 s)

## Phase 4: Polish

- [x] T012 README (grupos, tmpl, reload de estáticos), contratos em `specs/001-trilha-core/contracts/conventions.md` atualizados, `make test` verde
