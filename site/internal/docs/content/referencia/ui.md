---
title: ui
description: Componentes do kit, variantes, assets e o contrato de tema.
---

`import "github.com/emersonjoe/trilha/ui"` — só stdlib. Os componentes devolvem `h.Node`
com classes `ui-*` de `public/ui.css`; comportamentos em `public/ui.js`.

## Assets

| Símbolo | Papel |
|---|---|
| `ui.Head(c) h.Node` | `<link>` para `ui.theme.css` e `ui.css`, script inline (com nonce) que aplica o tema salvo, `<script defer src=ui.js>`; respeita `c.Base()` |
| `ui.Body() h.Node` | classe `ui-body` para o `<body>` |
| `ui.Asset(nome) []byte` | conteúdo embutido de `ui.css`, `ui.theme.css` ou `ui.js` |
| `ui.Files` | os três nomes, na ordem em que `trilha ui` os grava |

## Variantes e tamanhos

`ui.Secondary()`, `ui.Outline()`, `ui.Ghost()`, `ui.Destructive()`, `ui.LinkStyle()`,
`ui.Sm()`, `ui.Lg()`, `ui.IconSize()`. São atributos de classe: valem em `Button`,
`Submit`, `ButtonLink`, `Badge` e `Alert` (cada um traduz para a sua classe, ex.
`ui-btn-outline`, `ui-badge-outline`).

## Componentes

| Função | Renderiza |
|---|---|
| `Container, Stack, Row, Grid, Spacer` | layout: largura máxima, coluna, linha, grade responsiva |
| `Header(children...)`, `Brand(href, nome)`, `Nav(...)`, `NavLink(href, rótulo, atual)`, `Sidebar(...)` | barra fixa no topo, marca, navegação (com `aria-current`), coluna lateral |
| `H1, H2, H3, Lead, Muted, Code(s), Kbd(s)` | tipografia |
| `Button, Submit, ButtonLink(href, ...)` | `<button type=button>`, `<button type=submit>`, `<a>` com cara de botão |
| `Card, CardHeader, CardTitle(s), CardDescription(s), CardContent, CardFooter` | cartão |
| `Input, Textarea, Select, Checkbox, Radio, Switch, Label` | controles (`Switch` tem `role=switch`) |
| `Field(id, rótulo, controle, opts...)` | rótulo + controle + `Help(s)` + `Error(s)`; `With(nós...)` põe atributos no grupo |
| `CheckRow(controle, rótulo, id)` | checkbox/switch ao lado do rótulo |
| `Invalid()` | `aria-invalid="true"` (anel vermelho) |
| `ShowWhen(campo, valores...)` | `data-ui-show-when`: mostra o elemento só com o valor (ou qualquer valor não vazio); controles escondidos são desabilitados |
| `Badge`, `Alert(título, ...)`, `AlertDescription(...)` | selo e aviso (`role=alert`) |
| `Toaster(...)`, `Toast(tipo, texto, fadeMs)` | pilha de avisos; `tipo` = `""`, `success`, `error`; `fadeMs > 0` some sozinho |
| `Table(...)`, `Num()`, `Depth(n)` | tabela rolável; célula numérica; indentação de linha (árvore) |
| `Tabs(id, Tab{Label, Content}...)` | abas acessíveis (setas, Home/End); a primeira começa aberta |
| `Dialog(id, título, ...)`, `DialogDescription(s)`, `DialogFooter(...)`, `DialogTrigger(id, ...)`, `DialogClose(...)` | `<dialog>` nativo com `showModal` |
| `Menu(id, ...)`, `MenuItem(...)`, `MenuLink(href, ...)`, `MenuTrigger(id, ...)` | menu com o atributo `popover` nativo |
| `Separator, Skeleton, Progress(valor, máx), Breadcrumb(Crumb{Label, Href}...), Avatar(iniciais, src), Collapsible(resumo, ...)` | diversos |
| `ThemeToggle()` | botão que alterna claro/escuro (`localStorage["ui-theme"]`) |
| `Icon(nome, attrs...)`, `Icons()` | SVG inline do Lucide; nome desconhecido → pânico (erro de programação) |

## ui.js

Tudo por atributo, sem inicialização: `[data-ui-tabs]`, `[data-ui-dialog-open=id]`,
`[data-ui-dialog-close]`, `[data-ui-fade=ms]`, `[data-ui-show-when]`, `[data-ui-toast=texto]`
(`data-ui-toast-kind`), `[data-ui-theme-toggle]`, `[popover].ui-menu`. Também expõe
`window.ui.toast(texto, {kind, ms})`, `ui.fade(el)`, `ui.evalShowWhen(root)` e
`ui.applyTheme("dark"|"light")`. Elementos inseridos depois (HTMX, fetch) precisam de
`ui.evalShowWhen(el)`/`ui.fade(el)` se usarem esses atributos.

## Tema

`ui.theme.css` define, em `:root` e `.dark`, exatamente as variáveis do shadcn/ui v4:
`--background/--foreground`, `--card/--card-foreground`, `--popover/…`, `--primary/…`,
`--secondary/…`, `--muted/…`, `--accent/…`, `--destructive`, `--border`, `--input`, `--ring`,
`--chart-1…5`, `--sidebar…`, `--radius`. `ui.css` deriva `--radius-sm/md/lg/xl`. O modo
escuro é a classe `dark` no `<html>` (o script de `ui.Head` aplica a preferência salva ou a
do sistema antes da primeira pintura).

## CLI

`trilha ui [--force] [--css-only|--js-only]` grava os três arquivos em `public/`:
`ui.theme.css` só é criado (nunca sobrescrito); `ui.css` e `ui.js` são atualizados quando
iguais a uma versão anterior e, se você os editou, só com `--force`.
