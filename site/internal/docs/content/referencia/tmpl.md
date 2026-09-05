---
title: Pacote tmpl
description: Usar html/template dentro do pipeline de páginas e layouts.
---

```go
import "github.com/emersonjoe/trilha/tmpl"
```

| Função | Descrição |
|---|---|
| `Node(t *template.Template, nome string, dados any) h.Node` | nó que executa o template nomeado; erro de execução vira erro de render (500), sem saída parcial |
| `Must(fsys fs.FS, padrões ...string) *template.Template` | `template.ParseFS` com pânico em erro; chame em nível de pacote para falhar na subida |

## Uso

```go
//go:embed *.html
var arquivos embed.FS

var t = tmpl.Must(arquivos, "*.html")

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Relatório")
	return tmpl.Node(t, "relatorio", dados), nil
}
```

O template usa `{{define "relatorio"}}...{{end}}` e recebe `dados` como `.`. O escape é o
contextual do `html/template`. O nó pode ser combinado com o DSL:
`h.Section(h.Class("x"), tmpl.Node(t, "parte", d))`.
