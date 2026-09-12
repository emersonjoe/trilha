# Tarefas — spec 134

Teste antes do código, uma rodada de `make test` por bloco.

- [x] **T001** Testes que falham: `TestSessionsContextLevaOPrazoEOErro` em `auth/local_test.go`
      (store com `SessionListerContext`: contexto da requisição, cancelamento e erro nas duas
      operações; `MemoryStore` inalterado) e `TestDoisPublicosNoMesmoProcesso` no mesmo
      arquivo (dois `Auth` no mesmo mux, cookie e `Store` compartilhados, cada um
      só enxerga o seu) — SC-001 a SC-005.
- [x] **T002** `SessionListerContext`, `listSessions` e as duas operações por ele (SC-001,
      SC-002, SC-003).
- [x] **T003** `Options.Audience`, `a.ctxKey()`, `remember` como método, `User.Audience`
      carimbado no `write`, `Session` e `User` recusando o que não é do público (SC-004,
      SC-005).
- [x] **T004** `make api`, referência de `auth` em inglês e em português (SC-006).
- [x] **T005** `CHANGELOG.md`, `const version = "0.113.0"`, linha do `ROADMAP.md`.
- [x] **T006** `make test` verde, `verifica-trilha.sh --sem-testes` e revisão do diff.
