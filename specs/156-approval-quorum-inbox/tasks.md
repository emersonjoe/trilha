# Tasks: 156-approval-quorum-inbox

- [x] T001 Teste que falha em `approval/approval_test.go`: `Decide` com `Quorum: 3` mantém
      `Pending` após o 1º e o 2º voto, fecha no 3º; a mesma pessoa votando duas vezes recebe
      `ErrAlreadyVoted`; `Quorum` 0/1 fecha no primeiro voto (regressão)
- [x] T002 Teste que falha em `ui/inbox_test.go`: `InboxRow.Action` preenchida desenha esse
      nó em vez do `decideForm`; sem `Action`, comportamento de hoje (regressão);
      `InboxRow.Progress` aparece ao lado do `Status`
- [x] T003 Teste que falha em `ui/schema_test.go`: `schemaField` com `Type: "display"` e
      `Label` preenchido desenha o `Label`; sem `Label`, comportamento de hoje (regressão)
- [x] T004 Implementação: `approval/approval.go` (`Vote`, `Record.Quorum`, `Record.Votes`,
      `ErrAlreadyVoted`, `Decide`, `Open` grava `Quorum`)
- [x] T005 Implementação: `ui/inbox.go` (`InboxRow.Action`, `InboxRow.Progress`,
      `decideForm` usa `ui.Submit`)
- [x] T006 Implementação: `ui/schema.go` (`schemaField` desenha `Label` no `display`)
- [x] T007 `internal/recipes/approvals.go`: `linhas()` passa `Progress` quando o `Record` tem
      `Votes`, para o inbox do recipe continuar sendo o exemplo canônico do pacote
- [x] T008 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`, referência
      bilíngue em `site/internal/docs/content/{en,pt}/refer{e,ê}ncia/approval.md`
- [x] T009 `make test` verde (inclui `make api` e `go test ./internal/uidoc -update` para os
      catálogos gerados)
