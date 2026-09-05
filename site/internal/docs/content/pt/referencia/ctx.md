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
| `Render(code, node) error` | escreve a página **com os layouts da rota** (como o GET): para um `POST` devolver o formulário com erros (422) |
| `Stream() *Stream` | resposta em Server-Sent Events: `Send(evento, dados)`, `JSON(evento, v)`, `Comment(s)`, `Done()`; desliga o *write timeout* ([IA e agentes](/pt/aprender/ia-e-agentes)) |
| `Writer() http.ResponseWriter` | acesso direto (downloads longos, WebSocket) |
| `Written() bool` | se a resposta já começou |

## Entre página e layout

| Método | Descrição |
|---|---|
| `SetTitle(s)` / `Title() string` | título da página, lido pelos layouts |
| `Set(chave, v)` / `Get(chave) any` | valores por requisição (middleware → página → layout) |

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
