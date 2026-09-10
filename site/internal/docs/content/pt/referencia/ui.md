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
| `ui.Asset(nome) []byte` | conteúdo embutido de `ui.css`, `ui.theme.css`, `ui.js`, `ui.nav.js`, `ui.upload.js`, `ui.live.js`, `ui.chat.js` ou `ui.island.js` |
| `ui.Files` | os seis nomes, na ordem em que `trilha ui` os grava |

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
| `Errors(errs, campo)` | opção de `Field`: mostra a mensagem de `errs[campo]` (um `trilha.FieldErrors`) se houver |
| `InvalidIf(errs, campo)` | `Invalid()` só quando há erro para o campo |
| `SelectOptions([]Option{{Value, Label}}, selecionado)` | `<option>`s marcando o selecionado; `Value: ""` é placeholder (desabilitado) e fica selecionado quando nada casa |
| `Checked(bool)` | `checked` condicional (ida e volta de checkbox/switch/radio) |
| `ShowWhen(campo, valores...)` | `data-ui-show-when`: mostra o elemento só com o valor (ou qualquer valor não vazio); controles escondidos são desabilitados |
| `Combobox(ComboboxOpts{...}, attrs...)`, `ComboboxOptions(itens, de)` | campo de texto que busca numa lista — veja [Combobox](#combobox) |
| `Dropzone(DropzoneOpts{...}, filhos...)` | área de arrastar-e-soltar em cima de um campo de arquivo — veja [Upload com progresso](#upload-com-progresso) |
| `SchemaForm(esquema, values, errs, ...)` | formulário definido por dado: um campo por `trilha.SchemaField` — veja [Validação](/pt/referencia/validacao) |
| `Badge`, `Alert(título, ...)`, `AlertDescription(...)` | selo e aviso (`role=alert`) |
| `Toaster(...)`, `Toast(tipo, texto, fadeMs)` | pilha de avisos; `tipo` = `""`, `success`, `error`; `fadeMs > 0` some sozinho |
| `Flashes(c)` | o toaster com os avisos do [`c.Flash`](/pt/referencia/ctx) — ponha no layout; `FlashInfo`, `FlashSuccess` e `FlashError` são os tipos |
| `Table(...)`, `Num()`, `Depth(n)` | tabela rolável; célula numérica; indentação de linha (árvore) |
| `Tabs(id, Tab{Label, Content}...)` | abas acessíveis (setas, Home/End); a primeira começa aberta |
| `Dialog(id, título, ...)`, `DialogDescription(s)`, `DialogFooter(...)`, `DialogTrigger(id, ...)`, `DialogClose(...)` | `<dialog>` nativo com `showModal` |
| `Confirm(título, descrição)` | atributos para um `<form>`: o `ui.js` pergunta num diálogo antes de enviar, inclusive em formulário de fragmento. O botão que confirma repete o rótulo do botão apertado; o outro diz `Cancel`, ou o que estiver em `h.Data("ui-confirm-cancel", "…")`. Sem JavaScript o formulário envia direto |
| `Menu(id, ...)`, `MenuItem(...)`, `MenuLink(href, ...)`, `MenuTrigger(id, ...)` | menu com o atributo `popover` nativo |
| `Pagination(Pages{Page, Total, Href, Prev, Next, Label, Attrs})` | navegação de páginas em links; a página atual é um `<span>` com `aria-current`, as pontas somem em vez de virarem link desabilitado, e uma janela de sete casas guarda a primeira e a última página com `…` sobre cada buraco; uma página só não desenha nada |
| `Tooltip(texto, ...)` | dica no que ele embrulha: `title` mais `data-ui-tooltip`, promovido pelo `ui.js` a uma bolha com `role=tooltip` e `aria-describedby` |
| `Separator, Skeleton, Progress(valor, máx), Breadcrumb(Crumb{Label, Href}...), Avatar(iniciais, src), Collapsible(resumo, ...)` | diversos |
| `ThemeToggle()` | botão que alterna claro/escuro (`localStorage["ui-theme"]`) |
| `CSVErrors(c, res, CSVErrorsOpts{...})` | o que o `trilha.BindCSV` recusou, por linha e coluna — veja [Planilhas (CSV)](/pt/receitas/planilhas) |
| `DataTable(c, Columns[T], linhas, ListState)` | a listagem: formulário de filtro, cabeçalho ordenável, paginação e estado vazio, tudo na URL — veja [Listagens](/pt/referencia/listagens) |
| `Swap(id)` | `data-trilha-target`: o `<a>` ou `<form>` pede só o elemento `#id` e troca (fragmentos) |
| `SecretOnce(c, segredo)`, `APIKeysTable(c, linhas, opts)` | a chave mostrada uma vez, e a lista delas — veja [Auth](/pt/referencia/auth) |
| `SettingsForm(c, seção, errs)` | a tela de administração de uma seção do `trilha.Settings`, desenhada da struct — veja [App](/pt/referencia/app) |
| `Tree(TreeOpts{...})`, `TreePicker(TreePickerOpts{...})`, `TreeItems`, `TreeScript(c)` | a hierarquia que abre nó a nó, e o campo que escolhe um — veja [Árvores](#árvores) |
| `AuditTable(c, registros, AuditOpts{...})` | a trilha que o c.Audit escreve, com filtro, paginação e exportação CSV — veja [Observabilidade](/pt/referencia/observabilidade) |
| `Steps([]Step{Label, Href}, atual)` | o indicador de um formulário em várias telas — veja [Formulário em passos](/pt/receitas/formulario-em-passos) |
| `Preview(c, src, PreviewOpts{...})` | o arquivo ao lado do que se sabe dele: barra, quadro, imagem ou "não dá para pré-visualizar" — veja [Ctx](/pt/referencia/ctx) e [Uploads](/pt/receitas/uploads) |
| `Defer(c, id, src, DeferOpts{...})` | serve a página agora e preenche esta parte um instante depois — veja [Fragmentos vivos](/pt/referencia/vivo) |
| `Poll(intervalo, src)`, `Live(src)`, `On(evento, src)`, `LiveScript(c)` | fragmento que se atualiza pelo relógio ou por um evento do servidor — veja [Fragmentos vivos](/pt/referencia/vivo) |
| `NoPush()` | `data-trilha-push="false"`: a troca não mexe no histórico |
| `Markdown(texto, MarkdownOpts{...})` | texto de modelo ou de visitante como HTML, escapado por construção — veja [Markdown](#markdown) |
| `Chat(c, ChatOpts{...})`, `ChatScript(c)`, `ChatHTML(texto)` | uma conversa com um agente — veja [Chat](#chat) |
| `Icon(nome, attrs...)`, `Icons()` | SVG inline do Lucide; nome desconhecido → pânico (erro de programação) |
| `APIUsage(c, dados, opts)` | quanto uma chave foi usada, onde, e quando parou — veja [Auth](/pt/referencia/auth) |
| `SearchBox(c, action, opts)`, `SearchResults(c, res, opts)` | a caixa da barra de cima e o resultado agrupado de um `trilha.Search` — veja [Search](/pt/referencia/search) |
| `DeadlineCards(c, resumo)`, `DeadlineList(c, itens, opts)`, `DeadlineBadge(c, vencidos)` | o que vence e quando, a partir de um resumo do `trilha.Deadlines` — veja [DeadlineCards](#deadlinecards) |

## Árvores

Hierarquia com milhares de nós é o componente que as pessoas vão buscar no npm: expandir, buscar
e teclado são cada um fácil e juntos são trezentas linhas. O `ui.Tree` é a versão disso no
servidor — o servidor já conhece a árvore, então o navegador nunca precisa conhecer.

```go
ui.Tree(ui.TreeOpts{
	Nodes:   raizes,                  // com o caminho até o Current já dentro
	Source:  "/classificacao/nos",    // GET ?parent=100.1 responde os filhos
	Current: doc.Codigo,
	Label:   "Plano de classificação",
})
```

| Símbolo | Papel |
|---|---|
| `Tree(TreeOpts{...})` | a hierarquia; cada nó é um `<details>`, então abre sem script nenhum |
| `TreeNode{Value, Label, Leaf, Href, Children, Open, Path}` | um nó; os `Children` viajam junto quando já são conhecidos |
| `TreeItems(nos, TreeOpts{...})` / `TreeNodes(itens, of)` | o que uma rota-fonte responde: os filhos de um nó, em HTML |
| `TreePicker(TreePickerOpts{...})` | a mesma árvore como campo de formulário: **um radio por nó** |
| `TreeScript(c)` | carrega o `ui.tree.js`; página sem árvore não baixa nada disso |

**Um nó é `<details>`, e é essa a história inteira do sem-JavaScript.** O que o script
acrescenta é buscar os filhos na primeira vez que um ramo abre, em vez de pedir uma página
inteira ao servidor. Nó cujos `Children` já estão em `Nodes` não pede nada — é assim que o
caminho até o nó atual chega aberto e completo no primeiro desenho, inclusive depois de um 422
trazer o formulário de volta.

Os papéis são os de verdade (`tree`, `treeitem`, `group`, `aria-expanded`), as setas andam pelo
que está visível, `Home` e `End` vão às pontas, e `*` expande tudo. Só o primeiro nó entra na
ordem de tabulação: a árvore é uma parada só, e as setas andam dentro dela.

### O seletor

```go
ui.Field("codigo", "Classificação", ui.TreePicker(ui.TreePickerOpts{
	Name:   "codigo",
	Value:  form.Codigo,
	Nodes:  plano.Raizes(form.Codigo),
	Source: "/classificacao/nos",
	Search: "/classificacao/busca", // GET ?q= responde nós achatados, cada um com o seu Path
}))
```

**O que posta é um radio**, e é essa a razão inteira de isto funcionar sem script: a pessoa
navega pelos mesmos `<details>` e marca o mesmo radio, e o formulário manda o mesmo campo. Não
há hidden para manter em sincronia nem texto para resolver no servidor.

Com o script, digitar pergunta ao `Search` e põe os achados no lugar da árvore, cada um com a
ancestralidade de onde veio — um código achado fora de contexto não diz onde mora. Apagar a busca
traz a árvore de volta da memória, sem pedir de novo.

Uma árvore de radios se anuncia como **grupo de escolhas**, não como navegação: é campo de
formulário, e isso é um papel e não dois.

:::warning
O valor que chega ao servidor não veio da árvore — veio de uma requisição. Confira contra a
hierarquia (`validate:"required,..."` mais uma regra sua, como o
[`examples/cadastro`](https://github.com/emersonjoe/trilha/tree/main/examples/cadastro) faz): o
radio é o que uma pessoa usa, não o que limita quem ataca.
:::

## ui.js

Tudo por atributo, sem inicialização: `[data-ui-tabs]`, `[data-ui-dialog-open=id]`,
`[data-ui-dialog-close]`, `[data-ui-fade=ms]`, `[data-ui-show-when]`, `[data-ui-toast=texto]`
(`data-ui-toast-kind`), `[data-ui-theme-toggle]`, `[data-ui-tooltip=texto]`, `[popover].ui-menu`. Também expõe
`window.ui.toast(texto, {kind, ms})`, `ui.fade(el)`, `ui.evalShowWhen(root)` e
`ui.applyTheme("dark"|"light")`. Elementos inseridos depois (HTMX, fetch) precisam de
`ui.evalShowWhen(el)`/`ui.fade(el)`/`ui.initTooltips(el)` se usarem esses atributos —
`ui.hydrate(el)` faz os três de uma vez.

## Fragmentos

`[data-trilha-target=id]` em `<a>` ou `<form>` (veja `ui.Swap`) faz o kit pedir a mesma URL
com o cabeçalho `Trilha-Fragment` e trocar o elemento `#id` pelo HTML que voltou. Detalhes:
o alvo ganha `aria-busy` durante a espera; **204 com `Trilha-Location`** vira navegação de
verdade; **422** põe o foco no primeiro `[aria-invalid=true]`, senão o foco (e o cursor)
voltam para o campo em uso; o que entrou é hidratado (`fade`, `show-when`) e dispara
`trilha:swap` (`detail.target`, `detail.status`). Em 5xx, erro de rede ou fragmento sem o
id, o kit desiste e navega/envia normalmente. `ui.swap(id, html, status)` e
`ui.hydrate(el)` fazem a troca à mão (o `ui.swap` devolve uma promessa: a substituição pode
estar rodando dentro de uma transição de visualização). Veja
[Interatividade](/pt/aprender/interatividade).

### Espera

| Símbolo | O que faz |
|---|---|
| `ui.Indicator(id)` | este elemento só aparece enquanto o alvo `id` espera além do limiar |
| `ui.PendingAfter(ms)` | o limiar, no gatilho; padrão 120 ms, zero ou menos significa o padrão |
| `ui.NoTransition()` | sem *crossfade* neste gatilho |
| `ui.Spinner(attrs…)` | um anel girando do tamanho da fonte em que está, escondido da tecnologia assistiva |

Enquanto um alvo espera, o `data-trilha-pending` está no alvo, no gatilho e em todo indicador
daquele alvo, e o `aria-busy` está no alvo; `trilha:pending` e `trilha:settled` disparam no
`document` com `detail.target` e `detail.id`. Um segundo gatilho para um alvo que já está no
ar é ignorado. A substituição roda dentro do `document.startViewTransition` onde ele existe e
onde o sistema não pede menos movimento.

## Navegação

A navegação no cliente fica desligada até você pedir, em dois lugares:

| Símbolo | Papel |
|---|---|
| `ui.Navigate(id) h.Node` | marca uma região: um clique em link da mesma origem dentro dela troca o elemento `#id` pelo mesmo elemento da próxima página. `id` vazio significa o próprio elemento marcado |
| `ui.NoNavigate() h.Node` | deixa um link de fora (um download, outro app, uma rota que precisa recarregar) |
| `ui.NavigateScript(c) h.Node` | `<script defer src=ui.nav.js>`; ponha uma vez, no layout da área que usa |

O que o navegador continua fazendo: o endereço na barra é o mesmo de uma navegação normal,
Voltar e Avançar funcionam (e restauram a rolagem da entrada para onde voltam),
`Cmd`/`Ctrl`-clique e clique do meio abrem aba, e `target`, `download` e links para outra
origem passam intactos. O que o kit acrescenta: `aria-busy` na região durante a espera, foco
no que entrou, `ui.hydrate` e o evento `trilha:swap`, e uma requisição por vez — um segundo
clique cancela a primeira. Em 5xx, erro de rede, redirecionamento ou página sem o id, ele
desiste e navega de verdade.

O comportamento é um arquivo separado para que um app que não use não o baixe, e o `ui.Head`
não o carrega. Link marcado com `ui.Swap` continua sendo fragmento: ele pede um pedaço da
página, não a próxima página.

## Upload com progresso

Um formulário que manda arquivo é um formulário: `method="post"`,
`enctype="multipart/form-data"`, o campo de CSRF. Três símbolos põem a barra de progresso em
cima disso, e ela fica desligada até você pedir:

| Símbolo | Papel |
|---|---|
| `ui.UploadTo(id) h.Node` | no `<form>`: envia por XHR e troca o `#id` pelo que voltar |
| `ui.UploadBar(attrs…) h.Node` | o `<progress>` que o kit preenche; escondido até o envio começar |
| `ui.UploadScript(c) h.Node` | `<script defer src=ui.upload.js>`, uma vez por página que envia |

A requisição leva `Trilha-Fragment: id`, então o handler responde o pedaço com o mesmo
`c.Fragment()` de sempre. Enquanto sobe, a barra recebe `value`/`max` do evento de progresso
do próprio navegador (e perde o `value` — barra indeterminada — quando o total é
desconhecido), e um evento `trilha:upload` sobe com `detail: {loaded, total, form}`. Em 5xx,
erro de rede ou pedaço sem o id, o formulário envia de verdade: o usuário vê a página
recarregar, não um botão que não fez nada.

O atributo é `data-trilha-upload`, e não `data-trilha-target`, para o tratador de fragmento
do `ui.js` não enviar o mesmo formulário uma segunda vez. O limite de corpo é assunto do
servidor — veja [`AllowBody`](/pt/referencia/ctx).

O `ui.Dropzone(ui.DropzoneOpts{Name, Accept, MaxSize, Single, Attrs})` põe uma área de soltar
em cima do campo de arquivo: um `<label>` que recebe o arrasto e um `<ul class="ui-queue">`
com uma linha por arquivo. Com `ui.UploadTo` no formulário, a fila manda **um arquivo por
requisição**, então cada linha tem o próprio progresso e a própria resposta — uma mensagem que
diz `arquivos[2]` não tem onde pousar quando três arquivos viajam num corpo só. O elemento
trocado precisa estar fora do dropzone, ou a fila é destruída no meio do caminho.

O `Accept` e o `MaxSize` das opções só poupam uma ida ao servidor: dá para dizer qualquer coisa
ao navegador. Quem decide é o [`c.Files`](/pt/referencia/ctx) com as `FileRules` — as mesmas
regras, aplicadas a bytes que já chegaram. Sem JavaScript o campo é um `multiple` comum e o
formulário posta todos os arquivos de uma vez, no mesmo handler.

## Combobox

Um campo de texto que busca numa lista são dois campos: o que a pessoa digita e o que o
formulário manda. O `ui.Combobox` renderiza os dois — um `<input role=combobox
name="<nome>_q">` visível e um `<input type=hidden name="<nome>">` com o valor escolhido —
mais o `<ul role=listbox>` das opções.

| Campo do `ComboboxOpts` | Papel |
|---|---|
| `Name` | nome do campo escondido; o visível é `Name + "_q"` |
| `Value` / `Label` | o que foi escolhido e o que está escrito por ele (a ida e volta) |
| `Options []Option` | lista curta: o `ui.js` filtra no navegador, sem requisição |
| `Source string` | a URL que busca; a resposta é um `ui.ComboboxOptions(...)` |
| `With []string` | outros campos do mesmo formulário para levar na query (`?uf=SP&q=camp`) |
| `MinChars`, `Debounce` | quando buscar (padrão 1 caractere, 200 ms) |
| `Placeholder`, `Required`, `Attrs` | como no `Input` |

O `ComboboxOptions(itens []T, de func(T) (valor, rótulo string))` é a resposta da rota de
busca: só os `<li>`, sem envelope. A requisição leva `Trilha-Fragment`, então a rota responde
com `c.HTML` e o layout da página fica de fora.

```go
func GET(c *trilha.Ctx) error {
	return c.HTML(200, ui.ComboboxOptions(
		Buscar(c.Query("uf"), c.Query("q")),
		func(cidade string) (string, string) { return cidade, cidade },
	))
}
```

Sem JavaScript nada quebra: o campo visível é um texto comum, então o formulário chega com
`cidade_q` preenchido e `cidade` vazio. Resolver o texto digitado contra a lista é trabalho do
servidor — a mesma lista que a rota de busca lê.

## Markdown

```go
func Markdown(src string, opt MarkdownOpts) h.Node
```

Modelo escreve Markdown, e quem digita num formulário também. O `Markdown` transforma isso em
nós: parágrafos, ênfase, títulos, listas, citação, código inline e cercado, tabelas GFM, links e
quebras.

```go
h.Div(ui.Markdown(doc.Resumo, ui.MarkdownOpts{}))
```

**Não existe HTML cru, e não há como ligar.** Um `<script>` no texto é um `<script>` na tela,
como texto — o retorno é uma árvore, não uma string, então o escape não é uma regra que alguém
precise lembrar. É por isso que isto existe em vez de um `h.Raw` em volta de um conversor.

| Campo de `MarkdownOpts` | O que faz |
|---|---|
| `HeadingBase` | o nível em que `#` cai (padrão 3, para não competir com o `<h1>` da página); `######` nunca passa de `<h6>` |
| `Images` | `![alt](url)` vira `<img>`; desligado por padrão, e a URL é validada nos dois casos |
| `Class` | uma classe a mais no envelope, que é sempre `ui-md` |

Link só continua link quando o endereço é `http`, `https`, `mailto` ou relativo (`/`, `#`,
`./`); o resto — `javascript:`, `data:` — fica como o texto que era. Link externo leva
`rel="noopener nofollow ugc"`.

As páginas do próprio site usam outro conversor (`site/internal/md`): outro dialeto, sobre texto
deste repositório. O `ui.Markdown` é para texto que ninguém daqui escreveu.

## Chat

```go
func Chat(c *trilha.Ctx, o ChatOpts) h.Node
func ChatScript(c *trilha.Ctx) h.Node
func ChatHTML(texto string) string
```

A conversa com um agente: as bolhas, o campo e o botão. A rota do outro lado é o
[`ai.Serve`](/pt/referencia/ai#chat-por-http).

```go
ui.Chat(c, ui.ChatOpts{Action: "/api/chat", History: msgs, Greeting: "Pergunte o que quiser."})
ui.ChatScript(c)   // uma vez, no layout
```

| Campo de `ChatOpts` | O que faz |
|---|---|
| `Action` | a rota que responde — o único que precisa ser preenchido |
| `History` | o que já foi dito, do mais antigo para o mais novo. É do app: o framework não guarda sessão |
| `Greeting` | Markdown mostrado enquanto o histórico está vazio |
| `ID` | o id do elemento e o prefixo dos ids de dentro (padrão `chat`) |
| `Placeholder`, `Submit`, `Label` | as palavras na tela |
| `MaxLength` | limita o campo (padrão 4000; negativo tira o limite) |
| `Steps` | mostra a ferramenta que o agente chamou e o que voltou |
| `Markdown` | o `MarkdownOpts` das respostas |

`ChatMessage{Role, Text}` é um turno. `Role: "assistant"` sai como Markdown; `Role: "user"` sai
como texto — o que alguém digitou nunca é marcação.

Com o `ChatScript` a resposta chega palavra por palavra e o Markdown é renderizado no fim da
mensagem: passe `ui.ChatHTML` para o `ai.ServeOpts.HTML` e a bolha pronta fica igual à de uma
página recarregada. Sem o script o formulário submete do mesmo jeito e a rota responde tudo de
uma vez, então nada na tela depende do script rodar.

| Campo de `ChatOpts` | O que faz |
|---|---|
| `Context` | o que a página sabe e o modelo não: `{"folha": id}` vira campos escondidos `ctx.*`, mandados junto com a mensagem e lidos pelo `ai.ServeOpts.Context` |

### Assistant

```go
func Assistant(c *trilha.Ctx, o AssistantOpts) h.Node
```

O outro formato de um chat: um botão fixo no canto que abre um painel sobre a tela, em vez de
trocá-la. Monte uma vez, no layout da área que o tem.

```go
ui.Assistant(c, ui.AssistantOpts{
	Action: "/api/assistente",
	Page:   "/painel/assistente",          // a mesma conversa, como página
	Hint:   "Perguntando sobre a folha " + id, // o que ele sabe agora
	Chat:   ui.ChatOpts{Context: map[string]string{"folha": id}},
})
ui.ChatScript(c)
```

| Campo de `AssistantOpts` | O que faz |
|---|---|
| `Action` | a rota que responde, a do `ai.Serve` |
| `Page` | a conversa como página: para onde o launcher aponta quando não há script |
| `Label`, `Title`, `Hint` | a palavra no botão, o título do painel e a linha embaixo dele |
| `Icon` | um ícone do kit no botão; vazio é só o rótulo |
| `ID` | o prefixo de todos os ids de dentro (padrão `assistant`) — dois assistentes precisam de dois |
| `Chat` | a conversa em si: histórico, saudação, contexto |

O launcher é um link antes de ser um botão. **Sem JavaScript ele vai para o `Page`**, que é a
mesma conversa como página inteira; com JavaScript o script do kit abre o `<dialog>` no lugar, e
o foco preso e o Escape vêm do navegador. O `aria-expanded` acompanha o painel, o `aria-controls`
o nomeia, e tudo é HTML do servidor — uma página com assistente não vira ilha.

É composição, não um segundo chat: o painel tem um `Chat` dentro, então o streaming, o Markdown e
os erros são os que já existem.

### DeadlineCards

```go
func DeadlineCards(c *trilha.Ctx, s trilha.DeadlineSummary) h.Node
func DeadlineList(c *trilha.Ctx, items []trilha.Deadline, o DeadlineListOpts) h.Node
func DeadlineBadge(c *trilha.Ctx, items []trilha.Deadline) h.Node
```

O painel de um resumo de [`trilha.Deadlines`](/pt/referencia/app#prazos): os cartões em cima, a
lista embaixo, o número ao lado do item de menu.

```go
resumo := trilha.Deadlines(itens, trilha.DeadlineOpts{Now: time.Now().In(c.Location())})

ui.DeadlineCards(c, resumo)
ui.DeadlineList(c, itens, ui.DeadlineListOpts{Limit: 10, More: "/prazos"})
ui.DeadlineBadge(c, resumo.Overdue)
```

Os cartões seguem a ordem das faixas, então não trocam de lugar entre recargas, e o de vencidos só
fica vermelho quando há do que reclamar — um zero vermelho ensina a ignorar a cor, e aí o três
também é ignorado. O último cartão é o `Next`: uma contagem sem um exemplo ao lado é um número que
alguém precisa clicar para entender.

O `DeadlineListOpts` recebe `Limit` (com `More` para o link do "e mais 12"), `Owner` para a coluna
de responsável, `Empty` para a frase de quando não há nada, e `Now` — o relógio contra o qual o
atraso é medido, para um teste não quebrar sozinho na manhã seguinte. As datas são escritas pelo
[`ui.Date`](#formatação) com `Relative`, e a linha cujo dia acabou leva `ui-late`, a mesma classe
da [caixa de aprovações](/pt/referencia/approval): atraso é igual em toda a aplicação, ou parece
defeito.

O `DeadlineBadge` não desenha nada com a lista vazia, e diz no `aria-label` o que a cor diz.

### VersionList

```go
func VersionList(c *trilha.Ctx, rows []VersionRow, o VersionOpts) h.Node
func VersionBadge(n int, published bool, words ...map[string]string) h.Node
func Changed(before, after map[string]string) []string
```

O histórico de um [`trilha.Versioned[T]`](/pt/referencia/app#versionedt): quem, quando, o que
mudou, e os botões de publicar ou voltar. Do mais novo para o mais velho, porque um histórico se lê
de agora para trás.

Sem JavaScript e sem biblioteca de diff: o que mudou é uma lista de nomes de campo, e o
`ui.Changed` a monta a partir de dois mapas de strings — a metade honesta de um diff, e a metade
que alguém lê antes de abrir a versão. A linha publicada não traz botões: ela já é a que todo mundo
lê.

## Formatação

Data, tamanho, duração e contagem não são domínio: são iguais em toda aplicação, e toda
aplicação escreve de novo — normalmente quatro vezes, um pouco diferente em cada uma, e quase
sempre ignorando o fuso.

```go
// app/setup.go
func Config(cfg *trilha.Config) {
	cfg.Locale = "pt-BR"                  // "en" é o valor zero
	cfg.TimeZone = "America/Sao_Paulo"    // vazio significa UTC
}
```

```go
ui.Date(c, doc.CriadoEm)                  // <time datetime="…">08/09/2026 12:04</time>
ui.Date(c, doc.CriadoEm, ui.Relative())   // há 3 min, com o absoluto no title
ui.Date(c, doc.CriadoEm, ui.DateOnly())   // 08/09/2026
ui.Bytes(c, doc.Tamanho)                  // 1,4 MB      (en: 1.4 MB)
ui.Duration(c, tarefa.Levou)              // 2 min 13 s
ui.Number(c, total)                       // 12.345      (en: 12,345)
ui.Number(c, preco, ui.Decimals(2))       // 1.234,56
```

### As regras que valem saber

**Valor ausente é um travessão.** `time.Time` zero, `*time.Time` nil, tamanho zero: todos
renderizam `—` em texto apagado. Data zero saindo como `01/01/0001` é o bug que isto remove.

**O `datetime` é sempre o instante.** O texto é local e traduzido; o atributo que a máquina lê
é RFC 3339 em UTC — então um copiar-colar, uma ordenação ou um leitor de tela recebe o fato, e
não a apresentação.

**O `Relative` não anda.** Ele escreve "há 3 min" e guarda o absoluto no `title`. Nada o
atualiza: o kit não tem relógio e não quer um. Tela que precisa do número se mexendo põe o
pedaço num `ui.Poll` — uma decisão que a página toma e paga, uma vez.

**`Bytes` é base 10.** kB, MB, GB: o que o gerenciador de arquivos de quem está lendo já
mostra. A contagem exata fica no `title`.

**Fuso desconhecido cai para UTC e avisa no log.** Cair em silêncio deslocaria todo horário da
tela sem nada parecer quebrado.

### Eles recebem um Ctx, e isso é de propósito

`ui.Date(c, t)` e não `ui.Date(t)`. Um idioma de pacote seria compartilhado por duas aplicações
rodando num processo — que é exatamente o que o `trilha.Provide` e o app embutido existem para
permitir —, e a segunda a subir mudaria a primeira em silêncio. O `ui.Head`, o `ui.Flashes` e o
`ui.DataTable` recebem um `Ctx` pelo mesmo tipo de razão.

### Dinheiro não está aqui

A moeda, onde fica o símbolo, como um negativo se lê: são decisões da aplicação, e um framework
que chutasse estaria errado no país de alguém. `ui.Number(c, v, ui.Decimals(2))` com o símbolo
escrito ao lado é a receita inteira.

O `trilha audit` avisa sobre `time.Format("02/01/2006")` dentro de `app/`: layout na página
ignora o `Config.TimeZone`, que é como uma data mostrada a alguém de outro país acaba
simplesmente errada.

## Tema

`ui.theme.css` define, em `:root` e `.dark`, exatamente as variáveis do shadcn/ui v4:
`--background/--foreground`, `--card/--card-foreground`, `--popover/…`, `--primary/…`,
`--secondary/…`, `--muted/…`, `--accent/…`, `--destructive`, `--border`, `--input`, `--ring`,
`--chart-1…5`, `--sidebar…`, `--radius`. `ui.css` deriva `--radius-sm/md/lg/xl`. O modo
escuro é a classe `dark` no `<html>` (o script de `ui.Head` aplica a preferência salva ou a
do sistema antes da primeira pintura).

## CLI

`trilha ui [--force] [--css-only|--js-only]` grava os seis arquivos em `public/`:
`ui.theme.css` só é criado (nunca sobrescrito); `ui.css`, `ui.js`, `ui.nav.js`,
`ui.upload.js`, `ui.live.js`, `ui.chat.js` e `ui.island.js` são atualizados quando iguais a uma versão anterior e, se você os editou, só
com `--force`.
