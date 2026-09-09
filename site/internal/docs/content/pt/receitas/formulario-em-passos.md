---
title: Formulário em passos
description: Onde mora o passo 1 enquanto a pessoa está no passo 2, e o que acontece quando o rascunho vence.
---

Três telas, uma pergunta difícil — e não é o HTML: **onde mora o passo 1 enquanto a pessoa está
no passo 2?** O que se escreve no lugar é uma página cheia de `<input type="hidden">` (que o
primeiro upload quebra), ou uma linha meio preenchida no banco (que todo relatório depois precisa
aprender a ignorar), ou uma tela enorme com tudo.

O `c.Draft` é a resposta do framework: um rascunho com nome, guardado de uma requisição para a
outra, com prazo.

## Uma struct por passo

```go
// WizardStep1 is what a multi-step form actually needs: one struct per step,
// each with only its own rules. A single struct with every field cannot be
// validated halfway — step one would fail on an address nobody has typed —
// and what people do instead is drop the rules until the last screen, where a
// message about a field three screens back is useless.
type WizardStep1 struct {
	Name  string `form:"name"  validate:"required,max=80"`
	Email string `form:"email" validate:"required,email"`
}

// WizardStep2 is the second screen.
type WizardStep2 struct {
	Postcode string `form:"postcode" validate:"required"`
	City     string `form:"city"     validate:"required"`
}

// Wizard is what travels between the screens.
type Wizard struct {
	Who   WizardStep1 `json:"who"`
	Where WizardStep2 `json:"where"`
}
```

Esta é a parte que vale copiar. Uma struct só, com todos os campos e todas as tags `validate`,
não dá para conferir pela metade: o passo 1 falharia no endereço que ninguém digitou ainda, e a
saída que as pessoas acham é largar as regras até a última tela — onde uma mensagem sobre um
campo de três telas atrás não serve para nada.

## O fim de um passo

```go
// WizardPost is the end of a step: load what is there, bind only this step,
// save, move on. A 422 here costs nothing that was typed on an earlier screen,
// because the earlier screens are in the draft and not in this form.
func WizardPost(c *trilha.Ctx) error {
	var w Wizard
	_ = c.Draft("signup").Load(&w) // nothing yet on the first screen
	if err := c.Bind(&w.Who); err != nil {
		return err // FieldErrors: 422 with the messages next to the fields
	}
	if err := c.Draft("signup").Save(w, 30*time.Minute); err != nil {
		return err
	}
	return c.Redirect("/signup/where")
}
```

Carrega, faz o bind **só deste passo**, salva, segue. Um 422 aqui não custa nada do que foi
digitado antes, porque as telas anteriores estão no rascunho e não neste formulário.

## Toda tela depois da primeira

```go
// WizardResume is every screen after the first. No draft is not a failure: it
// is somebody whose draft expired, or who typed the address of step two
// directly, and the answer is step one — not an empty form that would lose
// whatever they filled in here.
func WizardResume(c *trilha.Ctx) (h.Node, error) {
	var w Wizard
	if err := c.Draft("signup").Load(&w); err != nil {
		return nil, c.Redirect("/signup/who")
	}
	return h.Div(ui.Steps(wizardSteps, 2), wizardForm(c, w)), nil
}
```

Não ter rascunho não é falha. É alguém cujo rascunho venceu, ou que digitou o endereço do passo 2
direto, ou que terminou este assistente ontem — e a resposta é o passo 1, não um formulário vazio
que perderia o que ela preenchesse aqui.

## O fim

```go
// WizardFinish is where the draft becomes a record and stops existing.
func WizardFinish(c *trilha.Ctx) error {
	var w Wizard
	if err := c.Draft("signup").Load(&w); err != nil {
		return c.Redirect("/signup/who")
	}
	if err := saveSignup(c, w); err != nil {
		return err
	}
	c.Draft("signup").Clear()
	return c.Redirect("/signup/done")
}
```

`Clear` antes do redirect, não depois: rascunho que virou registro precisa deixar de existir, ou
a próxima visita ao passo 2 retoma o que já aconteceu.

## Onde o rascunho mora

Abaixo de 2 KB de JSON ele é um **cookie assinado** — nada para configurar, nada para limpar, e
vence sozinho. Acima disso ele precisa do `Config.Drafts`, uma interface de três métodos sobre o
que a app já roda; sem ela, o `Save` devolve um erro citando esse campo em vez de mandar um
cookie que o navegador descartaria em silêncio.

:::warning
Rascunho é assinado, então não dá para editá-lo à mão. Ele **não é secreto**: o que está num
cookie viaja para o navegador e pode ser lido lá. Preço, desconto, o nome de outra pessoa — isso
mora atrás do `Config.Drafts`, com só a chave no cookie.
:::

O cookie é da própria pessoa, então um rascunho é invisível para os outros sem uma linha de código
sobre dono, e quem assina é o `TRILHA_SECRET`: sem segredo, o `Save` avisa.

## O indicador

O `ui.Steps(passos, atual)` desenha onde a pessoa está. Passo já feito é link; o atual leva
`aria-current="step"`; os da frente são texto — um assistente em que o passo 3 está a um clique é
um assistente cujos passos não precisavam ser em ordem.

O fluxo inteiro roda no
[`examples/cadastro`](https://github.com/emersonjoe/trilha/tree/main/examples/cadastro/app/assistente).
