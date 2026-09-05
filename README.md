# Trilha

[![ci](https://github.com/emersonjoe/trilha/actions/workflows/ci.yml/badge.svg)](https://github.com/emersonjoe/trilha/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/emersonjoe/trilha.svg)](https://pkg.go.dev/github.com/emersonjoe/trilha)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Framework web para Go com roteamento por arquivos.** Layouts aninhados, rotas de API,
middleware por pasta, dev server com recarga automática e um único binário de produção.
Zero dependências fora da biblioteca padrão. A organização por pastas segue o modelo
popularizado pelo Next.js\*, traduzido para as convenções do Go.

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
├── marketing-/          → grupo: não aparece na URL
│   ├── layout.go        → envolve /precos e /sobre
│   ├── precos/page.go   → GET /precos
│   └── sobre/page.go    → GET /sobre
├── admin/
│   ├── middleware.go    → só para /admin/**
│   └── page.go
└── api/posts/route.go   → GET/POST /api/posts
public/style.css         → servido em /style.css
```

## Documentação

**<https://emersonjoe.github.io/trilha>** — trilha "Aprender" (do `trilha new` ao deploy, com
desafios) e "Referência" por pacote. O site é um app Trilha, exportado com `trilha export`.

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
| `setup.go` (opcional) | `Config` | `func(cfg *trilha.Config)`, antes de `trilha.New` |

Pastas viram segmentos: `blog` → `/blog`; `slug_` → `/{slug}`; `path__` → `/{path...}`
(catch-all, precisa ser folha); `marketing-` → **grupo de rota**: não entra na URL, mas
seu `layout.go`/`middleware.go` valem para tudo abaixo (o equivalente ao `(marketing)` do Next.js).
`[slug]` e `(grupo)` não são válidos em import path do Go, por isso os sufixos `_` e `-`.
Pastas iniciadas por `_` ou `.` são ignoradas. Duas pastas que gerem a mesma URL são erro
de geração (`E_DUPLICATE_ROUTE`).

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

Prefere `html/template`? O pacote `tmpl` encaixa templates no mesmo pipeline (layouts,
título, escape contextual do próprio `html/template`):

```go
//go:embed relatorio.html
var files embed.FS
var t = tmpl.Must(files, "*.html") // falha na subida, nunca no request

func Page(c *trilha.Ctx) (h.Node, error) {
	return tmpl.Node(t, "relatorio", dados), nil
}
```

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

## Interface

Projetos novos vêm com o kit `ui`: componentes tipados (`ui.Button`, `ui.Card`, `ui.Field`,
`ui.Tabs`, `ui.Dialog`...) sobre um CSS prefixado e um JS de 200 linhas, ambos copiados
para `public/` e seus para editar. O tema usa as mesmas variáveis do
[shadcn/ui](https://ui.shadcn.com) (MIT): cole um tema pronto em `public/ui.theme.css` e
nada em Go muda. `trilha ui` atualiza o kit sem tocar no seu tema.

```go
ui.Card(
	ui.CardHeader(ui.CardTitle("Novo post")),
	ui.CardContent(h.Form(h.Method("post"), trilha.CSRFInput(c),
		ui.Field("titulo", "Título", ui.Input(h.ID("titulo"), h.Name("titulo"), h.Required())),
		ui.Submit(h.Text("Publicar")))),
)
```

## Exemplos

| Nível | Pasta | Ensina |
|---|---|---|
| Básico | `examples/blog` | convenções, layouts, API, middleware, sessão |
| Médio | `examples/cadastro` | formulário com regras: campos condicionais, validação por campo (`c.Bind`, `trilha.FieldErrors`, `c.Render`), seleção dependente, aviso que some |
| Complexo | `examples/orcamento` | plano de contas em árvore, drill-down, componentes recursivos, diálogo, CSV |
| IA | `examples/assistente` | chat em streaming, agente com ferramentas, MCP |

## IA e agentes

`ai` fala o protocolo de chat da OpenAI (funciona com OpenAI, Groq, Mistral, OpenRouter, Ollama,
LM Studio, vLLM...), com ferramentas tipadas, agentes, handoffs e streaming; `ai/mcp` usa e
expõe ferramentas pelo Model Context Protocol. Tudo sem dependências externas.

```go
clima := ai.NewTool("clima", "Temperatura em uma cidade.",
    ai.Schema(`{"type":"object","properties":{"cidade":{"type":"string"}},"required":["cidade"]}`),
    ai.Typed(func(ctx context.Context, in struct{ Cidade string }) (string, error) {
        return buscarTemperatura(ctx, in.Cidade)
    }))
agente := &ai.Agent{Name: "Assistente", Instructions: "Responda em português.", Tools: []*ai.Tool{clima}}
res, err := ai.Run(ctx, ai.NewFromEnv(), agente, "Está frio em Curitiba?")
```

Veja `examples/assistente` (chat em streaming com `c.Stream()`, handoff para um tradutor e
servidor MCP em `/mcp`) e o capítulo [IA e agentes](https://emersonjoe.github.io/trilha/aprender/ia-e-agentes).

## Como funciona

`trilha gen` varre `app/` com `go/ast` e escreve `trilha_gen.go` (commitado): um
`package main` que importa cada pacote de rota e chama `a.Register(...)` com tipos
verificados pelo compilador. Nada de `reflect`, nada de mágica em runtime; `go build .`
funciona sem a CLI. O roteador é o `http.ServeMux` do Go 1.22+.

`trilha dev` escuta em `:3000`, compila o app numa porta interna, faz proxy e injeta um
script de live-reload (SSE). Ao salvar: regenera, recompila, troca o processo e avisa o
navegador — cerca de 1 s no exemplo. Mudanças só em `public/` não recompilam: o navegador
recarrega em dezenas de milissegundos. Erro de compilação vira uma página com a saída do
`go build` que some sozinha quando você corrige.

Segurança por padrão: escape de HTML, `nosniff`/`X-Frame-Options`/`Referrer-Policy`,
limite de corpo (1 MiB), CSRF por double-submit cookie em formulários, estáticos sem path
traversal, logs `slog` sem corpo nem cookies.

## Fora do escopo (por enquanto)

Componentes cliente/hidratação e rotas paralelas. Interatividade no cliente fica em `public/*.js` (ou htmx).

## Licença

MIT (`LICENSE`). Os arquivos do spec-kit em `.specify/` e `.claude/skills` são MIT da GitHub,
Inc.; veja `THIRD_PARTY_NOTICES.md`.

\* Next.js é marca da Vercel, Inc. O Trilha é um projeto independente, sem afiliação, e não
contém código do Next.js.

Contribuições são bem-vindas: veja [CONTRIBUTING.md](CONTRIBUTING.md), o
[código de conduta](CODE_OF_CONDUCT.md), a [política de segurança](SECURITY.md) e a
[governança](GOVERNANCE.md). Mudanças de comportamento seguem o fluxo spec-kit em `specs/`.

## Desenvolvimento

```bash
make test        # gofmt + vet + go test ./... (inclui e2e da CLI e o exemplo)
make dev-example # trilha dev em examples/blog
make reload      # mede o ciclo editar→ver
```

Projeto guiado por spec-kit: veja `specs/` (001 núcleo, 002 grupos/templates/estáticos, 003 export, 004 segurança, 005 IA) e
`.specify/memory/constitution.md`.

---

## English

Trilha is a file-based web framework for Go (routing conventions inspired by Next.js, no
affiliation): routes live under `app/`
(`page.go`, `route.go`, `layout.go`, `middleware.go`), nested layouts, typed HTML DSL,
CSRF-protected forms, a dev server with live reload and a single production binary with
`public/` embedded. Dynamic segments use `name_` (`/{name}`) and `name__` (`/{name...}`)
because `[name]` is not a valid Go import path; `name-` is a route group (Next's `(name)`).
Prefer templates? `tmpl.Node(t, "name", data)` plugs `html/template` into the same pipeline. Standard library only. Run
`trilha new app && cd app && trilha dev`.
