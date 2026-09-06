# Spec 032 — Auxiliares de teste

- **Issue**: [#32](https://github.com/emersonjoe/trilha/issues/32) (ROADMAP, Fase 3, item 14)
- **Branch**: `032-auxiliares-de-teste`
- **Versão**: 0.23.0

## Por quê

Os cinco exemplos deste repositório têm, cada um, a mesma cinquentena de linhas antes do
primeiro teste: um `client` com `httptest`, um pote de cookies escrito à mão, uma função que
copia o token CSRF do cookie para o campo `_csrf` do formulário, um `wantContains`. É o
mesmo código cinco vezes, e é o código que todo projeto feito com o framework vai escrever
de novo — pior, vai escrever errado nas primeiras tentativas, porque a parte do CSRF só
funciona se você souber que o cookie tem que voltar no corpo.

O framework já sabe fazer isso: ele é quem emite o cookie e quem confere o token. O que
falta é oferecer, no mesmo pacote, o cliente de teste que qualquer app vai querer — sem
framework de teste externo, sem `Assert...` de biblioteca, sem inversão de controle.

A régua da ergonomia é a própria issue: os testes dos exemplos passam a usar os auxiliares.
Se depois disso as suítes não encolherem, a API está errada.

## O que muda

Um arquivo novo, `testing.go`, em `package trilha`.

### Um tiro só

```go
res := trilha.TestRequest(t, app, "GET", "/api/posts")
res.WantStatus(200).WantContains(`"slug"`)
```

`app` é o `*trilha.App` que o projeto já monta (o `newApp()` que o gerador escreve). A
requisição passa pelo caminho de verdade: mux, middlewares, layouts, CSRF, negociação de
erro. O que volta é a resposta gravada, com asserções curtas encadeáveis.

Para testar um `route.go` sem registrar nada:

```go
res := trilha.TestRoute(t, trilha.Route{Pattern: "/api/itens/{id}", Methods: map[string]trilha.HandlerFunc{"GET": GET}},
	"GET", "/api/itens/7")
```

E uma página, com os layouts aplicados, devolvendo também o nó renderizado:

```go
res := trilha.TestPage(t, trilha.Route{Pattern: "/blog/{slug}", Page: Page, Layouts: []trilha.LayoutFunc{Layout}},
	"/blog/ola")
res.WantStatus(200)
if res.Node == nil { … }
```

`TestRoute` e `TestPage` montam um `App` descartável em `Dev`; `WithApp(a)` usa o seu.

### Uma sessão inteira

Quando o teste é um fluxo (abrir formulário, enviar, seguir o redirecionamento), o cliente
guarda os cookies que o app põe:

```go
c := trilha.NewTestClient(t, app)
c.Get("/blog/novo").WantStatus(200)
c.PostForm("/blog/novo", url.Values{"titulo": {"Olá"}}).WantStatus(303)
```

### CSRF: por padrão o teste passa

Todo pedido enviado pelos auxiliares leva o cookie CSRF, e todo método com corpo leva o
mesmo valor no cabeçalho `X-CSRF-Token`. Não é uma brecha: o cookie e o token vêm do mesmo
cliente, que é exatamente o que a proteção de duplo envio exige de um navegador. Um teste
que queira provar a recusa pede a recusa:

```go
c.PostForm("/blog/novo", form, trilha.WithoutCSRF()).WantStatus(403)
```

### Sessão assinada sem passar pelo login

`WithSigned(nome, valor)` grava um cookie assinado com o `Signer` do app — o mesmo que o
`c.SetSigned` do handler usa. Testar a página do administrador deixa de exigir um `POST
/login` antes de cada caso:

```go
trilha.TestRequest(t, app, "GET", "/admin", trilha.WithSigned("sessao", "ana"))
```

### Superfície

| Símbolo | Papel |
|---|---|
| `TestingT` | `Helper()` e `Fatalf(...)`: o que os auxiliares usam de `*testing.T`, para o pacote não importar `testing` |
| `TestRequest(t, a *App, method, target string, opts ...TestOption) *TestResponse` | um pedido no app inteiro |
| `TestRoute(t, r Route, method, target string, opts ...TestOption) *TestResponse` | um `route.go`, com seus middlewares |
| `TestPage(t, r Route, target string, opts ...TestOption) *TestResponse` | uma página, com seus layouts; `Node` vem preenchido |
| `NewTestClient(t, a *App) *TestClient` | o cliente com pote de cookies |
| `(*TestClient) Request / Get / PostForm / PostJSON` | os pedidos |
| `TestOption` | `WithApp`, `WithHeader`, `WithCookie`, `WithSigned`, `WithForm`, `WithJSON`, `WithBody`, `WithoutCSRF` |
| `TestResponse` | `Node`, `WantStatus`, `WantContains`, `WantHeader`, `JSON(&v)`, `Cookie(nome)`; embute `*httptest.ResponseRecorder` |

`TestResponse` embute o `*httptest.ResponseRecorder`, então `Code`, `Body` e `Header()`
continuam à mão para o que as asserções prontas não cobrirem. Nenhuma asserção devolve
`error`: em teste, o valor de um erro é parar o teste com a mensagem certa.

## Fora de escopo

- **Framework de asserção.** `WantStatus`, `WantContains`, `WantHeader` e `JSON` cobrem o que
  os exemplos usam hoje; o resto é `if` com `t.Fatalf`, que é o que o Go pede.
- **Servidor de verdade (`httptest.NewServer`).** O handler responde em memória; quem
  precisar de rede monta o servidor com duas linhas do `httptest`.
- **Navegador sem estado de verdade** (seguir redirecionamento sozinho). O teste que quer o
  destino pede o destino: um redirecionamento é uma asserção, não um detalhe.
- **Auxiliar de banco/fixtures.** Cada exemplo já tem o seu `Seed()`; isso é do app.
- **Teste no scaffold do `trilha new`.** Vale, mas é uma convenção nova de arquivo — issue
  própria.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/http/httptest`, `net/url`, `encoding/json`; `testing` não é importado (por isso `TestingT`) |
| IV — contrato de handler pequeno | os auxiliares recebem `Route` e `*App`, que já existem; nenhuma assinatura nova de handler |
| VI — teste primeiro, exemplo como integração | `testing_test.go` antes do código, e os cinco exemplos passam a usar os auxiliares |
| VII — segurança por padrão | o CSRF continua sendo conferido; o auxiliar age como navegador, e `WithoutCSRF` existe para provar a recusa |
| IX — API pública pequena | um arquivo, um canto (`Test*`, `With*`), nada muda no runtime |

## Aceitação

- **SC-001** `examples/blog`, `examples/orcamento`, `examples/cadastro`, `examples/sso` e
  `examples/assistente` não têm mais `client`, pote de cookies nem `wantContains` próprios: a
  soma das cinco suítes encolhe.
- **SC-002** `TestRequest` num `POST` de página passa no CSRF sem o teste tocar em token, e
  falha com 403 sob `WithoutCSRF()`.
- **SC-003** `WithSigned` dá acesso a uma rota protegida sem `POST /login` antes.
- **SC-004** `TestPage` devolve `Node` não nulo com os layouts aplicados, e `TestRoute`
  resolve `{id}` a partir do alvo.
- **SC-005** `go test ./...` verde e `TestNoExternalDeps` continua verde.
