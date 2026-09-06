# Spec 023 — Navegação no cliente, por atributo e opcional

- **Issue**: [#23](https://github.com/emersonjoe/trilha/issues/23) (ROADMAP, Fase 1, item 4)
- **Branch**: `023-navegacao`
- **Versão**: 0.14.0

## Por quê

O fragmento (018) resolve quando o handler responde *um pedaço*: filtrar lista, salvar
formulário, trocar um bloco. A ilha (022) resolve quando o estado é do cliente. Sobrou o
caso mais comum de todos, que é ir para *outra página*: um painel com barra lateral, um
docs, um app com abas de nível superior. Hoje cada clique recarrega o documento inteiro — o
cabeçalho pisca, a rolagem de uma lista longa some, o tema reaplica, e quem tem um player ou
um socket aberto perde o estado. A saída de quem sente isso é adotar uma SPA e passar a
manter roteador, estado e hidratação no cliente, por um problema que é de pintura.

Fazer isso à mão não é difícil, é chato de acertar: `history.pushState`, `popstate`,
rolagem, foco para leitor de tela, requisição concorrente, recuo quando o servidor responde
5xx ou redireciona. Errar qualquer um deles quebra o Voltar — e um Voltar quebrado é pior
que uma recarga.

## O que muda

Três símbolos no kit `ui` e um arquivo novo em `public/`:

```go
func Navigate(id string) h.Node    // marca a região; id vazio = o próprio elemento
func NoNavigate() h.Node           // deixa um link de fora
func NavigateScript(c *trilha.Ctx) h.Node // <script defer src=ui.nav.js>
```

```go
// app/painel-/layout.go
return h.Section(h.Class("app"), ui.Navigate("conteudo"), ui.NavigateScript(c),
	ui.Sidebar(ui.Nav(ui.NavLink("/painel", "Painel", cur == "/painel"))),
	h.Div(h.Class("app-content"), children),
), nil
```

Um clique em link da mesma origem dentro da região busca a **página inteira** no mesmo
endereço, extrai o elemento `#id` e troca. O servidor não muda: mesma rota, mesma resposta,
mesmo endereço na barra — recarregar dá a mesma página. Sem JavaScript, o link é um link.

O que o navegador continua fazendo: Voltar e Avançar (restaurando a rolagem da entrada para
onde voltam, com `history.scrollRestoration = "manual"`), `Cmd`/`Ctrl`-clique e botão do
meio, `target`, `download`, âncora na mesma página, link para outra origem. O que o kit
acrescenta: `aria-busy` na região durante a espera, foco (`tabindex="-1"`) no que entrou,
`ui.hydrate` e o evento `trilha:swap` — que é como uma ilha da página nova monta. Uma
requisição por vez: um segundo clique aborta a primeira. Recuo para `location.assign` em
5xx, erro de rede, redirecionamento ou página sem aquele id.

O comportamento vai em **`ui/assets/ui.nav.js`**, arquivo à parte, não no `ui.js`, e o
`ui.Head` não o carrega: quem não navega assim não baixa nada. `ui.Files` passa a ter quatro
nomes e `trilha ui --js-only` grava os dois `.js`.

## Fora de escopo

- **Prefetch no `hover`/`mousedown`.** É a otimização óbvia e a fonte óbvia de tráfego
  inútil; entra depois, medida, se entrar.
- **Barra de progresso.** `aria-busy` já dá o gancho de CSS; barra é decisão de design do
  app, não do framework.
- **Trocar mais de um elemento por navegação.** Duas regiões independentes viram duas
  fontes de verdade de uma resposta só; se aparecer o caso, ele é um fragmento.
- **Roteador no cliente, cache de páginas visitadas, transição animada.** Cada um traz
  estado que precisa ser invalidado; a resposta do servidor continua sendo a verdade.
- **Ligar a navegação por padrão.** O ganho é de moldura e o custo é um `fetch` no caminho
  de todo clique; quem tem moldura pede.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | JS de ~3 KB sem dependência; nada novo no runtime Go |
| III — coerente com Go | os helpers são `h.Node` como o resto do kit; `Asset` já dá a URL com hash |
| IV — aprimoramento progressivo | sem JavaScript o link recarrega; o servidor responde a página inteira sempre |
| VI — teste primeiro | testes do kit e do `examples/blog` escritos antes |
| VII — segurança por padrão | `credentials: "same-origin"`, só mesma origem, `Accept: text/html` para que `auth.Require` responda 302 e a sessão expirada caia no login |

## Tarefas

- [x] T001 Testes que falham: `TestNavigateIsOptIn`, `TestNavigateScript`
- [x] T002 `ui/assets/ui.nav.js`, `Navigate`/`NoNavigate`/`NavigateScript`, `Files`, filtro por sufixo no `scaffold.WriteUI`
- [x] T003 Uso em `examples/blog` (`app/painel-/layout.go`) + `TestNavegacaoNoClienteDegradaSemScript`
- [x] T004 Documentação nas duas locales (`learn/interactivity`, `reference/ui`)
- [x] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item 4 da Fase 1 no `ROADMAP.md`
- [x] T006 `make test` verde e `make release VERSION=0.14.0 ISSUES="23"`

## Aceitação

- **SC-001** `ui.Navigate`/`NoNavigate` rendem `data-trilha-nav`, `ui.nav.js` está em
  `ui.Files` e o `ui.js` não conhece `data-trilha-nav` (`TestNavigateIsOptIn`).
- **SC-002** `NavigateScript` respeita `BasePath` e o hash de conteúdo (`TestNavigateScript`).
- **SC-003** `/painel` traz a região marcada e o script, e `/relatorio` pedido direto devolve
  o documento inteiro (`TestNavegacaoNoClienteDegradaSemScript`).
- **SC-004** `trilha ui --js-only` grava `ui.js` e `ui.nav.js` (`TestWriteUIStamp`).
