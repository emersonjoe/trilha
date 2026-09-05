---
title: Interface com ui
description: O kit de componentes padrão do Trilha, compatível com temas do shadcn/ui, e como ele fica seu para customizar.
---

Todo projeto criado com `trilha new` já vem com o kit `ui`: componentes tipados em Go
(`ui.Button`, `ui.Card`, `ui.Field`...) que renderizam classes de um CSS pequeno e
prefixado (`ui-*`), mais um JavaScript de 200 linhas para o que o HTML não faz sozinho
(abas, avisos que somem, campos condicionais, tema claro/escuro). Nenhuma dependência: os
três arquivos ficam em `public/` e são seus.

```text
public/ui.theme.css   ← as cores e o raio: edite ou cole um tema pronto
public/ui.css         ← os componentes; `trilha ui` atualiza
public/ui.js          ← comportamentos; `trilha ui` atualiza
```

O contrato de tema é o do [shadcn/ui](https://ui.shadcn.com) (MIT): as mesmas variáveis,
`--background`, `--primary`, `--radius`, em `oklch`. Gere um tema em ui.shadcn.com/themes
ou tweakcn.com, cole o bloco `:root { … } .dark { … }` em `ui.theme.css` e pronto: nada em Go
muda. O Trilha não usa React nem Tailwind; a compatibilidade é só do tema.

## Ligando o kit

O layout gerado já faz isso; num projeto existente, rode `trilha ui` e adicione:

```go
h.Head(…, ui.Head(c)),          // ui.theme.css, ui.css, tema salvo, ui.js
h.Body(ui.Body(),               // fonte e cores do tema
	ui.Header(ui.Brand("/", "Meu app"), ui.Nav(ui.NavLink("/", "Início", true)), ui.Spacer(), ui.ThemeToggle()),
	h.Main(ui.Container(children)),
	ui.Toaster(),               // onde os avisos aparecem
)
```

## Variantes são atributos

Um componente é uma função que devolve `h.Node`; variantes e tamanhos são atributos de
classe que você mistura com qualquer atributo do `h`, na ordem que quiser. O `h` funde os
`class` repetidos em um só.

@demo ui-botoes

## Formulários

`ui.Field` junta rótulo, controle, ajuda e erro com os `id`/`for` e o `aria-*` certos.
`ui.ShowWhen("campo", "valor")` mostra o grupo só enquanto o campo tem aquele valor e
**desabilita os controles escondidos**, para eles não irem no `POST`. Sem JavaScript, os
campos simplesmente aparecem todos.

@demo ui-formulario

Depois de um `POST`, renderize o erro no próprio campo (`ui.Error("Título obrigatório")` +
`ui.Invalid()` no controle) e um aviso que some sozinho: `ui.Toast("success", "Salvo!",
4000)` dentro do `ui.Toaster()` do layout. O exemplo `examples/blog` faz as duas coisas em
`app/blog/novo/page.go`.

## Cards, abas, progresso

@demo ui-card

## Diálogo e avisos

`ui.Dialog` é um `<dialog>` nativo: fecha com Esc, clique fora ou `ui.DialogClose`; o
formulário dentro dele faz `POST` normalmente.

@demo ui-dialogo

## Tabelas com hierarquia

`ui.Depth(n)` indenta a primeira célula: serve para plano de contas, árvore de categorias e
qualquer *drill-down* renderizado no servidor. `ui.Num()` alinha números à direita.

@demo ui-tabela

## Atualizar e customizar

- `trilha ui` regrava `ui.css` e `ui.js` quando você atualiza o Trilha; nunca toca em
  `ui.theme.css`. Se você editou `ui.css`, ele avisa e só sobrescreve com `--force`.
- Para mudar um componente, edite `ui.css` (ele é seu) ou sobreponha em `style.css`. Para
  um componente novo, escreva a função no seu pacote: `func Preco(v int) h.Node { return
  h.Span(h.Class("ui-badge preco"), …) }`.
- Ícones: `ui.Icon("check")`, um conjunto pequeno do [Lucide](https://lucide.dev) (ISC).
  `ui.Icons()` lista os nomes. Para outros, cole o SVG num `h.Raw` seu.

## Desafio

Faça um formulário de cadastro em que o campo "Empresa" só aparece quando "Tipo" é
"Jurídica" e, ao enviar sem preencher, o erro apareça no campo e um aviso some após 3 s.

:::solucao
```go
func Page(c *trilha.Ctx) (h.Node, error) {
	erro := c.Query("erro")
	return h.Form(h.Method("post"), h.Class("ui-stack"), trilha.CSRFInput(c),
		ui.Field("tipo", "Tipo", ui.Select(h.ID("tipo"), h.Name("tipo"),
			h.Option(h.Value("pf"), h.Text("Física")), h.Option(h.Value("pj"), h.Text("Jurídica")))),
		ui.Field("empresa", "Empresa", ui.Input(h.ID("empresa"), h.Name("empresa"), h.If(erro != "", ui.Invalid())),
			ui.Error(erro), ui.With(ui.ShowWhen("tipo", "pj"))),
		ui.Submit(h.Text("Cadastrar")),
		h.If(erro != "", ui.Toaster(ui.Toast("error", erro, 3000))),
	), nil
}

func POST(c *trilha.Ctx) error {
	if c.Form("tipo") == "pj" && strings.TrimSpace(c.Form("empresa")) == "" {
		return c.Redirect("/cadastro?erro=Empresa+obrigat%C3%B3ria")
	}
	return c.Redirect("/cadastro/ok")
}
```
:::
