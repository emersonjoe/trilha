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

O exemplo acima devolve o erro pela query string, o que mantém o padrão POST → redirect →
GET e funciona sem JavaScript. Para formulários maiores, guarde os valores digitados em um
cookie curto ou renderize a página de novo com `return c.HTML(422, Page...)`, sem redirecionar.

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

Adicione ao formulário um campo `vagas` numérico e rejeite valores negativos com uma
mensagem, sem perder o padrão de redirecionamento.

:::solucao
```go
vagas, err := strconv.Atoi(c.Form("vagas"))
if err != nil || vagas < 0 {
	return c.Redirect("/eventos/novo?erro=Vagas+precisa+ser+um+n%C3%BAmero+positivo")
}
```
:::
