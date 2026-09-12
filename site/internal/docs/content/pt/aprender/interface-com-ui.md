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

@demo ui-confirmar

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

## A moldura de um app interno

O `ui.Shell` é a barra lateral, o topo e o menu de quem está logado, escritos uma vez só. O
item cujo `Href` é o prefixo mais longo de `Current` ganha `aria-current="page"` — um match
exato sempre vence, então `/items/42/edit` acende `/items`, não `/`. O `ui.PageHeader` é o
título da tela dentro dele, com a volta e as ações da tela. Veja [Shell](/pt/referencia/shell)
para `Hide`, a barra que colapsa e o `IconNode`.

@demo ui-shell

## Onde você está, e quem está logado

O `ui.Breadcrumb` desenha a trilha com links de verdade, com a página atual num `<span
aria-current="page">` em vez de um link para ela mesma. O `ui.Avatar` cai para as iniciais
quando não há foto.

@demo ui-breadcrumb

Um menu que abre uma lista pequena de ações é o `ui.MenuTrigger` e o `ui.Menu` dividindo um
`id`, sobre o atributo `popover` do próprio navegador — sem script nenhum.

@demo ui-menu

## Mais conteúdo atrás de um clique

O `ui.Collapsible` é um `<details>` com estilo: nenhum script decide se está aberto, o
navegador já faz isso.

@demo ui-colapsavel

O `ui.Tabs` funciona igual onde quer que apareça — as setas e Home/End movem a seleção, a
primeira aba começa aberta:

@demo ui-abas

## Os blocos com que o layout é construído

`ui.Row`, `ui.Stack` e `ui.Grid` são `<div>`s com uma classe cada — linha, coluna, grade
responsiva — e `ui.Separator` é o traço entre seções de uma tela.

@demo ui-grade

## Antes do dado chegar

O `ui.Skeleton` é a forma do que está por vir; o `ui.Progress` é uma barra numa posição
conhecida. Nenhum dos dois precisa de script — o `ui.Defer`, mais adiante, é o que troca um
skeleton pelo conteúdo de verdade.

@demo ui-carregamento

## Uma tecla, e um trecho

`ui.Kbd` e `ui.Code` ficam em linha com a frase ao redor.

@demo ui-tipografia

## Um valor do enum, como emblema

`ui.Status(enum, valor)` lê o rótulo e o tom que um `trilha.Enum` declarou uma vez só — a
mesma declaração que as opções de um `<select>` e a validação de um formulário já usam. Um
valor que o enum não conhece mais renderiza discreto, não em branco.

@demo ui-status

## Nada para mostrar, e o que não deu para carregar

O `ui.Empty` é a tela sem nada nela: ícone, título, uma dica do que fazer a seguir e uma
saída. O `ui.EmptyError` é a mesma forma para uma tela que falhou ao carregar — o erro em si
só aparece em desenvolvimento, nunca na tela de quem está visitando.

@demo ui-vazio

## Um formulário em várias telas

O `ui.Steps` desenha onde alguém está: o que ficou para trás linka de volta, o que está à
frente é texto simples, e a etapa atual carrega `aria-current="step"`. O estado entre as
telas é o `Ctx.Draft` — veja [o formulário em etapas](/pt/receitas/formulario-em-passos).

@demo ui-etapas

## A parte lenta, um instante depois

Um painel que precisa de sete consultas não deveria segurar a página inteira pela que demora
dois segundos. O `ui.Defer` desenha um placeholder agora e pede o fragmento assim que a
página carregou; veja [Fragmentos vivos](/pt/referencia/vivo) para a rota que ele espera do
outro lado.

@demo ui-atraso

## Um arquivo ao lado dos seus metadados

O `ui.Preview` mostra uma imagem como `<img>` que abre em tamanho cheio, um documento que o
navegador desenha embutido, ou — quando o tipo não dá para mostrar no lugar — um cartão com
botão de download em vez de um quadro em branco.

@demo ui-preview

## O que acontece durante uma troca

O `ui.Indicator(id)` marca qualquer coisa — um emblema, um spinner — para aparecer só
enquanto o alvo daquele `id` estiver esperando além do limite do `ui.PendingAfter` (120 ms
por padrão), e o `ui.NoTransition()` desliga o esmaecimento cruzado de um gatilho que dispara
com frequência, como uma busca ao vivo.

@demo ui-espera

## Tabelas que vivem na URL

O `ui.DataTable` é a tela que todo app de gestão tem: filtro em cima, tabela no meio,
paginação no rodapé, e todo o estado — página, ordem, busca — no endereço. Veja
[Listagens](/pt/referencia/listagens) para o `trilha.ListParams`, a peça que o lê de volta.

@demo ui-listagem

## Vazia, e filtrada até vazia

Uma lista sem nada dentro e uma lista que uma busca não encontrou nada são duas telas
diferentes; o `ui.DataTable` já faz essa distinção sozinho, em inglês, e o `ListState.Empty` é
como uma aplicação diz isso no seu próprio idioma.

@demo ui-listagem-vazia

## Uma hierarquia que abre nó a nó

O `ui.Tree` é um plano de classificação, uma árvore de pastas, um organograma: cada nó é um
`<details>`, então abre sem script nenhum. O `ui.TreePicker` é a mesma árvore como campo de
formulário — veja [Árvores](/pt/referencia/ui#arvores).

@demo ui-arvore

## Um campo que busca enquanto você digita

O `ui.Combobox` é um campo de texto que o servidor busca — uma classificação entre centenas,
uma cidade entre milhares — sem viagem ao servidor para uma lista curta o bastante para
filtrar no navegador. Veja [Combobox](/pt/referencia/ui#combobox).

@demo ui-combobox

## Uma caixa, vários tipos de coisa

O `ui.SearchBox` e o `ui.SearchResults` são a busca da barra de cima, sobre um
`trilha.Search` — um índice, vários tipos, resultado agrupado e contado. Veja
[Search](/pt/referencia/search).

@demo ui-busca

## A mesma tela, dois idiomas

O `ui.Date`, o `ui.Bytes`, o `ui.Duration` e o `ui.Number` leem o `Config.Locale` do `Ctx` — a
palavra muda, a chamada não. Veja [Formatação](/pt/referencia/ui#formatacao).

@demo ui-locale

## Quatro números e os desenhos ao lado

O `ui.Stat`, o `ui.Bars`, o `ui.Sparkline` e o `ui.Donut` desenham um painel no servidor, em
SVG, sem biblioteca de gráfico. Veja [Gráficos](/pt/referencia/graficos).

@demo ui-indicadores

## Uma célula que se atualiza sozinha

O `ui.Poll` pergunta de novo pela rota de um fragmento, no relógio; o `ui.Live` e o `ui.On`
fazem isso quando o servidor avisa, em vez disso. Veja
[Fragmentos vivos](/pt/referencia/vivo).

@demo ui-ao-vivo

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

## O que vem depois: as telas prontas

O catálogo acima são as peças. [Telas prontas](/pt/aprender/telas-prontas) é uma tela montada
com elas, de ponta a ponta — a moldura, os números, a tabela que vive na URL, a parte lenta, o
arquivo, o formulário em etapas —, com o motivo de cada peça estar onde está.
