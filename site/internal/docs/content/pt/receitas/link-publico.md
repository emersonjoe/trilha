---
title: Link público
description: O formulário que alguém de fora preenche e o código que confere um documento — os dois sem login e sem tabela de tokens.
---

Três fluxos de toda aplicação interna acontecem sem login: alguém de fora preenche um
formulário, alguém confere um documento por um código, alguém responde a um pedido que chegou
por e-mail. O que se escreve para eles é uma string aleatória numa tabela, em claro, sem prazo —
e token em URL que é curto o bastante para ser adivinhado é adivinhado.

O `c.Link` e o `c.Claim` são esse padrão, com as partes que ninguém lembra já no lugar.

## O link

```go
// InviteLink is what somebody inside the application sends to somebody
// outside it. The link is the credential: it says what it is for, who it is
// about and until when, all signed, and it needs no row in a table.
//
// Uses: 1 is the difference between a form and a form somebody can fill in
// twice. It is the only part that needs storage, and only because "how many
// times has this been used" cannot be answered by arithmetic.
func InviteLink(c *trilha.Ctx, taskID string) (string, error) {
	return c.Link("form", trilha.LinkOpts{
		Data: map[string]string{"task": taskID},
		TTL:  7 * 24 * time.Hour,
		Uses: 1,
		Path: "/form",
	})
}
```

`Uses: 1` é a diferença entre um formulário e um formulário que dá para preencher duas vezes. É
também a única parte que precisa de estado, e só porque "quantas vezes isto foi usado" não é
coisa que aritmética responda.

## A rota que ele abre

```go
// PublicForm is the route the link opens. There is no session here and there
// is not supposed to be one.
//
// Every way the link can fail — wrong signature, wrong purpose, expired,
// already used — answers the same 404. Telling a stranger which of the four
// happened tells them how close they are.
func PublicForm(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim("form")
	if err != nil {
		return nil, err
	}
	return h.Div(
		ui.H1(h.Text("Fill in your details")),
		h.Form(h.Method("post"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("task"), h.Value(link.Data["task"])),
			ui.Submit(h.Text("Send"))),
	), nil
}
```

**Toda forma de o link falhar responde o mesmo 404** — assinatura errada, fim errado, vencido,
já gasto. Dizer a um estranho qual das quatro aconteceu é dizer o quão perto ele está. E token
errado custa ao endereço que o mandou um ponto de um orçamento pequeno: adivinhar token em URL é
força bruta, e a resposta à força bruta é errar sair caro.

## Gastando

```go
// SubmitPublicForm spends the link — after the work and not before. A link
// burned by a validation error is a link somebody has to ask for again
// because they typed a date wrong.
func SubmitPublicForm(c *trilha.Ctx) error {
	link, err := c.Claim("form")
	if err != nil {
		return err
	}
	if err := saveAnswer(c, link.Data["task"]); err != nil {
		return err
	}
	if err := link.Consume(); err != nil {
		return err
	}
	return c.Render(http.StatusOK, ui.H1(h.Text("Thank you")))
}
```

O `Consume` vem **depois** do trabalho, não antes. Link queimado por erro de validação é link que
a pessoa tem de pedir de novo porque digitou uma data errada. Abrir a página também não gasta —
senão um recarregar queimaria o link de quem ainda está preenchendo.

## O mesmo, sem limite

```go
// VerifyCode is the same primitive with no limit at all: a code printed on a
// document, checked as many times as anybody likes until it expires. Nothing
// is stored and nothing is looked up — verifying is a signature check.
func VerifyCode(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim("verify")
	if err != nil {
		return nil, err
	}
	return ui.H1(h.Text("Document " + link.Data["doc"] + " is authentic")), nil
}
```

Com `Uses: 0` não há estado nenhum: nem linha, nem consulta, nem limpeza. Conferir é checar uma
assinatura, e o link funciona até vencer. É o código de verificação impresso num documento.

:::warning
**O que está no link é assinado, não secreto.** Quem tem o link lê o `Data` — é base64, não
cifra. Ponha um id ali, não um nome, um preço ou um motivo. O que não pode ser lido mora na sua
tabela, achado por esse id.
:::

O `Config.Links` conta os usos dos links com limite; nil conta no processo, o que é honesto sobre
uma réplica e é dito uma vez no log. Um de verdade é um `UPDATE ... WHERE usos < max` ou um
`INCR` do Redis — a interface tem dois métodos e o difícil é atômico de propósito.
