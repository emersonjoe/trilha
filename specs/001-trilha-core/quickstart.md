# Quickstart: Trilha

```bash
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha new meu-app && cd meu-app
trilha dev            # → http://localhost:3000
```

Estrutura criada:

```
meu-app/
├── go.mod
├── trilha_gen.go          # gerado, commitar
├── public/style.css
└── app/
    ├── layout.go          # <html> raiz
    ├── page.go            # GET /
    ├── not_found.go
    └── api/hello/route.go # GET /api/hello
```

Nova página: `app/sobre/page.go`

```go
package sobre

import ("github.com/emersonjoe/trilha"; "github.com/emersonjoe/trilha/h")

func Page(c *trilha.Ctx) (h.Node, error) {
    c.SetTitle("Sobre")
    return h.Main(h.H1(h.Text("Sobre")), h.P(h.Text("Feito com Trilha."))), nil
}
```

Rota dinâmica: `app/blog/slug_/page.go` → `/blog/{slug}` com `c.Param("slug")`.
Formulário: exporte `POST` no mesmo `page.go` e inclua `trilha.CSRFInput(c)` no `<form>`.
Produção: `trilha build && ./bin/meu-app` (porta via `PORT`, `TRILHA_ENV=prod` padrão).
