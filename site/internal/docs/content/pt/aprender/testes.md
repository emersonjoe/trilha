---
title: Testes
description: Um cliente de teste no próprio framework: um pedido, uma sessão inteira, CSRF que já passa.
---

Um app feito com Trilha é um `http.Handler`, então sempre deu para testá-lo com `httptest` e
mais nada. O problema é o que vem antes da primeira asserção: um cliente, um pote de cookies
e o token do CSRF copiado do cookie para o campo do formulário. São cinquenta linhas que todo
projeto escreve de novo — e erra na primeira tentativa, porque o duplo envio só passa quando
o cookie volta no pedido.

O framework já emite esse cookie e já confere esse token, então ele traz o cliente junto. Sem
framework de teste externo, sem biblioteca de asserção: o `package trilha` nunca importa
`testing`.

## Um pedido

```go
func TestListaPosts(t *testing.T) {
	res := trilha.TestRequest(t, newApp(), "GET", "/api/posts")
	res.WantStatus(200).WantContains(`"slug"`)
}
```

`newApp()` é a função que o gerador escreve no `trilha_gen.go`: o mesmo app que serve em
produção. O pedido passa pelo caminho de verdade — mux, middlewares, layouts, CSRF,
negociação de erro — e o que volta é a resposta gravada.

As asserções encadeiam e nenhuma devolve `error`. Em teste, o valor de um erro é parar com a
mensagem certa, então a falha imprime o status, o alvo e o corpo:

```text
GET /api/posts: status = 500, want 200
{"status":500,"title":"Internal Server Error","request_id":"…"}
```

## Uma sessão inteira

Quando o teste é um fluxo — abrir o formulário, enviar, seguir o redirecionamento — o cliente
guarda os cookies que o app põe:

```go
func TestPublicar(t *testing.T) {
	c := trilha.NewTestClient(t, newApp())
	c.Get("/blog/novo").WantStatus(200)
	res := c.PostForm("/blog/novo", url.Values{"titulo": {"Olá"}})
	res.WantStatus(303).WantHeader("Location", "/blog/ola")
	c.Get("/blog/ola").WantContains("Olá")
}
```

`Get`, `PostForm` e `PostJSON` são atalhos do `Request`, que aceita qualquer método. O
redirecionamento não é seguido sozinho: o teste que quer o destino pede o destino, porque
onde um `303` para é uma asserção, não um detalhe.

## O CSRF passa por padrão

Todo pedido enviado pelos auxiliares leva o cookie do CSRF, e todo método com corpo leva o
mesmo valor no cabeçalho `X-CSRF-Token`.

:::note
Isso não é uma brecha na proteção. O duplo envio pede ao navegador que prove que consegue ler
o próprio cookie, e o cliente de teste prova exatamente isso: cookie e token vêm do mesmo
lugar. O que a conferência recusa — um formulário enviado de outro site, que não consegue ler
o cookie — continua recusado.
:::

Um teste que queira provar a recusa pede a recusa:

```go
c.PostForm("/blog/novo", form, trilha.WithoutCSRF()).WantStatus(403)
```

## Sessão assinada sem passar pelo login

`WithSigned` grava um cookie assinado com o signer do próprio app — o mesmo que o
`c.SetSigned` do handler usa. A página do administrador deixa de exigir um `POST /login`
antes de cada caso:

```go
res := trilha.TestRequest(t, newApp(), "GET", "/admin", trilha.WithSigned("sessao", "ana"))
res.WantStatus(200)
```

A assinatura é de verdade: uma sessão forjada à mão continua falhando, que é para isso que
serve o `trilha.WithCookie("sessao", "ana|9999999999|assinatura-falsa")` quando o que você
quer testar é a recusa.

## Um `route.go`, uma página

O `TestRoute` monta um app descartável em `Dev` em volta de uma rota só, então dá para testar
o handler onde ele mora, antes de estar registrado em qualquer lugar:

```go
res := trilha.TestRoute(t, trilha.Route{
	Pattern: "/api/itens/{id}",
	Methods: map[string]trilha.HandlerFunc{"GET": GET},
}, "GET", "/api/itens/7")
res.WantStatus(200).WantContains(`"id":7`)
```

É o padrão que resolve o `{id}`, então o `c.Param("id")` responde `7` — quem faz o trabalho é
o roteador, não um dublê.

O `TestPage` faz o mesmo por uma página e ainda devolve o nó renderizado, com os layouts já
aplicados:

```go
res := trilha.TestPage(t, trilha.Route{Page: Page, Layouts: []trilha.LayoutFunc{Layout}}, "/sobre")
res.WantStatus(200)
if h.Render(res.Node) == "" {
	t.Fatal("página vazia")
}
```

O `res.Body` tem o documento inteiro, com o layout em volta; o `res.Node` é só o que a página
devolveu. Asserção no nó sobrevive a uma troca de layout, que costuma ser o que você quer.

Os dois montam o app para você; `trilha.WithApp(a)` usa o seu, quando a rota depende de algo
que o `Setup` põe no `a.Values()`.

## As opções

| Opção | O que faz |
|---|---|
| `WithApp(a)` | usa o seu app no `TestRoute`/`TestPage` em vez de um descartável |
| `WithHeader(nome, valor)` | um cabeçalho (`Accept`, `Trilha-Fragment`, `Authorization`) |
| `WithCookie(nome, valor)` | um cookie cru |
| `WithSigned(nome, valor)` | um cookie assinado pelo app, válido por uma hora |
| `WithForm(values)` | corpo em `application/x-www-form-urlencoded` |
| `WithJSON(v)` | corpo em `application/json` |
| `WithBody(contentType, corpo)` | corpo exatamente como escrito (multipart, CSV, um JSON quebrado) |
| `WithoutCSRF()` | não manda nada de CSRF, para testar a recusa |

## A resposta

O `TestResponse` embute o `*httptest.ResponseRecorder`, então `Code`, `Body` e `Header()`
continuam à mão para o que as asserções prontas não cobrirem.

| Método | O que faz |
|---|---|
| `WantStatus(código)` | falha com o corpo quando o status é outro |
| `WantContains(texto)` | falha com o corpo quando o texto não está lá |
| `WantHeader(nome, valor)` | falha quando o cabeçalho é outro |
| `JSON(&v)` | decodifica o corpo em `v`, falhando com o corpo se o JSON for inválido |
| `Cookie(nome)` | o cookie que esta resposta pôs, ou `nil` |
| `Node` | o nó da página, preenchido pelo `TestPage` |

O `Cookie` é como se faz a asserção de uma saída: o que prova que a sessão acabou é o app
apagar o cookie, não o redirecionamento que vem depois.

```go
if res.Cookie("sessao") == nil {
	t.Fatal("sair devia limpar a sessão")
}
```

## Desafio

Escreva um teste provando que o formulário do blog recusa um título maior que o limite e
mostra a mensagem na página, sem passar pela API.

:::solucao
```go
func TestTituloLongo(t *testing.T) {
	c := trilha.NewTestClient(t, newApp())
	res := c.PostForm("/blog/novo", url.Values{"titulo": {strings.Repeat("a", 200)}})
	res.WantStatus(422).WantContains("no máximo")
}
```
O formulário responde `422` com a página redesenhada — o mesmo corpo que o navegador
mostraria —, então um pedido cobre a validação e a mensagem. O token do CSRF foi junto
sozinho.
:::
