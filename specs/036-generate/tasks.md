# Tasks: `trilha generate page|route|component`

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — tradução e escrita (SC-001, SC-002, SC-003, SC-005)

- [x] T001 Teste que falha em `internal/scaffold/generate_test.go`: tabela URL → pasta,
      pacote e padrão (literal, `{slug}`, `{path...}`, grupo `-`, pasta com ponto, raiz);
      pacote lido do `.go` existente; palavra reservada; segmentos inválidos; conflito
      `page`×`route`; componente.
- [x] T002 `internal/scaffold/generate.go`: `Generate`, `GenResult`, `ErrGenExists`,
      `ErrGenConflict` e os três esqueletos.
- [x] T003 Teste em `internal/scaffold/generate_test.go` cruzando com o scanner: a árvore
      gerada, varrida por `scan.Scan`, devolve o padrão que o `Generate` prometeu.

## Bloco 2 — comando (SC-004, SC-006)

- [x] T004 `cmd/trilha/generate.go` com `--force` e `--dir`; despacho em `main.go`.
- [x] T005 `cmd/trilha/i18n.go`: mensagens nas duas línguas e a linha no `usage`.
- [x] T006 E2E em `cmd/trilha/e2e_test.go`: os três subcomandos, a recusa sem `--force`, o
      conflito, e o projeto ainda compilando depois.

## Bloco 3 — documentação e fechamento

- [x] T007 `reference/cli` nas duas locales: o comando e a tabela de tradução.
- [x] T008 `learn/pages-and-routes` nas duas locales: uma nota de que o gerador escreve a pasta
      pela URL.
- [x] T009 `CHANGELOG.md` (0.27.0), `version` em `cmd/trilha/main.go`, ROADMAP (item 18).
- [ ] T010 `make test` verde e `make release VERSION=0.27.0 ISSUES="36"`.
