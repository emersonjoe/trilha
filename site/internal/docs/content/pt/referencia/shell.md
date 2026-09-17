---
title: Shell
description: ui.Shell é a moldura de um app interno — barra lateral, cabeçalho, menu do usuário — e ui.PageHeader é o título da tela dentro dela.
---

Todo app de gestão tem a mesma moldura: uma barra lateral com as seções, um cabeçalho com
o usuário e a tela no meio. `ui.Shell` é essa moldura escrita uma vez. É composição de
componentes que já existiam — `ui.Sidebar`, `ui.Nav`, `ui.Menu`, `ui.ThemeToggle` —, não
uma primitiva nova: o que ela não cobre você escreve ao lado, com as mesmas peças.

## Shell

```go
func Layout(c *trilha.Ctx, children ...h.Node) h.Node {
	u := session.User(c)
	return ui.Shell(c, ui.ShellOpts{
		Brand: h.A(h.Class("ui-brand"), h.Href("/"), h.Text("Acme")),
		Nav: []ui.NavGroup{
			{Label: "Trabalho", Items: []ui.NavItem{
				{Href: "/", Label: "Painel", Icon: "house"},
				{Href: "/itens", Label: "Itens", Icon: "search", Badge: "12"},
			}},
			{Label: "Administração", Hide: u.Role != "admin", Items: []ui.NavItem{
				{Href: "/usuarios", Label: "Usuários", Icon: "user"},
			}},
		},
		User:   ui.UserMenu{Name: u.Name, Detail: u.Email, Items: []h.Node{formDeSair(c)}},
		Header: []h.Node{ui.ThemeToggle()},
	}, children...)
}
```

`ShellOpts`:

| Campo | O que é |
|---|---|
| `Brand` | o que fica no topo da barra lateral; em geral um link para `/` |
| `Nav` | os grupos do menu, na ordem em que aparecem |
| `User` | o nome, a linha de detalhe e os itens do menu do canto superior direito |
| `Header` | nós extras do cabeçalho, à esquerda do menu do usuário |
| `Current` | o caminho a marcar como ativo; o caminho do request quando vazio |

Um `NavItem` é `{Href, Label, Icon, Badge, IconNode, Hide}`. `Icon` é um nome de
`ui.Icon`; `Badge` é uma contagem curta ao lado do rótulo. `IconNode`, quando presente,
vence `Icon`: é a saída para um nome que os 31 ícones do kit não cobrem — um documento,
um prédio — e o app desenha o próprio nó em vez de o kit entrar em pânico com um nome
desconhecido:

```go
{Href: "/contratos", Label: "Contratos", IconNode: h.Svg(h.Class("ui-icon"),
	h.Attr("viewBox", "0 0 24 24"), h.El("path", h.Attr("d", "…")))}
```

A classe `ui-icon` é o que mantém o kit dono do tamanho e do alinhamento; o app só
desenha o que vai dentro dela.

## O item ativo é o prefixo mais longo

O item marcado com `aria-current="page"` é aquele cujo `Href` é o prefixo mais longo do
caminho atual: `/itens/42/editar` acende `/itens`, não `/`. Igualdade exata sempre ganha,
e `/` só casa com `/` — senão o painel ficaria ativo em toda tela.

## Esconder é enfeite

`Hide: true` deixa o item (ou o grupo inteiro) fora do HTML. Isso é gentileza com quem lê
o menu, **não** é permissão. O que de fato mantém alguém do lado de fora é um middleware
na raiz da pasta:

```go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return session.Flow.RequireRole("admin")(c, next)
}
```

Link escondido com rota desprotegida é rota que qualquer um digita.

## Recolher é preferência; a gaveta não é

No desktop o botão do cabeçalho carimba `ui-sidebar-collapsed` no `<html>`, a página toma a
largura inteira e a escolha fica no `localStorage`. O script inline que o `ui.Head` já emite
lê isso antes da primeira pintura, então a barra não abre e fecha a cada navegação.

Abaixo de 768px a barra é uma gaveta: fechada por padrão, aberta pelo mesmo botão, sobre um
véu, com um botão de fechar próprio; toque no véu, num link do menu ou `Escape` fecha. A
gaveta é `ui-drawer-open`, uma classe que ninguém guarda — um celular que abre o menu não
está pedindo o menu aberto em toda página daqui em diante, e nunca mexe na preferência do
desktop. O código está no `ui.js`, que o shell já precisa — não há nada novo para baixar.

## PageHeader

Dentro do shell, cada tela abre com o próprio título e os próprios botões:

```go
ui.PageHeader("Itens",
	ui.Back{Href: "/", Label: "Painel"},
	ui.ButtonLink("/itens/novo", ui.Icon("plus"), h.Text("Novo item")),
)
```

`ui.Back` é lido pelo `PageHeader` e desenhado como o caminho de volta, acima do título;
`ui.Subtitle("…")` é a linha abaixo dele, na cor apagada; todo outro filho vai à direita do
título, que é o lugar das ações da tela.

```go
ui.PageHeader("Contrato 41", ui.Subtitle("Assinado em 3 de maio, vence em 12 meses"))
```

## Começar daqui

`trilha new meu-app --template app` grava um projeto que já tem tudo isso: o shell, um
login, um painel com gráficos, uma listagem e um formulário. Veja
[a CLI](/pt/referencia/cli).
