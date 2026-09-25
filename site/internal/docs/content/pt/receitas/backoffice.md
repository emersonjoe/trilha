---
title: Um backoffice que nega por padrão
description: trilha add admin — app/admin/ fechado a todo papel que não é admin, com usuários e papéis, a trilha de auditoria, a caixa de aprovações e a busca, e toda decisão na trilha com quem e sobre quem.
---

Um backoffice são quatro telas que todo app tem — pessoas e seus papéis, a trilha de quem fez o
quê, as decisões esperando alguém, uma busca — atrás de uma porta. As telas são a metade fácil. A
porta é a metade que dá errado: uma tela nova no mês que vem, na mesma pasta, e ninguém lembrou
de guardá-la.

## O comando

```bash
trilha add login admin
```

`trilha add admin` é **feita de receitas**: aplica [`users`](/pt/referencia/cli#trilha-add), `audit`,
`approvals` e `search` sob `app/admin/` — as mesmas receitas, com os mesmos testes, e não cópias
das telas — e acrescenta o que nenhuma delas tem: a porta e a tela de entrada.

## O que instala

| Onde | O quê |
|---|---|
| `app/admin/middleware.go` | a porta: só o papel `admin` passa |
| `app/admin/page.go` | a tela de entrada, com as decisões esperando |
| `app/admin/usuarios/` | convidar, trocar papel, desativar, resetar |
| `app/admin/auditoria/` | a trilha, desenhada com `ui.AuditTable` |
| `app/admin/aprovacoes/` | a caixa, desenhada com `ui.Inbox` sobre o `approval` |
| `app/admin/busca/` | a busca sobre o que o app indexou |
| `admin_test.go` | a porta em tabela, e as decisões na trilha |

## Preço

`trilha ctx --pack admin` custa **~104 tokens (est.)**, medido num projeto mínimo pelo
`TestRecipeCtxPackCost`. As receitas de que ela é feita têm os próprios preços no
`trilha add --list`.

## Negar por padrão

O middleware guarda a pasta e tudo abaixo dela — inclusive a tela escrita ali amanhã. Uma tela
nova nasce fechada até alguém abri-la de propósito, e não aberta até alguém lembrar de fechar.
Quem entrou sem o papel recebe `403`, e não o login de novo: é conhecido, só não tem permissão.

```go
// exige is the rule, and Middleware is what the scanner reads: middleware.go
// has to export a function with that signature, and a var of the right type
// is not one.
var exige = sessao.Flow.RequireRole("admin")

// Middleware guards this folder and everything below it — the screens written
// tomorrow included. That is what deny by default means here: a new screen is
// closed until somebody opens it on purpose, not open until somebody
// remembers to close it.
//
// Somebody signed in without the role gets 403 and not a redirect to the
// login: they are known, just not permitted, and sending them back to a login
// they already passed is a loop with no exit.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
```

O teste que segura a porta é uma tabela: cada tela sob `/admin` contra o anônimo, alguém que
entrou sem o papel e o admin. Uma tela acrescentada à lista é uma linha.

```go
// telasDoAdmin is every screen under /admin. A screen written there tomorrow
// is one more row, and the table says who may open it.
var telasDoAdmin = []string{"/admin", "/admin/usuarios", "/admin/auditoria", "/admin/aprovacoes", "/admin/busca"}
```

## Toda decisão na trilha

Trocar o papel de alguém, desativar uma conta, resetar uma senha, convidar, aprovar um pedido:
cada uma é escrita com `c.Audit`, com o admin como ator e a pessoa ou o pedido como alvo, e o
`/admin/auditoria` mostra. O `TestAdminAuditTrail` e o `TestAdminApprovalFlow` seguram isso no
seu projeto.

## Como estender

Ponha a próxima tela sob `app/admin/` e ela está guardada. Se o projeto já tinha `users`,
`audit`, `approvals` ou `search` na raiz, as telas agora existem também sob `/admin` — apague as
pastas antigas para ficar com uma porta só. [O app administrável](/pt/receitas/app-administravel)
é o mesmo conjunto, escrito na criação pelo `trilha new --template app`.
