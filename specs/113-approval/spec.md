# Spec 113 — `approval`: a fila de trabalho que espera uma pessoa

- **Issue**: [#147](https://github.com/emersonjoe/trilha/issues/147) — a issue é a fonte do escopo.
- **Branch**: `113-approval`
- **Versão**: 0.92.0

## Por quê

Todo app de negócio acaba com uma caixa de coisas que alguém precisa aprovar. Medido no Acervo:
464 linhas de tela e ~220 de rota para um padrão que se repete — uma fila com dono, prazo,
estado, decisão registrada e quem decidiu.

O `task` (0.64.0) é para trabalho que roda sozinho. Isto é o outro: trabalho que **espera uma
pessoa**, e que por isso precisa de prazo, de justificativa e de trilha.

## O que muda

Um pacote `approval` com a forma do `task`, e a tela dele no kit:

```go
var Aprovacoes = approval.New(approval.Options{})

id, err := Aprovacoes.Open(c, approval.Request{
	Kind:    "eliminacao",
	Subject: "Listagem 2024/07",
	Target:  "/admin/retencao/123",
	Assign:  approval.Role("cpad"),
	Due:     time.Now().Add(72 * time.Hour),
})

err = Aprovacoes.Decide(c, id, approval.Approved, "ok pelo quórum")
```

- **Os estados são um `trilha.Enum` registrado** — `pending`, `approved`, `rejected`,
  `withdrawn`, `expired` — então o `ui.Status` já os colore e o `enum=` já os valida.
- **`Decide` audita sozinho**, com o ator que o `auth` pôs na requisição: quem decidiu é a
  primeira pergunta de qualquer conversa sobre uma decisão.
- **`On(kind, fn)`** roda quando a decisão cai, e é ali que o app faz o que a decisão significa —
  eliminar, mandar e-mail, emitir webhook. O pacote não sabe o que uma aprovação aprova.
- **`Inbox(c)`** é o que a pessoa da vez pode decidir: por papel ou por id, lido da sessão.
- **O prazo vence sozinho**, no mesmo relógio do `task`: um pedido vencido é `expired` e não uma
  linha pendente para sempre.
- **`ui.Inbox` e `ui.InboxBadge`**: a caixa com abas, prazo em `ui.Relative`, atraso destacado,
  os dois botões com `ui.Confirm` e o campo de justificativa.
- **`trilha add approvals`**: o pacote ligado, a tela sob a pasta pedida, e o teste.

## Fora de escopo

- **Fluxo com várias etapas e condições.** Isso é um motor, não uma caixa; encadear é o `On`
  abrindo o próximo pedido.
- **Delegação e férias.**
- **Store em SQL.** Memória aqui, tabela na receita do cookbook, como em todos os módulos.
- **O cenário do `bench/agent`.** Fica na #94, com os outros.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `sync`, `time`, e o que o repositório já tem |
| VI — teste primeiro | abrir, decidir, vencer, e o gancho que roda |
| VII — segurança por padrão | quem decide é quem a caixa deixa decidir, conferido no `Decide` |

## Tarefas

- [x] T001 Teste que falha: abrir, decidir, o gancho, o prazo e quem pode
- [x] T002 O pacote `approval`: estados, store em memória, relógio, auditoria
- [x] T003 `ui.Inbox` e `ui.InboxBadge`
- [x] T004 A receita `trilha add approvals`
- [x] T005 Documentação (en + pt) e superfície de API
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.92.0`

## Aceitação

- **SC-001** `Open` grava pendente; `Decide` grava a decisão, o motivo e quem.
- **SC-002** Quem não é o dono do pedido não decide.
- **SC-003** Prazo vencido vira `expired` sem cron externo.
- **SC-004** O `On` do tipo roda depois da decisão, e um erro dele não desfaz a decisão.
- **SC-005** A decisão aparece na trilha de auditoria sem código do app.
