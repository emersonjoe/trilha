# Implementation Plan: Kit de UI (006-ui)

**Branch**: `006-ui` | **Spec**: spec.md

## Constitution Check

| Princípio | Como a feature respeita |
|---|---|
| I Convenção | não muda rotas; `public/ui*.css|js` são estáticos comuns |
| II Só stdlib | `ui` usa `embed`, `strings`, `h`; JS/CSS são arquivos, não dependências |
| III Geração explícita | `trilha new`/`trilha ui` gravam arquivos visíveis; nada em runtime |
| IV Contrato de handler | inalterado; `ui.Head(c)` é um `h.Node` |
| V Dev/prod | CSS/JS em `public/` → recarga sem rebuild; embutidos no binário |
| VI Teste primeiro | testes do pacote `ui`, e2e da CLI, integração dos exemplos, navegador |
| VII Segurança | escape via `h`; `ui.js` externo (CSP `'self'`); ícones são SVG fixos em código, não entrada do usuário |

Emenda à constituição (seção "Estilo e idioma" → "Estilo, idioma e interface"): "O kit
`ui` é a interface padrão dos projetos gerados e dos exemplos; é copiado para o projeto e
customizável; compatível com o contrato de tema do shadcn/ui (MIT)". Versão 1.1.0.

## Design

### Arquivos
```
ui/
  ui.go          // Head, Button, Card..., helpers de classe
  icons.go       // Icon(name) com SVGs Lucide (ISC), mapa fixo
  assets/ui.css  // componentes (classes ui-*)
  assets/ui.theme.css // :root/.dark com variáveis shadcn v4 (neutral)
  assets/ui.js   // comportamentos
  ui_test.go
internal/scaffold: templates usam ui; função WriteUI(dir, opts) reutilizada por `trilha ui`
cmd/trilha/ui.go  // comando
examples/blog, examples/assistente: layouts e páginas com ui
site: content/aprender/interface-com-ui.md, referencia/ui.md, demos vivas
```

### API do pacote `ui` (h.Node por toda parte)
```go
func Head(c *trilha.Ctx) h.Node                       // link ui.theme.css + ui.css, script ui.js defer
func Button(children ...h.Node) h.Node                // variantes: ui.Secondary(), ui.Outline(), ui.Ghost(), ui.Destructive(), ui.LinkStyle(); tamanhos ui.Sm(), ui.Lg(), ui.IconSize()
func Card / CardHeader / CardTitle / CardDescription / CardContent / CardFooter
func Input(attrs ...h.Node) h.Node; Textarea; Select(children...); Checkbox(attrs...); Switch(attrs...)
func Label(children ...h.Node) h.Node
func Field(label string, control h.Node, opts ...FieldOpt) h.Node  // Help(s), Error(s)
func Badge(children...) (variantes Secondary/Outline/Destructive)
func Alert(title string, children...) (Destructive)
func Table(children...) + THead/TBody/Tr/Th/Td finos (só classes)
func Tabs(id string, tabs ...Tab) h.Node; type Tab struct{Label string; Content h.Node}
func Dialog(id, title string, children...) h.Node; DialogTrigger(id, children...) h.Node
func Separator(); Skeleton(class...); Progress(value, max int); Breadcrumb(items ...Crumb); Avatar(initials, imgSrc)
func Toast(kind, text string, fadeMs int) h.Node       // data-ui-fade
func Collapsible(summary string, children...) h.Node   // <details>
func Icon(name string, attrs ...h.Node) h.Node         // pânico em nome desconhecido (erro de programação)
func ShowWhen(field, value string) h.Node              // atributo data-ui-show-when
func ThemeToggle() h.Node
```
Variantes são `h.Node` de classe adicional, compostas como qualquer atributo do `h`:
`ui.Button(ui.Outline(), ui.Sm(), h.Text("Cancelar"))`.

### Tema
`ui.theme.css` reproduz os nomes e valores do tema neutro do shadcn/ui v4 (oklch),
`--radius: 0.625rem`, derivados `--radius-sm/md/lg/xl`. `ui.css` consome apenas
`var(--…)`. Modo escuro: `.dark` ou `@media (prefers-color-scheme: dark)` quando não há
`.light`/`.dark` explícito.

### CLI
`trilha ui`: compara o conteúdo atual dos três arquivos com o embutido (hash); grava os
que não existem; para os que existem e diferem: `ui.theme.css` nunca é sobrescrito (só
criado), `ui.css`/`ui.js` só com `--force`. Saída lista o que fez.

## Riscos
- Tamanho do CSS: manter foco em ~22 componentes; medir no teste (FR-007).
- Contraste: usar valores do tema neutro (já auditados pelo shadcn/ui).
