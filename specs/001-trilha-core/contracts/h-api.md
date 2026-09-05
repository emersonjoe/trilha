# Contrato: DSL `h`

```go
package h

type Node interface{ Render(w io.Writer) error }

func Render(n Node) (string, error)            // conveniência para testes

// Estruturais
func Doctype() Node
func Text(s string) Node                        // escapado
func Textf(format string, a ...any) Node
func Raw(html string) Node                      // SEM escape — única porta
func Fragment(children ...Node) Node
func If(cond bool, n Node) Node
func IfElse(cond bool, a, b Node) Node
func Map[T any](items []T, f func(T) Node) Node
func El(tag string, children ...Node) Node      // elemento arbitrário
func Void(tag string, attrs ...Node) Node

// Elementos (gerados): Html, Head, Body, Title, Meta, Link, Script, Style,
// Div, Span, P, A, Ul, Ol, Li, H1..H6, Header, Footer, Main, Nav, Section, Article,
// Aside, Form, Input, Button, Label, Select, Option, Textarea, Table, Thead, Tbody,
// Tr, Th, Td, Img, Br, Hr, Pre, Code, Strong, Em, Small, Time, Details, Summary, ...

// Atributos (gerados/manuais)
func Attr(name, value string) Node
func Bool(name string) Node                     // ex.: h.Bool("disabled")
func Class(v ...string) Node                    // junta com espaço, ignora vazios
func ID, Href, Src, Alt, Type, Name, Value, Placeholder, Action, Method, Rel, Lang,
     Charset, Content, For, Role, Target, Width, Height (string) Node
func StyleAttr, TitleAttr, LabelAttr (string) Node   // Style, Title e Label são elementos
func Data(key, value string) Node               // data-key
func Aria(key, value string) Node               // aria-key
```

Regras: texto e valores de atributo escapam `& < > " '`; atributos são escritos na ordem em
que aparecem; filhos após atributos; elementos void (`Br`, `Img`, `Input`, `Meta`, `Link`,
`Hr`) não fecham e ignoram filhos não-atributo. `nil` como filho é ignorado.
