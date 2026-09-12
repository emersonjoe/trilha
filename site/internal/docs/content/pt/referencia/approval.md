---
title: approval
description: Approvals, Request, Decide, On, Inbox e os estados — a API do pacote approval, a fila que espera uma pessoa.
---

`import "github.com/emersonjoe/trilha/approval"` — a outra metade do trabalho de uma aplicação: a
parte que espera uma pessoa. O pacote [task](/pt/referencia/task) roda o que a máquina termina
sozinha; este é a fila que todo app de negócio ganha de qualquer jeito — coisas que alguém precisa
aprovar, rejeitar, ou deixar vencer.

Ele responde as quatro coisas que essa fila sempre precisa e que ninguém escreve na primeira vez:
**um dono, um prazo, o motivo registrado, e quem decidiu.**

```go
var Fila = approval.New(approval.Options{
	Roles: func(c *trilha.Ctx) []string { return sessao.Atual(c).Roles },
})

id, err := Fila.Open(c, approval.Request{
	Kind:    "eliminacao",           // agrupa a fila, e é para o que o On é registrado
	Subject: "Listagem 2024/07",     // a linha que alguém lê
	Target:  "/admin/retencao/123",  // onde mora a coisa que está sendo decidida
	Assign:  approval.Role("cpad"),  // ou approval.User(id)
	Due:     time.Now().Add(72 * time.Hour),
})

err = Fila.Decide(c, id, approval.Approved, "ok pelo quórum")
```

## O que ele decide por você, e o que não

**Quem pode decidir é resposta do pacote**, conferida dentro do `Decide` e não na tela: uma tela
que esconde um botão é uma tela, e o endereço atrás dele continua sendo um endereço. O `Roles` é
como um pedido atribuído a um papel encontra a gente dele — uma função que você passa, porque este
pacote não sabe como você autentica, e uma checagem que ele adivinhasse pareceria garantia sem
ser uma. O `MayDecide(c, rec)` é essa mesma checagem, exportada para a tela perguntar ao pacote
em vez de reimplementar a regra: os botões que ela desenha e a decisão que o `Decide` aceita
nunca podem discordar. Quando discordam, o `Decide` responde `approval.ErrNotYours`; um id que
não é pedido responde `approval.ErrUnknown`. A quem o pedido é atribuído é um
`approval.Assignee` — o que o `approval.Role(nome)` e o `approval.User(id)` constroem.

**O que uma decisão significa é seu.** O `On(kind, fn)` roda depois de a decisão ser gravada, e é
ali que a aplicação apaga a coisa, manda o e-mail ou emite o webhook. O erro dele **não desfaz a
decisão**: uma pessoa escolheu, e está registrado; um servidor de e-mail fora do ar não é motivo
para fingir que ela não escolheu. Fazer esse trabalho sobreviver a uma falha é do gancho — uma
[tarefa](/pt/referencia/task), um [webhook](/pt/referencia/webhook).

**O prazo vence sozinho**, num relógio deste processo, pelo mesmo motivo que o `task` varre o
dele: uma aplicação que precisa de um cron para estar certa é uma aplicação errada no dia em que o
cron não roda. A varredura é o `Expire(ctx)`, exportado e devolvendo quantos fechou, para um
teste mover o prazo na mão e conferir o número em vez de esperar um tique.

## Os estados

`pending`, `approved`, `rejected`, `withdrawn`, `expired` — um `trilha.Enum` registrado
(`approval.States`), então o [`ui.Status`](/pt/referencia/ui) já os colore e a tag `enum=` já os
valida sem a aplicação declarar a lista uma segunda vez.

Em Go são constantes: `approval.Pending`, `approval.Approved`, `approval.Rejected`,
`approval.Withdrawn` e `approval.Expired`. As quatro primeiras são decisão de alguém e entram
no `Decide`; `Expired` é o único estado que o pacote escreve sozinho, e por isso não é uma
decisão que se possa passar.

## A tela

```go
ui.Inbox(c, linhas, ui.InboxOpts{Decide: "/admin/aprovacoes", CSRF: trilha.CSRFInput(c)})
ui.InboxBadge(len(pendentes))   // o número no item de menu; zero não desenha nada
```

Uma tabela e dois formulários, sem JavaScript. O prazo é escrito pelo `ui.Relative`, então "em 3
dias" e "há 2 dias" são a mesma frase no idioma de quem lê, e a linha atrasada leva `ui-late`. A
decisão e o motivo viajam **no mesmo formulário**: um motivo digitado num campo que um segundo
clique descarta é um motivo que ninguém escreveu.

O [`trilha add approvals`](/pt/referencia/cli#trilha-add) escreve o pacote ligado, a tela e o
teste.

@demo ui-aprovacoes

## Store

`Memory()` é o padrão e o certo para um processo só. Uma tabela atrás dos mesmos três métodos —
`Save`, `Get`, `List` — é o passo seguinte, e nenhuma tela muda: o framework não é dono do seu
esquema.
