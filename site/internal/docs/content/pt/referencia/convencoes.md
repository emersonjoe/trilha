---
title: Convenções de arquivo
description: Tabela completa do que cada arquivo e nome de pasta em app/ significa.
---

## Arquivos

| Arquivo | Função exportada | Assinatura | Alcance |
|---|---|---|---|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` | rota GET da pasta |
| `page.go` | `POST`, `PUT`, `PATCH`, `DELETE` (opcionais) | `func(c *trilha.Ctx) error` | formulários; CSRF exigido |
| `route.go` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS` (ao menos um) | `func(c *trilha.Ctx) error` | API JSON da pasta |
| `kind.go`, ou qualquer arquivo (opcional) | `Kind` | `var Kind = trilha.KindPage` ou `KindAPI` | subárvore: como erros são renderizados e se há CSRF (veja [Erros](/pt/referencia/erros)) |
| `route.go` (opcional) | `CORS` | `var CORS = trilha.CORS{...}` | política de origem cruzada só desta rota, preflight incluído |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` | subárvore |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` | subárvore |
| `middleware.go` (opcional) | `MiddlewareGET`, `MiddlewarePOST`, `MiddlewarePUT`, `MiddlewarePATCH`, `MiddlewareDELETE`, `MiddlewareOPTIONS` | `func(c *trilha.Ctx, next trilha.Next) error` | subárvore, só naquele método |
| `not_found.go` (só na raiz) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` | 404 do app |
| `error.go` (só na raiz) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` | todo status de erro menos o 404 |
| `setup.go` (só na raiz) | `Setup` | `func(a *trilha.App) error` | antes de servir |
| `setup.go` (opcional) | `Config` | `func(cfg *trilha.Config)` ou `func(cfg *trilha.Config) error` | antes de `trilha.New`; o erro interrompe a subida |
| `setup.go` (opcional) | `Shutdown` | `func(a *trilha.App) error` | depois de parar de aceitar requisições (fechar pool, fila, flush de log) |

`page.go` e `route.go` na mesma pasta é erro. A função pode estar em qualquer arquivo do
pacote; o nome do arquivo é o que liga a convenção.

### O Kind segue a subárvore

`Kind` é variável, não função, e é herdado como o `Layout` e o `Middleware`: declarado no
pacote de uma pasta, vale para ela e para tudo abaixo, e a declaração mais funda ganha.
`kind.go` é o nome do arquivo para a pasta que não tem `route.go` próprio — a raiz de uma
subárvore precisa poder falar sem ter rota:

```go
// app/painel/kind.go — este ramo é de páginas do navegador, então as escritas cobram CSRF
package painel

var Kind = trilha.KindPage
```

Isso pesa mais do que a renderização do erro: **`Kind` é o que liga o CSRF**. Um `route.go`
nasce API, e API não confere o token, então a mesma ação de formulário movida de um `page.go`
para um `route.go` passa a aceitar POST de outro site — em silêncio. Uma linha na raiz do ramo
cobre todas as folhas, inclusive a que alguém criar no mês que vem. O `trilha audit` aponta a
rota de escrita que nenhum `Kind` alcança num app que também serve páginas.

Uma rota de `page.go` é página independente do que o ramo acima diz: um `KindAPI` herdado
nunca transforma página em JSON.

## Pastas

| Nome | Vira | Exemplo |
|---|---|---|
| `eventos` | segmento literal | `/eventos` |
| `slug_` | parâmetro `{slug}` | `/eventos/{slug}` → `c.Param("slug")` |
| `caminho__` | catch-all `{caminho...}`; precisa ser folha | `/docs/{caminho...}` |
| `organizador-` | grupo de rota; não entra na URL | layout/middleware para a subárvore |
| `app.css`, `robots.txt` | caminho fixo com extensão (ponto no meio do nome) | `/app.css`, `/manifest.webmanifest`, `/sw.js` |
| `.well-known` | a única pasta com ponto no começo que é rota | `/.well-known/security.txt` |
| `_x`, `.x`, `testdata` | ignoradas | — |

Uma pasta com ponto no nome serve um caminho fixo com extensão. Como `app.css` não é um
identificador Go, declare outro nome de pacote no arquivo (`package appcss`); o gerador
importa tudo com alias, então o nome do pacote não importa.

Pastas que **começam** com ponto continuam ignoradas, com uma exceção: `.well-known`, onde
a RFC 8414, a RFC 9728, a RFC 8555, a RFC 9116 e o OpenID Discovery mandam publicar
documento. Dentro dela valem as convenções de sempre —
`app/.well-known/security.txt/route.go` responde `/.well-known/security.txt`. Um `page.go`
ou `route.go` dentro de **qualquer outra** pasta com ponto agora é erro `E_HIDDEN_ROUTE`, em
vez de um 404 que ninguém explica; para tirar uma pasta do roteamento de propósito, comece o
nome com `_`.

A ferramenta Go não casa caminho com ponto no padrão `./...`, então `go vet ./...` e `go
test ./...` não pegam esse pacote como alvo. Ele compila do mesmo jeito: o `trilha_gen.go` o
importa pelo caminho explícito.

## Origem cruzada numa rota só

`Config.CORS` é a política do app inteiro. Quando só alguns caminhos são públicos — os
documentos de descoberta em `/.well-known/`, buscados de outra origem por um cliente que
ainda não tem sessão —, a política mora na rota:

```go
package oauthresource

// Só esta rota. As outras seguem de mesma origem.
var CORS = trilha.CORS{Origins: []string{"*"}, Methods: []string{"GET"}}

func GET(c *trilha.Ctx) error { ... }
```

O framework responde o preflight a partir dela (204 com `Access-Control-Allow-*`, ou 403 se
a origem ou o método estiverem fora da lista) e põe os cabeçalhos em toda resposta daquela
rota. A rota que declara a própria política decide sozinha: a lista do app não a alarga nem
a estreita. Escrever `func OPTIONS` no mesmo arquivo retoma o preflight — o caso comum é
declarativo, o esquisito continua seu.

`HEAD` não é nome de handler: desde o Go 1.22 o roteador responde HEAD com o handler do
`GET`.

Precedência: literal vence parâmetro, que vence catch-all. Duas pastas dinâmicas irmãs são
erro. Duas pastas que gerem a mesma URL (via grupos) são erro.

## Outras pastas do projeto

| Pasta | Papel |
|---|---|
| `public/` | arquivos estáticos servidos na raiz; embutidos no binário em produção |
| `trilha_gen.go` | gerado; commitado; nunca editado à mão; leva o pacote que a pasta declara (veja [CLI](/pt/referencia/cli)) |
| `.trilha/` | binários temporários do `dev` e do `export`; ignorada pelo git |

## Ordem de execução de `GET /a/b`

```text
middleware(app) → middleware(app/a) → middleware(app/a/b)
  → middlewareGET(app) → middlewareGET(app/a) → middlewareGET(app/a/b)
  → Page (ou método)
  → layout(app/a/b) → layout(app/a) → layout(app)
```

A cadeia do método roda por dentro da cadeia da rota: uma regra de um método só refina o que
a rota já decidiu. Para `POST` é `MiddlewarePOST`, e assim por diante; um método sem cadeia
própria roda só a da rota.

## Erros de geração

| Código | Causa |
|---|---|
| `E_PAGE_AND_ROUTE` | `page.go` e `route.go` na mesma pasta |
| `E_NO_PAGE_FUNC` | `page.go` sem `Page` |
| `E_NO_METHOD` | `route.go` sem método exportado |
| `E_NO_LAYOUT_FUNC`, `E_NO_MIDDLEWARE_FUNC`, `E_NO_NOT_FOUND_FUNC`, `E_NO_ERROR_FUNC`, `E_NO_SETUP_FUNC` | arquivo sem a função esperada |
| `E_AMBIGUOUS_SEGMENT` | duas pastas dinâmicas no mesmo nível |
| `E_CATCHALL_NOT_LEAF` | rotas abaixo de uma pasta `x__` |
| `E_BAD_SEGMENT` | nome de parâmetro inválido ou grupo dinâmico (`x_-`) |
| `E_DUPLICATE_ROUTE` | duas pastas produzindo a mesma URL |
| `E_UNUSED_METHOD_MIDDLEWARE` | `MiddlewareX` que não alcança nenhuma rota que sirva `X` |
| `E_PARSE` | arquivo Go que não compila |
| `E_NO_APP` | não há pasta `app/` |
| `E_HIDDEN_ROUTE` | `page.go` ou `route.go` dentro de pasta cujo nome começa com ponto |
| `E_UNROUTABLE_METHOD` | `func HEAD`, `TRACE` ou `CONNECT`: o roteador não tira esses de um arquivo |
| `E_CORS_ON_PAGE` | `var CORS` num `page.go` |
