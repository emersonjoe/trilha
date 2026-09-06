# Tasks: Cookbook, checklist de produção e guia de migração

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — a garantia (SC-002)

- [x] T001 Teste que falha em `site/site_test.go`: todo bloco ```go` das páginas do cookbook
      aparece literalmente em um `.go` do repositório.
- [x] T002 `examples/cookbook/`: `doc.go`, `db.go`, `sessions.go`, `uploads.go`,
      `pagination.go`, `email.go`, `jobs.go`.

## Bloco 2 — as páginas (SC-001, SC-005)

- [x] T003 `content/en/cookbook/`: índice, `database`, `sessions`, `uploads`, `pagination`,
      `email`, `scheduled-tasks`, `docker`, `production-checklist`, `migration`.
- [x] T004 `content/pt/receitas/`: as dez traduções, mesmos slugs na mesma ordem.

## Bloco 3 — ligação (SC-004)

- [x] T005 Seção em `site/internal/docs/docs.go` nas duas locales.
- [x] T006 Rotas: `site/app/cookbook/**`, `site/app/pt/receitas/**`, atalho
      `site/app/receitas/**`; `trilha gen` no site.

## Bloco 4 — fechamento

- [x] T007 `CHANGELOG.md` (0.29.0), `version` em `cmd/trilha/main.go`, ROADMAP (item 20).
- [x] T008 `make test` verde e `make release VERSION=0.29.0 ISSUES="38"`.
