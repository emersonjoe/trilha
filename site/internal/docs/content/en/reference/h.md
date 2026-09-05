---
title: Package h
description: Reference for the HTML DSL: nodes, elements, attributes and control flow.
---

```go
import "github.com/emersonjoe/trilha/h"
```

## The Node type

```go
type Node interface {
	Render(w io.Writer) error
}
```

Any value with that method can be a child of an element. `h.Render(n) (string, error)` is
the convenience for tests.

## Text and structure

| Function | Output |
|---|---|
| `Text(s)` | escaped text |
| `Textf(fmt, a...)` | formatted and escaped text |
| `Raw(html)` | unescaped HTML — the only door |
| `Fragment(children...)`, `Group(...)` | children in sequence, without a wrapping element |
| `Doctype()` | `<!doctype html>` |
| `Nil` | empty node |
| `El(tag, children...)` | element with an arbitrary tag |
| `Void(tag, attrs...)` | arbitrary void element |

## Control flow

| Function | Behavior |
|---|---|
| `If(cond, n)` | `n` if true, empty if false |
| `IfElse(cond, a, b)` | `a` or `b` |
| `Map(items, f)` | `f(item)` for each item |
| `MapIndex(items, f)` | `f(i, item)` |

`nil` as a child is ignored.

## Elements

Every commonly used HTML element has a function with a capitalized name: `Html`, `Head`,
`Body`, `Title`, `Meta`, `Link`, `Script`, `Style`, `Div`, `Span`, `P`, `A`, `Ul`, `Ol`,
`Li`, `H1`…`H6`, `Header`, `Footer`, `Main`, `Nav`, `Section`, `Article`, `Aside`, `Form`,
`Input`, `Button`, `Label`, `Select`, `Option`, `Textarea`, `Table`, `Thead`, `Tbody`, `Tr`,
`Th`, `Td`, `Img`, `Br`, `Hr`, `Pre`, `Code`, `Strong`, `Em`, `Small`, `Time`, `Details`,
`Summary`, `Dialog`, `Figure`, `Picture`, `Video`, `Audio`, `Canvas`, `Iframe`, `Svg`,
`Template`, among others. Void elements (`Br`, `Img`, `Input`, `Meta`, `Link`, `Hr`,
`Source`, `Track`, `Wbr`, `Area`, `Col`, `Embed`, `Base`) accept only attributes.

## Attributes

| Function | Attribute |
|---|---|
| `Attr(name, value)` | any attribute, escaped value |
| `Bool(name)` | boolean attribute |
| `Class(v...)` | `class`, joined with spaces and skipping empty values |
| `ID`, `Href`, `Src`, `Alt`, `Type`, `Name`, `Value`, `Placeholder`, `Action`, `Method`, `Rel`, `Lang`, `Charset`, `Content`, `For`, `Role`, `Target`, `Width`, `Height`, `Rows`, `Cols`, `Min`, `Max`, `Step`, `Pattern`, `Enctype`, `Accept`, `Datetime`, `Tabindex`, `Onclick` | the attribute of the same name |
| `StyleAttr`, `TitleAttr`, `LabelAttr` | `style`, `title`, `label` (the names without the suffix are elements) |
| `Data(key, v)`, `Aria(key, v)` | `data-key`, `aria-key` |
| `Disabled()`, `Checked()`, `Selected()`, `Required()`, `Autofocus()`, `Hidden()`, `Readonly()`, `Multiple()`, `Open()`, `Defer()`, `Async()`, `Autoplay()`, `Controls()`, `Novalidate()` | booleans |

Attributes may appear at any position among the children; they are written in the opening
tag, in the order they appear.
