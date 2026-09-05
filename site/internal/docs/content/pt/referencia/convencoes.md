---
title: Convenções de arquivo
description: Tabela completa do que cada arquivo e nome de pasta em app/ significa.
---

## Arquivos

| Arquivo | Função exportada | Assinatura | Alcance |
|---|---|---|---|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` | rota GET da pasta |
| `page.go` | `POST`, `PUT`, `PATCH`, `DELETE` (opcionais) | `func(c *trilha.Ctx) error` | formulários; CSRF exigido |
| `route.go` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE` (ao menos um) | `func(c *trilha.Ctx) error` | API JSON da pasta |
| `route.go` (opcional) | `Kind` | `var Kind = trilha.KindPage` ou `KindAPI` | como erros são renderizados e se há CSRF (veja [Erros](/pt/referencia/erros)) |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` | subárvore |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` | subárvore |
| `not_found.go` (só na raiz) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` | 404 do app |
| `error.go` (só na raiz) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` | 500 do app |
| `setup.go` (só na raiz) | `Setup` | `func(a *trilha.App) error` | antes de servir |
| `setup.go` (opcional) | `Config` | `func(cfg *trilha.Config)` ou `func(cfg *trilha.Config) error` | antes de `trilha.New`; o erro interrompe a subida |
| `setup.go` (opcional) | `Shutdown` | `func(a *trilha.App) error` | depois de parar de aceitar requisições (fechar pool, fila, flush de log) |

`page.go` e `route.go` na mesma pasta é erro. A função pode estar em qualquer arquivo do
pacote; o nome do arquivo é o que liga a convenção.

## Pastas

| Nome | Vira | Exemplo |
|---|---|---|
| `eventos` | segmento literal | `/eventos` |
| `slug_` | parâmetro `{slug}` | `/eventos/{slug}` → `c.Param("slug")` |
| `caminho__` | catch-all `{caminho...}`; precisa ser folha | `/docs/{caminho...}` |
| `organizador-` | grupo de rota; não entra na URL | layout/middleware para a subárvore |
| `app.css`, `robots.txt` | caminho fixo com extensão (ponto no meio do nome) | `/app.css`, `/manifest.webmanifest`, `/sw.js` |
| `_x`, `.x`, `testdata` | ignoradas | — |

Uma pasta com ponto no nome serve um caminho fixo com extensão. Como `app.css` não é um
identificador Go, declare outro nome de pacote no arquivo (`package appcss`); o gerador
importa tudo com alias, então o nome do pacote não importa. Pastas que **começam** com
ponto continuam ignoradas.

Precedência: literal vence parâmetro, que vence catch-all. Duas pastas dinâmicas irmãs são
erro. Duas pastas que gerem a mesma URL (via grupos) são erro.

## Outras pastas do projeto

| Pasta | Papel |
|---|---|
| `public/` | arquivos estáticos servidos na raiz; embutidos no binário em produção |
| `trilha_gen.go` | gerado; commitado; nunca editado à mão |
| `.trilha/` | binários temporários do `dev` e do `export`; ignorada pelo git |

## Ordem de execução de `GET /a/b`

```text
middleware(app) → middleware(app/a) → middleware(app/a/b)
  → Page (ou método)
  → layout(app/a/b) → layout(app/a) → layout(app)
```

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
| `E_PARSE` | arquivo Go que não compila |
| `E_NO_APP` | não há pasta `app/` |
