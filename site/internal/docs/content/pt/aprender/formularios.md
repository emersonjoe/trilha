---
title: Formulários
description: POST no mesmo page.go, proteção CSRF automática e o padrão redirecionar-depois-de-gravar.
---

Uma página pode receber formulários exportando `POST` (ou `PUT`, `PATCH`, `DELETE`) ao lado
de `Page`. O Trilha verifica o token CSRF antes de chamar a sua função.

## A página com o formulário

`app/eventos/novo/page.go`:

```go
package novo

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"agenda/internal/eventos"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Novo evento")
	erro := c.Query("erro")
	return h.Fragment(
		h.H1(h.Text("Novo evento")),
		h.If(erro != "", h.P(h.Class("erro"), h.Text(erro))),
		h.Form(h.Method("post"), h.Action("/eventos/novo"),
			trilha.CSRFInput(c),
			h.Label(h.For("nome"), h.Text("Nome")),
			h.Input(h.ID("nome"), h.Name("nome"), h.Required(), h.Autofocus()),
			h.Label(h.For("cidade"), h.Text("Cidade")),
			h.Input(h.ID("cidade"), h.Name("cidade")),
			h.Button(h.Type("submit"), h.Text("Publicar")),
		),
	), nil
}

func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err // 400 em formulário inválido, 413 se passou do limite
	}
	nome := strings.TrimSpace(c.Form("nome"))
	if nome == "" {
		return c.Redirect("/eventos/novo?erro=Informe+o+nome")
	}
	ev := eventos.Criar(nome, c.Form("cidade"))
	return c.Redirect("/eventos/" + ev.Slug)
}
```

@demo form

## O que acontece no envio

1. O navegador manda `POST /eventos/novo` com os campos e o `_csrf`.
2. O Trilha compara o `_csrf` com o cookie `trilha_csrf` (tempo constante). Diferente ou
   ausente: **403**, e `POST` nem roda.
3. `POST` roda e devolve `c.Redirect(...)`: resposta **303 See Other**. O navegador faz um
   `GET` na URL nova. Recarregar a página não reenvia o formulário.

`trilha.CSRFInput(c)` cria o cookie na primeira renderização e o campo oculto. Clientes
JavaScript podem mandar o mesmo valor no cabeçalho `X-CSRF-Token`.

## Validação e mensagens

O exemplo acima confere o nome na mão e devolve o erro pela query string, o que mantém o
padrão POST → redirect → GET e funciona sem JavaScript. Assim que o formulário passa de dois
ou três campos, ponha as regras na struct: a tag `validate` fica do lado do campo de que ela
fala, e o `Bind` aplica todas antes de voltar.

```go
type entrada struct {
	Nome  string `form:"nome" validate:"required,min=3,max=80"`
	Email string `form:"email" validate:"required,email"`
	Vagas int    `form:"vagas" validate:"min=1,max=10"`
}

func POST(c *trilha.Ctx) error {
	var in entrada
	if err := c.Bind(&in); err != nil {
		if errs, ok := err.(trilha.FieldErrors); ok {
			// Mesma página, 422, valores preservados, uma mensagem por campo.
			return c.Render(http.StatusUnprocessableEntity, formulario(c, in, errs))
		}
		return err
	}
	ev := eventos.Criar(in.Nome, in.Email, in.Vagas)
	return c.Redirect("/eventos/" + ev.Slug)
}
```

`FieldErrors` é um `map[string]string` (campo → mensagem), então o formulário lê direto:
`ui.Errors(errs, "email")` mostra a mensagem e `ui.InvalidIf(errs, "email")` marca o campo
com `aria-invalid`. Nada para no primeiro erro — a pessoa vê tudo de uma vez, não um erro a
cada envio.

As regras são `required`, `min`, `max`, `len`, `email`, `url`, `oneof` e `eqfield`; a
[referência de validação](/pt/referencia/validacao) tem o que cada uma quer dizer em cada
tipo. Duas merecem ser ditas aqui:

- **Toda regra além de `required` ignora valor vazio.** Um campo opcional com `min=3` só
  responde pelo que alguém digitou.
- **`required` quer dizer "não é o valor zero".** Onde `0` ou `false` é resposta de verdade,
  declare o campo como ponteiro (`*int`): ausente continua ausente, e zero chega como zero.

As mensagens vêm em inglês. App que fala outra língua chama `trilha.UseValidationPTBR()` no
`Setup`, ou escreve as próprias em `trilha.ValidationMessages`.

### Quando a tag não basta

Regra sobre o formato de um valor pertence ao tipo — e aí todo formulário que usa o tipo
está coberto:

```go
type Dinheiro string

func (d Dinheiro) Validate() error {
	if v, err := ParseMoney(string(d)); err != nil || v <= 0 {
		return errors.New("valor deve ser maior que zero")
	}
	return nil
}
```

Regra que lê dois campos pertence à struct: dê a ela um `Validate() error` e ela roda no
fim, só quando nenhum campo falhou. Regra que se repete de projeto em projeto vira uma tag
sua:

```go
trilha.AddRule("cep", func(f trilha.Field) bool { return cepValido(f.Text) })
trilha.ValidationMessages["cep"] = "CEP inválido"
```

A fronteira é esta: a tag diz o que um **valor** aceita, não o que o **sistema** aceita.
"Essa conta existe" e "essa sala está livre nessa noite" são perguntas para os seus dados, e
ficam no seu pacote. Rode depois do `Bind` e junte o resultado no mesmo `FieldErrors`, para
os dois tipos de mensagem chegarem na mesma resposta.

## Métodos que o navegador não manda

Formulários HTML só enviam GET e POST. Para "apagar", exporte `DELETE` para clientes de API
e faça o `POST` da página chamar a mesma lógica:

```go
func DELETE(c *trilha.Ctx) error {
	if !eventos.Apagar(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	return c.Redirect("/eventos")
}

func POST(c *trilha.Ctx) error { return DELETE(c) }
```

## Limites

O corpo da requisição tem limite de 1 MiB por padrão (`Config.MaxBodyBytes`). Acima disso a
resposta é 413 antes de o seu código rodar.

## Desafio

Adicione ao formulário um campo `vagas` numérico, aceite só de 1 a 10, e mostre a mensagem
ao lado do campo em vez de na página seguinte.

:::solucao
```go
type entrada struct {
	Nome   string `form:"nome" validate:"required,min=3"`
	Cidade string `form:"cidade"`
	Vagas  int    `form:"vagas" validate:"required,min=1,max=10"`
}

// No POST, c.Bind(&in) devolve trilha.FieldErrors, e a página é renderizada de
// novo com c.Render(http.StatusUnprocessableEntity, ...) e ui.Errors(errs, "vagas").
```
:::
