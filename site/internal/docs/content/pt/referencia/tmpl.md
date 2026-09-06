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
| `Wrap(t *template.Template, nome, slot string) *Shell` | prepara uma casca para receber um `h.Node` onde ela chama `{{template slot .}}`; chame em nível de pacote — ele clona o conjunto, e o `html/template` só clona um conjunto que ainda não executou |
| `(*Shell) Node(dados any, filhos h.Node) h.Node` | renderiza a casca com `dados` e os `filhos` no slot; casca que nunca chega ao slot é erro de render |
| `HTML(n h.Node) (template.HTML, error)` | o nó como `template.HTML`, para dados de um template que o próprio app executa |

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

## O caminho inverso: um h.Node dentro de um template

A casca de um app que já existe em `html/template`, com as telas novas escritas em `h`, é o
meio do caminho de toda migração. O `Wrap` liga os dois:

```go
var casca = tmpl.Wrap(tmpl.Must(arquivos, "*.html"), "casca", "conteudo")

func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return casca.Node(pagina(c.Request()), children), nil
}
```

O template continua com a forma que tinha — o slot é o `{{template "conteudo" .}}` que já
estava lá — e nada no app converte coisa nenhuma para `template.HTML`: o que o `h`
renderizou foi escapado na entrada, e o `tmpl` é o único lugar que afirma isso. Os dados da
casca podem sair só do `*http.Request`, inclusive o
[`trilha.NonceFrom(r)` e o `trilha.CSRFTokenFrom(r)`](/pt/referencia/ctx). O `examples/blog`
tem a coisa inteira em `app/legado-`.

O `Wrap` clona o conjunto, então chame em nível de pacote: o `html/template` recusa clonar um
conjunto que já executou. Uma casca que nunca chega ao slot — um `{{if}}` que o escondeu, o
nome errado — quebra o render com `tmpl: template %q never rendered the slot` em vez de
responder calada uma página sem conteúdo.

O `HTML(n)` é a saída de baixo nível, para um template que o próprio app executa.
