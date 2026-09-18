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

## Consulta por código, sem conta

O link é metade do público. A outra metade é a pessoa com um papel na mão: um número de
protocolo, o código impresso num comprovante, mais algo que ela já tem — os últimos dígitos do
telefone com que o caso foi aberto. Link nenhum foi enviado, então não há nada para reivindicar.
Ela digita, e vê o próprio caso.

```sh
trilha add public-lookup
```

Escreve o `app/consulta/` — uma pasta própria, de propósito fora da árvore que o app pôs atrás de
login, porque quem digita um protocolo é justamente quem não tem conta (uma aplicação com dois
públicos dá ao seu fluxo de entrada um `Options.Audience` próprio, e esta pasta fica fora dele) —
mais o `internal/consulta/`, onde moram o `Buscar` e a regra de quais eventos são públicos.

A tela que se escreve à mão no lugar dela é a que responde "protocolo inexistente" para um número
que não existe e "código errado" para um fator que não bate. Isso é um oráculo de graça: quem
está varrendo o espaço de números agora sabe quais são reais, e só lhe falta achar o segundo
fator. Esta aqui tem uma negativa só.

**O dígito verificador vem primeiro.** Um código que alguém digita só deveria chegar ao banco
quando é um código que poderia ter sido emitido:

```go
// Emitir mints a code: the check digits travel with the number, printed on the
// receipt beside it. Two of them catch every single-character typo and every
// swap of two neighbours, and only one code in ninety-seven is worth a query
// at all — which is what makes enumeration expensive before anything counts it.
func Emitir(base string) string { return base + trilha.CheckDigit(base) }
```

O `trilha.CheckDigit` é o ISO 7064 MOD 97-10, o esquema do IBAN. Na entrada a tela chama o
`trilha.HasCheckDigit(codigo)` e, quando ele é falso, responde com as mesmas palavras que um
código inexistente recebe — nunca com "código malformado", que seria uma segunda resposta e
portanto um jeito de separar números inventados dos reais. Letras valem `A=10 … Z=35`, e espaços
e hífens são ignorados, então um código impresso como `2026-0001-04` e digitado como
`20260001 04` são o mesmo código. O que isso custa à pessoa é nada — os dígitos são parte do que
ela recebeu. O que custa a quem está chutando é noventa e seis tentativas em cada noventa e sete,
recusadas sem consulta.

**Depois, dois orçamentos, porque os ataques são dois.** Um endereço varrendo o espaço de números
é barrado por um limite por IP; um protocolo real sendo martelado de uma botnet só é barrado por
um limite por código:

```go
var (
	porIP     = trilha.NewLimiter(trilha.RateLimit{RPS: 0.05, Burst: 5})
	porCodigo = trilha.NewLimiter(trilha.RateLimit{RPS: 0.05, Burst: 5})
)
```

**E uma negativa só.** Um código que ninguém emitiu e um fator que não bate dão o mesmo erro, o
mesmo status e os mesmos bytes — o teste da receita afirma que os dois corpos são idênticos. A
comparação do fator é `subtle.ConstantTimeCompare`, e uma busca que não achou nada ainda paga uma
comparação contra um valor de mentira, para a resposta de um código desconhecido não voltar
mensuravelmente antes da resposta de um fator errado. Toda tentativa é auditada com o código
mascarado até os três últimos caracteres: uma trilha que guarda protocolos inteiros é uma lista de
protocolos válidos.

O que sai é um `Registro` com seus `[]Evento`, e cada evento carrega `Visivel`. Só o que o
servidor marcou como público é renderizado — a linha do tempo que um analista vê e a que o
cidadão vê não são a mesma, e um filtro escrito na página é um filtro que alguém esquece na
segunda página. A tela desenha isso com `ui.Steps` (uma lista ordenada, com `aria-current` em
onde o caso está) e `ui.Date(c, ev.Em, ui.Relative())`, num formulário com rótulos de verdade,
`inputmode`, `autocomplete="off"` e `aria-describedby` — o público aqui é o cidadão, no celular.

@demo public-lookup

:::warning
**Não ponha essa rota atrás de login,** e não devolva de volta o que foi digitado. As duas coisas
são o mesmo erro em duas formas: a primeira pede a conta que a pessoa não tem, a segunda faz a
recusa de um código que existe ser uma página diferente da recusa de um que não existe.
:::
