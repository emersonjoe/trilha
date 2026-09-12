# Tarefas — spec 131

Teste antes do código, uma rodada de `make test` por bloco.

- [x] **T001** Árvores sintéticas em `internal/migrate/testdata/` — `unused-import/`,
      `server-actions/`, `media/` — e os três testes que falham:
      `TestSinalDeModuloSegueOsNomesImportados` (SC-001, SC-002),
      `TestServerActionsAparecemNoRelatorio` (SC-003, SC-004),
      `TestCapturaDeMidiaEhIlha` (SC-006).
- [x] **T002** `decls.go`: `declsOf`, `usedSource`, `identsIn`. `deps.go`: `dep`, `bindings`, o
      import não usado que não é seguido. `analyze.go`: `hit.whole` e `, whole module` (SC-001,
      SC-002).
- [x] **T003** `Action`, `actionsOf`, `Project.Actions`, a seção *Server Actions* nas duas
      línguas, `· N server actions` na coluna Sugestão e a linha do comentário gerado (SC-003,
      SC-004).
- [x] **T004** `--actions` na CLI, com i18n e teste em `cmd/trilha/migrate_test.go` (SC-005).
- [x] **T005** Sinais `media capture` e `media playback`, mais o parágrafo da regra nas duas
      línguas (SC-006).
- [x] **T006** `testdata/next` com o caso do relatório inteiro, `make golden`, doc do
      `trilha migrate` em `en/` e `pt/` (SC-007).
- [x] **T007** `CHANGELOG.md`, `const version = "0.110.0"`, linha do `ROADMAP.md`.
- [x] **T008** `make test` verde, `verifica-trilha.sh --sem-testes` e revisão do diff.
