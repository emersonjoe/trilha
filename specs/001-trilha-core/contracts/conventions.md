# Contrato: convenções de arquivo em `app/`

| Arquivo | Função exigida | Assinatura | Escopo |
|---------|----------------|------------|--------|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` | rota da pasta |
| `page.go` | `POST`/`PUT`/`PATCH`/`DELETE` (opcional) | `func(c *trilha.Ctx) error` | formulários (CSRF exigido) |
| `route.go` | `GET`/`POST`/`PUT`/`PATCH`/`DELETE` (≥1) | `func(c *trilha.Ctx) error` | API da pasta |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` | subárvore |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` | subárvore |
| `not_found.go` (raiz) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` | app |
| `error.go` (raiz) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` | app |
| `setup.go` (raiz) | `Setup` | `func(a *trilha.App) error` | boot |

Pastas: literal → segmento literal; `nome_` → `{nome}`; `nome__` → `{nome...}` (folha);
`nome-` → grupo de rota (sem segmento na URL; layout/middleware valem para a subárvore) — spec 002.
`public/` (na raiz do projeto, fora de `app/`) → servido em `/`.
Ignorados: pastas iniciadas por `_` ou `.`, `testdata`, arquivos `_test.go`.

Ordem de execução de uma requisição a `/a/b`:
`middleware(app) → middleware(app/a) → middleware(app/a/b) → handler → layout(app/a/b) → layout(app/a) → layout(app)`.

Erros de geração têm o formato `app/blog/page.go: E_NO_PAGE_FUNC: page.go precisa exportar func Page(c *trilha.Ctx) (h.Node, error)`.
