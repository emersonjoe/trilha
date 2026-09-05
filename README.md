# Trilha

**Framework web para Go no estilo Next.js.** Roteamento por arquivos, layouts aninhados,
rotas de API, middleware por pasta, dev server com recarga automática e um único binário
de produção. Zero dependências fora da biblioteca padrão.

```
app/
├── layout.go            → <html> raiz (envolve tudo)
├── page.go              → GET /
├── middleware.go        → roda em toda requisição
├── not_found.go         → página 404
├── error.go             → página 500
├── setup.go             → inicialização (banco, cache...)
├── blog/
│   ├── layout.go        → envolve /blog/**
│   ├── page.go          → GET /blog
│   ├── novo/page.go     → GET /blog/novo  (+ POST do formulário)
│   └── slug_/page.go    → GET /blog/{slug}
├── docs/path__/page.go  → GET /docs/{path...}
├── admin/
│   ├── middleware.go    → só para /admin/**
│   └── page.go
└── api/posts/route.go   → GET/POST /api/posts
public/style.css         → servido em /style.css
```

## Começando

```bash
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha new meu-app && cd meu-app
trilha dev              # → http://localhost:3000, recarrega ao salvar
trilha build            # → bin/meu-app, com public/ embutido
```

Ainda não publicado? Use a cópia local: `trilha new meu-app --trilha-dir ../trilha`.

## Convenções

| Arquivo | Exporta | Assinatura |
|---------|---------|------------|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` |
| `page.go` | `POST`, `PUT`, `PATCH`, `DELETE` (opcionais) | `func(c *trilha.Ctx) error` — formulários, com CSRF |
| `route.go` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE` | `func(c *trilha.Ctx) error` — API |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` |
| `not_found.go` (raiz) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` |
| `error.go` (raiz) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` |
| `setup.go` (raiz) | `Setup` | `func(a *trilha.App) error` |

Pastas viram segmentos: `blog` → `/blog`; `slug_` → `/{slug}`; `path__` → `/{path...}`
(catch-all, precisa ser folha). O `[slug]` do Next.js não é válido em import path do Go,
por isso o sufixo `_`. Pastas iniciadas por `_` ou `.` são ignoradas.

Ordem de execução para `GET /admin`:
`middleware(app) → middleware(app/admin) → Page → layout(app/admin)? → layout(app)`.

## Uma página

```go
package sobre

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Sobre")
	return h.Main(
		h.H1(h.Text("Sobre")),
		h.P(h.Textf("Você é a requisição %s.", c.RequestID())),
	), nil
}
```

`h` é um DSL de HTML tipado: elementos e atributos são funções, texto é escapado por
padrão e `h.Raw` é a única porta sem escape. `h.If`, `h.Map` e `h.Fragment` cobrem o
fluxo de controle.

## Formulários e APIs

```go
// app/blog/novo/page.go
func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Form(h.Method("post"), trilha.CSRFInput(c),
		h.Input(h.Name("titulo")), h.Button(h.Text("Publicar"))), nil
}

func POST(c *trilha.Ctx) error {
	p := posts.Create(c.Form("titulo"), c.Form("corpo"))
	return c.Redirect("/blog/" + p.Slug) // 303: POST → redirect → GET
}

// app/api/posts/route.go
func GET(c *trilha.Ctx) error { return c.JSON(200, posts.All()) }
func POST(c *trilha.Ctx) error {
	var in struct{ Title string `json:"title"` }
	if err := c.BindJSON(&in); err != nil { return err } // 400 / 413
	return c.JSON(201, posts.Create(in.Title, ""))
}
```

Erros são valores: `trilha.ErrNotFound` → 404 (HTML ou JSON conforme a rota),
`trilha.Redirect(url)` → 303, `trilha.Errorf(422, "...")` → status com mensagem,
qualquer outro `error` → 500 com stack só em dev. Métodos não exportados respondem 405
com `Allow`.

## Middleware

```go
// app/admin/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if ck, err := c.Cookie("session"); err != nil || ck.Value != "ok" {
		return trilha.RedirectCode("/login", 302)
	}
	c.Set("user", "admin") // páginas leem com c.Get("user")
	return next()
}
```

## Como funciona

`trilha gen` varre `app/` com `go/ast` e escreve `trilha_gen.go` (commitado): um
`package main` que importa cada pacote de rota e chama `a.Register(...)` com tipos
verificados pelo compilador. Nada de `reflect`, nada de mágica em runtime; `go build .`
funciona sem a CLI. O roteador é o `http.ServeMux` do Go 1.22+.

`trilha dev` escuta em `:3000`, compila o app numa porta interna, faz proxy e injeta um
script de live-reload (SSE). Ao salvar: regenera, recompila, troca o processo e avisa o
navegador — cerca de 1 s no exemplo. Erro de compilação vira uma página com a saída do
`go build` que some sozinha quando você corrige.

Segurança por padrão: escape de HTML, `nosniff`/`X-Frame-Options`/`Referrer-Policy`,
limite de corpo (1 MiB), CSRF por double-submit cookie em formulários, estáticos sem path
traversal, logs `slog` sem corpo nem cookies.

## Fora do escopo (por enquanto)

Componentes cliente/hidratação, streaming, geração estática, grupos de rota `(grupo)`,
rotas paralelas. Interatividade no cliente fica em `public/*.js` (ou htmx).

## Desenvolvimento

```bash
make test        # gofmt + vet + go test ./... (inclui e2e da CLI e o exemplo)
make dev-example # trilha dev em examples/blog
make reload      # mede o ciclo editar→ver
```

Projeto guiado por spec-kit: veja `specs/001-trilha-core/` e `.specify/memory/constitution.md`.

---

## English

Trilha is a Next.js-style web framework for Go: file-based routing under `app/`
(`page.go`, `route.go`, `layout.go`, `middleware.go`), nested layouts, typed HTML DSL,
CSRF-protected forms, a dev server with live reload and a single production binary with
`public/` embedded. Dynamic segments use `name_` (`/{name}`) and `name__` (`/{name...}`)
because `[name]` is not a valid Go import path. Standard library only. Run
`trilha new app && cd app && trilha dev`.
