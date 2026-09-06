---
title: Ctx
description: Tudo que uma função de rota pode fazer com o contexto da requisição.
---

`*trilha.Ctx` embrulha a requisição e a resposta. É criado por requisição e não deve ser
usado por outra goroutine depois que o handler devolve.

## Requisição

| Método | Descrição |
|---|---|
| `Request() *http.Request` | a requisição original |
| `SetContext(ctx)` | troca o contexto da requisição: um middleware passa valores a código que só recebe `*http.Request` |
| `SetRequest(*http.Request)` | troca a requisição (URL reescrita, corpo embrulhado) |
| `Context() context.Context` | contexto da requisição (cancelamento) |
| `Param(nome) string` | parâmetro de rota (`slug_` → `"slug"`) |
| `Query(nome) string` | primeiro valor do parâmetro de query |
| `Form(nome) string` | campo do formulário (faz o parse sob demanda, com limite de tamanho) |
| `FormErr() error` | erro do parse do formulário: 400 inválido, 413 grande demais |
| `BindJSON(&v) error` | decodifica o corpo JSON; campos desconhecidos são erro (400); 413 acima do limite |
| `Cookie(nome) (*http.Cookie, error)` | cookie da requisição |
| `RequestID() string` | `X-Request-ID` recebido ou um id gerado |
| `Env() trilha.Env` | `trilha.Dev` ou `trilha.Prod` |
| `Base() string` | prefixo de URL (`TRILHA_BASE_PATH`), sem barra final |
| `App() *trilha.App` | a aplicação |
| `Fragment() string` | id que o cliente quer trocar (cabeçalho `Trilha-Fragment`), ou `""` numa navegação normal ([Interatividade](/pt/aprender/interatividade)) |

## Resposta

| Método | Descrição |
|---|---|
| `JSON(code, v) error` | escreve JSON com `Content-Type` correto |
| `Text(code, s) error` | escreve texto simples |
| `HTML(code, node) error` | escreve um nó como documento inteiro, sem layouts |
| `Redirect(url) error` | devolve o erro de redirecionamento 303 (use com `return`) |
| `Status(code)` | status que a próxima renderização de página vai usar |
| `Header(k, v)` | define um cabeçalho de resposta |
| `SetCookie(*http.Cookie)` | adiciona `Set-Cookie` |
| `Render(code, node) error` | escreve a página **com os layouts da rota** (como o GET): para um `POST` devolver o formulário com erros (422); num fragmento, sem os layouts |
| `Stream() *Stream` | resposta em Server-Sent Events: `Send(evento, dados)`, `JSON(evento, v)`, `Comment(s)`, `Done()`; desliga o *write timeout* ([IA e agentes](/pt/aprender/ia-e-agentes)) |
| `Writer() http.ResponseWriter` | acesso direto (downloads longos, WebSocket) |
| `Written() bool` | se a resposta já começou |

## Entre página e layout

| Método | Descrição |
|---|---|
| `SetTitle(s)` / `Title() string` | título da página, lido pelos layouts |
| `Set(chave, v)` / `Get(chave) any` | valores por requisição (middleware → página → layout) |

## Ilhas

```go
func (c *Ctx) Island(src string, props any, children ...h.Node) h.Node
```

Renderiza `<div data-trilha-island="…" data-trilha-props="…">` com os filhos como conteúdo
de origem, vindo do servidor. `src` é um módulo em `public/` (endereçado pelo `Asset`, então
leva o hash do conteúdo) cuja **exportação padrão** é a função de montagem, chamada uma vez
com `(el, props)`. `props` é qualquer coisa que o `encoding/json` serialize, ou `nil`; viaja
como atributo escapado e volta pelo `JSON.parse`, então é dado, nunca marcação. Props que
não serializam avisam uma vez e deixam o conteúdo de origem em paz. O carregador é um único
script inline com o nonce da requisição, emitido junto da primeira ilha da resposta
([Interatividade](/pt/aprender/interatividade)).

## Conexão longa e corpo grande

| Método | Descrição |
|---|---|
| `AllowBody(n int64)` | limite de corpo **desta** requisição, no lugar do `Config.MaxBodyBytes` |
| `NoReadDeadline() error` | tira o prazo de leitura desta requisição (upload lento não é erro) |
| `NoWriteDeadline() error` | tira o prazo de escrita (download longo, SSE) |
| `Hijack() (net.Conn, *bufio.ReadWriter, error)` | assume a conexão: prazos removidos, e o Trilha não escreve mais nada nela |

O limite padrão é do app; a exceção é da rota. Levante no `middleware.go` da rota, não no
handler — o CSRF de formulário lê o corpo antes do handler rodar, então a decisão tem de vir
antes:

```go
// app/anexos/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20) // só esta requisição; o resto do app segue no limite do app
		c.NoReadDeadline()
	}
	return next()
}
```

Estourar o limite continua sendo 413 com a mensagem de sempre, pelo `FormErr`, pelos `Bind*`
ou na leitura direta do `Request().Body`.

### WebSocket

O Trilha não tem WebSocket próprio, e isso é decisão. O protocolo é transporte: não encosta
em rota, em layout nem em render. O que ele exige — frames de fragmentação e continuação,
frame de controle no meio de uma mensagem, aperto de mão de fechamento com prazo, validação
de UTF-8, máscara, limite de tamanho, escrita concorrente, contrapressão,
`permessage-deflate` — são algumas centenas de linhas que a suíte Autobahn cobra em mais de
500 casos. A assimetria decide: o seu app pode pôr `coder/websocket` no go.mod **dele** (o
princípio II obriga o framework, não o app), mas não consegue tirar essas linhas do
framework.

O que faltava era a porta, e ela é o `Hijack`:

```go
func WS(c *trilha.Ctx) error {
	conn, _, err := c.Hijack() // prazos de leitura e escrita já removidos
	if err != nil {
		return err
	}
	defer conn.Close()
	return meuWebsocket.Serve(conn) // coder/websocket, gorilla, o que você escolher
}
```

Depois do `Hijack` a conexão é sua: o framework não escreve cabeçalho, página de erro nem
corpo nela, e o log de acesso registra 101.

## Segurança

| Método | Descrição |
|---|---|
| `CSRFToken() string` | token da requisição; cria o cookie na primeira chamada |
| `trilha.CSRFInput(c) h.Node` | `<input type="hidden" name="_csrf">` para formulários |

O token é verificado automaticamente em `POST`, `PUT`, `PATCH` e `DELETE` de `page.go`
(e de `route.go` se `Config.CSRFForAPI` estiver ligado), pelo campo `_csrf` ou pelo
cabeçalho `X-CSRF-Token`.

## Bind

`Bind(v any) error` preenche uma struct a partir do formulário (ou do JSON, quando o
`Content-Type` é `application/json`). Campos casam pela tag `form:"nome"` (ou pelo nome do
campo); tipos: `string`, `[]string`, `bool` (`on`/`true`/`1`), `int`, `int64`, `float64`
(vírgula ou ponto), `time.Time` (`2006-01-02` ou `2006-01-02T15:04`) e ponteiros (nil quando
ausente). Struct aninhada é achatada, com a tag como prefixo (`Cobranca Endereco
`+"`form:\"cob_\"`"+` lê `cob_cep`…). Valores que não convertem viram `FieldErrors`
(mensagem `trilha.BindInvalid`, ajustável) depois de todos os campos serem tentados.
