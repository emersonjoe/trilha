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
	ui.Flashes(c),              // onde os avisos aparecem, o c.Flash junto
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
4000)` dentro do toaster do layout. O exemplo `examples/blog` faz as duas coisas em
`app/blog/novo/page.go`.

## Contar o que aconteceu, e perguntar antes de destruir

Um `POST` que deu certo termina em redirect, e o redirect come a notícia. O `c.Flash`
escreve num cookie assinado, e o `ui.Flashes(c)` do layout mostra na página seguinte:

```go
c.Flash(ui.FlashSuccess, "Post apagado")
return c.Redirect("/blog")
```

Os tipos são `ui.FlashInfo`, `ui.FlashSuccess` e `ui.FlashError`. Numa resposta de fragmento
não há redirect para sobreviver: os avisos vão num cabeçalho e quem mostra é o `ui.js` — a
chamada no handler é a mesma. Sem `TRILHA_SECRET` nada é escrito, e o app avisa uma vez no
log.

Antes de algo irreversível, o `ui.Confirm` põe a pergunta no próprio formulário:

```go
h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
	ui.Confirm("Apagar este post?", "Não dá para desfazer."),
	h.Data("ui-confirm-cancel", "Cancelar"),
	ui.Submit(ui.Destructive(), h.Text("Apagar")))
```

O `ui.js` segura o envio, abre o diálogo do kit e só então deixa passar. Sem JavaScript o
formulário envia direto; quando isso não serve, pergunte numa página própria (`GET
/blog/{slug}/apagar` renderizando o mesmo formulário), que funciona dos dois jeitos.

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

## Paginação e dicas

`ui.Pagination` desenha a navegação de páginas com links de verdade, então uma página pode ser
compartilhada, recarregada e indexada. A página atual é um `<span>` com `aria-current` — link
para onde você já está é link para lugar nenhum — e a primeira página não tem *anterior*, então
nada é desenhado no lugar. A janela guarda a primeira página, a última e as vizinhas da atual,
com reticências sobre cada buraco, para o rodapé não crescer junto com a tabela.

`ui.Tooltip` escreve a dica no `title`, que é o tooltip do próprio navegador e funciona com o
`ui.js` desligado. Com o script na página o `title` some — dois tooltips é pior que nenhum —,
uma bolha com `role="tooltip"` toma o lugar dele, o alvo ganha `aria-describedby` e a dica
responde ao mouse, ao foco do teclado e ao toque, fechando com Escape.

@demo ui-paginacao

:::nota
A dica é uma string de propósito. Dica com link dentro é *popover*, e para isso existe o
`ui.Menu`.
:::

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
