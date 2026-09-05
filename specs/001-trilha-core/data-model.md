# Data Model: Trilha

## Route (interno — `internal/scan`)

| Campo | Tipo | Descrição |
|-------|------|-----------|
| Pattern | string | `/blog/{slug}`; raiz é `/` |
| Kind | enum page \| api | derivado de `page.go` ou `route.go` |
| Dir | string | caminho relativo à raiz do app (`app/blog/slug_`) |
| ImportPath | string | `github.com/x/app/blog/slug_` |
| PkgName | string | nome declarado no arquivo (`slug`) |
| Alias | string | alias único e válido para o import (`app_blog_slug_`) |
| Methods | []string | funções exportadas entre GET/POST/PUT/PATCH/DELETE (api) ou POST/PUT/PATCH/DELETE (page) |
| HasPage | bool | `Page` exportada (só page) |
| Layouts | []Ref | de dentro para fora, cada `layout.go` ancestral (inclusive a própria pasta) |
| Middlewares | []Ref | de fora para dentro, cada `middleware.go` ancestral |
| Segments | []Segment | cada segmento: literal, `{name}` ou `{name...}` |

**Ref** = `{Alias, ImportPath, Func}`.

**Regras de validação** (erro de geração):
- `page.go` e `route.go` na mesma pasta → `E_PAGE_AND_ROUTE`.
- `page.go` sem `Page` → `E_NO_PAGE_FUNC`; `route.go` sem nenhum método → `E_NO_METHOD`.
- Duas pastas irmãs dinâmicas (`a_`, `b_` ou `a__`) → `E_AMBIGUOUS_SEGMENT`.
- Catch-all (`x__`) com subpastas contendo rotas → `E_CATCHALL_NOT_LEAF`.
- Nome de segmento inválido (não identificador Go) → `E_BAD_SEGMENT`.
- Pastas `_x`, `.x`, `testdata` são ignoradas (regra do go tool).

## Tree (interno)

Árvore de diretórios com `Layout *Ref`, `Middleware *Ref`, `NotFound *Ref`, `Error *Ref`,
`Setup *Ref` (só raiz) e filhos ordenados por nome. `NotFound`/`Error` só são lidos na raiz na v1.

## Runtime (`package trilha`)

- **App**: `Config{Addr, Env, MaxBodyBytes, Logger, Public fs.FS, CSRFForAPI}`, mux,
  `Values map[string]any` (globais definidos por `Setup`), `notFound`/`errorPage` handlers.
- **Ctx**: `w http.ResponseWriter`, `r *http.Request`, `app *App`, `values map[string]any`,
  `title string`, `status int` (já escrito?), `kind` (page|api) para formato de erro.
- **Next**: `func() error`.
- **PageFunc**: `func(*Ctx) (h.Node, error)`; **LayoutFunc**: `func(*Ctx, h.Node) (h.Node, error)`;
  **HandlerFunc**: `func(*Ctx) error`; **MiddlewareFunc**: `func(*Ctx, Next) error`;
  **ErrorPageFunc**: `func(*Ctx, error) (h.Node, error)`.
- **Erros**: `ErrNotFound` (sentinela), `*RedirectError{URL, Code}`, `*HTTPError{Code, Msg}`.

## h (`package h`)

- **Node**: `interface{ Render(w io.Writer) error }`.
- **Element**: tag, void bool, children []Node; ao renderizar separa atributos (`isAttr()`).
- **Attr**: name, value (escapado com `html.EscapeString`); atributo booleano sem valor.
- **Text**: escapado. **Raw**: sem escape. **Fragment**: lista. **Doctype**.
- **If(cond, node)**, **Map(items, fn)**, **Group(nodes...)** devolvem `Node`.

## CLI (`cmd/trilha`)

Comandos: `new <dir>`, `gen`, `dev [--port 3000]`, `build [-o bin/nome]`, `routes`.
Estado do `dev`: `idle | building | running | failed(err)`; transições disparadas pelo watcher.
