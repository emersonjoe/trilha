---
title: Pacote h
description: Referência do DSL de HTML: nós, elementos, atributos e fluxo de controle.
---

```go
import "github.com/emersonjoe/trilha/h"
```

## O tipo Node

```go
type Node interface {
	Render(w io.Writer) error
}
```

Qualquer valor com esse método pode ser filho de um elemento. `h.Render(n) (string, error)`
é a conveniência para testes.

## Texto e estrutura

| Função | Saída |
|---|---|
| `Text(s)` | texto escapado |
| `Textf(fmt, a...)` | texto formatado e escapado |
| `Raw(html)` | HTML sem escape — a única porta |
| `Fragment(filhos...)`, `Group(...)` | filhos em sequência, sem elemento em volta |
| `Doctype()` | `<!doctype html>` |
| `Nil` | nó vazio |
| `El(tag, filhos...)` | elemento com tag arbitrária |
| `Void(tag, attrs...)` | elemento vazio arbitrário |

## Fluxo de controle

| Função | Comportamento |
|---|---|
| `If(cond, n)` | `n` se verdadeiro, vazio se falso |
| `IfElse(cond, a, b)` | `a` ou `b` |
| `Map(itens, f)` | `f(item)` para cada item |
| `MapIndex(itens, f)` | `f(i, item)` |

`nil` como filho é ignorado.

## Elementos

Todos os elementos HTML de uso comum têm uma função com o nome em maiúscula inicial:
`Html`, `Head`, `Body`, `Title`, `Meta`, `Link`, `Script`, `Style`, `Div`, `Span`, `P`,
`A`, `Ul`, `Ol`, `Li`, `H1`…`H6`, `Header`, `Footer`, `Main`, `Nav`, `Section`, `Article`,
`Aside`, `Form`, `Input`, `Button`, `Label`, `Select`, `Option`, `Textarea`, `Table`,
`Thead`, `Tbody`, `Tr`, `Th`, `Td`, `Img`, `Br`, `Hr`, `Pre`, `Code`, `Strong`, `Em`,
`Small`, `Time`, `Details`, `Summary`, `Dialog`, `Figure`, `Picture`, `Video`, `Audio`,
`Canvas`, `Iframe`, `Svg`, `Template`, entre outros. Elementos vazios (`Br`, `Img`, `Input`,
`Meta`, `Link`, `Hr`, `Source`, `Track`, `Wbr`, `Area`, `Col`, `Embed`, `Base`) aceitam só
atributos.

## Atributos

| Função | Atributo |
|---|---|
| `Attr(nome, valor)` | qualquer atributo, valor escapado |
| `Bool(nome)` | atributo booleano |
| `Class(v...)` | `class`, juntando com espaço e ignorando vazios |
| `ID`, `Href`, `Src`, `Alt`, `Type`, `Name`, `Value`, `Placeholder`, `Action`, `Method`, `Rel`, `Lang`, `Charset`, `Content`, `For`, `Role`, `Target`, `Width`, `Height`, `Rows`, `Cols`, `Min`, `Max`, `Step`, `Pattern`, `Enctype`, `Accept`, `Datetime`, `Tabindex`, `Onclick` | o atributo de mesmo nome |
| `StyleAttr`, `TitleAttr`, `LabelAttr` | `style`, `title`, `label` (os nomes sem sufixo são elementos) |
| `Data(chave, v)`, `Aria(chave, v)` | `data-chave`, `aria-chave` |
| `Disabled()`, `Checked()`, `Selected()`, `Required()`, `Autofocus()`, `Hidden()`, `Readonly()`, `Multiple()`, `Open()`, `Defer()`, `Async()`, `Autoplay()`, `Controls()`, `Novalidate()` | booleanos |

Atributos podem aparecer em qualquer posição entre os filhos; são escritos na tag de
abertura, na ordem em que aparecem.
