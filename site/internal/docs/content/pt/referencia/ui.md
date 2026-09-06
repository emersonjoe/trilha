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
| `ui.Asset(nome) []byte` | conteúdo embutido de `ui.css`, `ui.theme.css`, `ui.js`, `ui.nav.js`, `ui.upload.js` ou `ui.live.js` |
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
| `DataTable(c, Columns[T], linhas, ListState)` | a listagem: formulário de filtro, cabeçalho ordenável, paginação e estado vazio, tudo na URL — veja [Listagens](/pt/referencia/listagens) |
| `Swap(id)` | `data-trilha-target`: o `<a>` ou `<form>` pede só o elemento `#id` e troca (fragmentos) |
| `Poll(intervalo, src)`, `Live(src)`, `On(evento, src)`, `LiveScript(c)` | fragmento que se atualiza pelo relógio ou por um evento do servidor — veja [Fragmentos vivos](/pt/referencia/vivo) |
| `NoPush()` | `data-trilha-push="false"`: a troca não mexe no histórico |
| `Icon(nome, attrs...)`, `Icons()` | SVG inline do Lucide; nome desconhecido → pânico (erro de programação) |

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
`ui.upload.js` e `ui.live.js` são atualizados quando iguais a uma versão anterior e, se você os editou, só
com `--force`.
