---
title: Migração
description: De net/http puro para Trilha uma rota por vez, sem reescrita — e o que olhar quando você anda entre versões menores.
---

Ninguém reescreve um app que funciona. Este é o outro caminho: colocar o Trilha na frente,
mover uma rota, publicar, e repetir até não sobrar nada para mover.

## Vindo de `net/http`

Aqui está o app como ele era. Um mux com os endereços numa tabela, um handler que começa
descobrindo qual endereço ele é, um template executado à mão e o tratamento de erro escrito uma
vez por rota:

```go
// Routes is the table every net/http app grows: one mux, one line per
// address, and a handler that starts by finding out which address it is.
func Routes(find func(string) (Article, bool)) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blog/{slug}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := find(r.PathValue("slug"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, a); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /api/articles/{slug}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := find(r.PathValue("slug"))
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(a); err != nil {
			return
		}
	})
	return mux
}
```

E a cadeia que todo mundo escreve de novo — cabeçalhos, checagem de host, recover:

```go
// Secure is the middleware chain: the headers, the request id, the log and
// the recover that every app writes again, in the order that matters.
func Secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Host, "example.com") {
			http.Error(w, "bad host", http.StatusMisdirectedRequest)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

### A mesma coisa depois

O endereço é onde o arquivo mora, `app/blog/slug_/page.go`, então nada é declarado duas vezes:

```go
// Page is the same blog page after the move: the address is the folder it
// lives in (app/blog/slug_/page.go), the layout is applied for it, the 404
// is an error it returns, and the HTML is a value instead of a string.
func Page(c *trilha.Ctx) (h.Node, error) {
	a, err := ArticleBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return nil, err
	}
	c.SetTitle(a.Title)
	return h.Article(
		h.H1(h.Text(a.Title)),
		h.P(h.Time(h.Attr("datetime", a.Published.Format("2006-01-02")), h.Text(a.Published.Format("2 Jan 2006")))),
	), nil
}
```

```go
// GET is the same API route: no writer, no encoder, no Content-Type by
// hand. The error carries its own status, and an unexpected one becomes a
// problem+json body with the request id in it.
func GET(c *trilha.Ctx) error {
	a, err := ArticleBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return err
	}
	return c.JSON(200, a)
}
```

O que sumiu vale ser listado, porque é a troca inteira:

| Escrito à mão antes | Para onde foi |
|---|---|
| `mux.HandleFunc("GET /blog/{slug}", …)` | a pasta `app/blog/slug_/` |
| `http.NotFound` por rota | `return trilha.ErrNotFound`, negociado como HTML ou `problem+json` |
| `w.Header().Set("Content-Type", …)` | `c.JSON`, `c.HTML`, `c.Text` |
| o template, executado e conferido | o `h`, que é Go e escapa por construção |
| os cabeçalhos de segurança e o `recover` | o runtime, ligados por padrão |
| o layout repetido em cada template | o `layout.go` da pasta |

### Uma rota por vez

Você não precisa de uma virada de chave. O app do Trilha é um `http.Handler`, e o seu mux
também, então qualquer um dos dois pode estar na frente do outro:

```go
// Front is how the two systems share a process while the move happens: the
// old mux answers what has not been moved yet, and everything it does not
// know falls through to the framework. The old middleware still wraps both,
// so nothing loses its headers halfway.
func Front(mux *http.ServeMux, a *trilha.App) http.Handler {
	mux.Handle("/", a.Handler())
	return before.Secure(mux)
}
```

Mova as folhas primeiro — uma página sem dependências, uma rota de API que só lê. Publique
depois de cada uma. Os dois sistemas dividem o mesmo processo, o mesmo pool e o mesmo logger;
uma rota está num ou no outro, nunca metade nos dois.

### Quando o app mora dentro do binário antigo

O `Front` acima supõe que os dois vivem no mesmo `package main`. Muitas vezes não vivem: o
que está sendo movido é uma área de um servidor maior e quer a pasta dele — `internal/crm/`,
com o `app/` dele. Declare o pacote à mão lá dentro e o `trilha gen` acompanha, escrevendo o
`NewApp` no mesmo pacote em vez de um `main` que ninguém pediu:

```go
// Package crm is one area of a server that already exists: it has its own
// app/ folder and its own package name, written by hand in this file.
// `trilha gen` follows the package it finds here and writes NewApp into the
// same one, so the binary that hosts it mounts the app with no registration
// file of its own.
package crm
```

O binário que já existe monta o app como monta qualquer outro handler:

```go
// Host is the same move when the app does not live in package main: crm is a
// folder of the binary that already exists, `trilha gen` wrote NewApp into
// the package that folder declares, and mounting it is one line. There is no
// registration file to keep by hand.
func Host(mux *http.ServeMux) http.Handler {
	mux.Handle("/", crm.NewApp().Handler())
	return before.Secure(mux)
}
```

Não existe arquivo de registro escrito à mão, e essa é a razão: o `trilha gen --check` do CI
continua pegando a pasta que alguém criou sem gerar. O `trilha dev` e o `trilha build` não
valem dentro de `internal/crm` — o binário é o hospedeiro — e eles dizem isso. Veja
[CLI](/pt/referencia/cli#um-app-dentro-de-um-binario-que-ja-existe).

Duas coisas precisam de decisão antes:

- **Sessões.** Se o app antigo tem cookie próprio, continue lendo ele num middleware enquanto o
  novo escreve com `SetSigned`, e tire o leitor antigo quando tudo tiver migrado.
- **Arquivos estáticos.** O `public/` é servido pelo framework com URLs com hash, via
  `c.Asset`. Um caminho escrito à mão no HTML antigo continua funcionando; ele só não ganha o
  cache longo.

:::dica
Comece com `trilha new` num diretório vazio e copie os seus handlers para lá, em vez de
acrescentar o framework à árvore que já existe. Comparar dois diretórios é mais fácil do que
desembaraçar um.
:::

## Entre versões menores

A regra que o projeto segue: antes do 1.0, uma versão menor pode mudar como um app novo se
parece, mas a atualização está sempre escrita. Na prática, quatro passos:

```bash
go get -u github.com/emersonjoe/trilha@latest
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha gen        # o arquivo gerado tem que bater com a versão da CLI
trilha audit      # entre outras coisas, ele compara CLI e biblioteca
make test
```

O `trilha audit` é o que pega a divergência que ninguém percebe: um `trilha_gen.go` escrito por
uma CLI mais velha serve as rotas de um `app/` mais velho. É um aviso, não um erro fatal, que é
precisamente por que vale rodar.

O [changelog](https://github.com/emersonjoe/trilha/blob/main/CHANGELOG.md) é a fonte do que
mudou; as seções `## O que muda para você` de uma release são escritas para este momento. O que
vem depois de trocar a versão é o de sempre: leia a seção, rode os testes, e se a release
acrescentou uma convenção (um nome de pasta novo, um arquivo novo que passa a ser lido), o
`trilha routes` imprime o que o scanner enxerga agora, que é o jeito mais rápido de conferir se
ele viu o que você quis dizer.

:::nota
Um símbolo público nunca desaparece numa versão menor sem antes ser marcado como obsoleto em
uma. A superfície versionada mora em `api/current.txt`, e uma mudança nela que não foi
intencional quebra a suíte de testes do próprio framework.
:::
