# Spec 039 — `Pagination` e `Tooltip` no kit `ui`

- **Issue**: [#39](https://github.com/emersonjoe/trilha/issues/39) — a issue é a fonte do
  escopo.
- **Branch**: `039-ui-pagination-tooltip`
- **Versão**: 0.30.0

## Por quê

O kit já tem os quatorze componentes da lista de avaliação menos dois, e os dois que faltam
são justamente os que a pessoa escreve à mão errado. Paginação vira uma fileira de `<span>`
com `onclick`, sem `rel`, sem `aria-current` e sem endereço para compartilhar. Dica vira um
`div` que aparece no `:hover` — invisível para quem navega por teclado e inalcançável em
telefone, onde não existe hover.

Hoje quem usa o kit copia a `Pages` da receita de [paginação](/pt/receitas/paginacao) (que é o
rodapé mínimo, sem janela de números) e usa `title=""` para a dica, aceitando o atraso de meio
segundo do navegador e nenhum estilo.

## O que muda

Duas funções novas em `ui`, mais o CSS e o comportamento correspondentes em `ui.css`/`ui.js`.

```go
// Pages descreve a navegação de páginas a renderizar.
type Pages struct {
	Page, Total int              // página atual (1-based) e quantas existem
	Href        func(int) string // endereço de uma página
	Prev, Next  string           // rótulos; "Previous"/"Next" quando vazios
	Label       string           // aria-label do <nav>; "Pagination" quando vazio
}

func Pagination(p Pages) h.Node
func Tooltip(text string, children ...h.Node) h.Node
```

`Pagination` rende um `<nav aria-label>` com uma `<ul>` de links reais: anterior com
`rel="prev"`, seguinte com `rel="next"`, uma janela de números em torno da atual (primeira e
última sempre presentes, reticências `aria-hidden` nos buracos) e a atual marcada com
`aria-current="page"` — sem `<a>`, porque ela não leva a lugar nenhum. As bordas não viram
link desabilitado: elas simplesmente não são renderizadas.

`Tooltip` envolve o alvo num `<span class="ui-tooltip" data-ui-tooltip="…" title="…">`. Sem
JavaScript o `title` é a dica, que é o comportamento nativo do navegador. Com `ui.js` na
página o `title` é removido (para não haver duas dicas), uma bolha com `role="tooltip"` é
criada, o alvo ganha `aria-describedby`, e a bolha aparece no `hover`, no foco de teclado e
num toque — e some no `Escape`, o que é exigência do WCAG 1.4.13.

```go
ui.Pagination(ui.Pages{
	Page: page, Total: total,
	Href: func(n int) string { return "/blog?page=" + strconv.Itoa(n) },
})

ui.Tooltip("Only you can see this", ui.Button(ui.Ghost(), ui.Icon("eye")))
```

## Fora de escopo

- **Paginação por cursor no componente.** O cursor não tem número de página; o rodapé dele são
  dois links e já está escrito na receita.
- **Posicionamento com `anchor-position`.** Ainda não está em todo navegador estável; a bolha
  é posicionada com CSS relativo ao invólucro e presa à janela pelo `ui.js`.
- **Tooltip em conteúdo rico.** O texto é `string` de propósito: uma dica com link dentro é um
  popover, e o kit já tem `Menu`.

## Orçamento do `ui.js`

A FR-007 da spec 006 fixou `ui.js` em 10 KB sem minificar, e o arquivo já estava em 9,9 KB. O
tooltip é o primeiro componente desde o lançamento do kit que precisa de script próprio — uma
dica que não fecha não é acessível (WCAG 1.4.13) —, então o teto do `ui.js` passa a 12 KB e o
teste do tamanho vai junto. O `ui.css` continua dentro dos 25 KB dele.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `h` e `strconv`; nenhum arquivo novo além do CSS/JS já embutidos |
| VI — teste primeiro | `ui/ui_test.go` cobre a janela de números, os `rel`, o `aria-current` e o `title` do tooltip antes da implementação |
| VII — segurança por padrão | o texto da dica passa por `h.Text`/atributo escapado; nenhum `innerHTML` no `ui.js` |

## Tarefas

- [x] T001 Teste que falha em `ui/ui_test.go`: `Pagination` com 1, 3 e 20 páginas (janela,
      reticências, `rel=prev/next`, `aria-current`, nada de link para a página atual, sem
      anterior na primeira e sem seguinte na última); `Tooltip` com `title`, `data-ui-tooltip`
      e o texto escapado.
- [x] T002 `ui/ui.go`: `Pages`, `Pagination`, `Tooltip`.
- [x] T003 `ui/assets/ui.css` e `ui/assets/ui.js`: estilo da paginação e da bolha; upgrade
      progressivo do tooltip (remove `title`, cria a bolha, `aria-describedby`, foco, toque e
      `Escape`).
- [x] T004 Demo `ui-paginacao` nas duas locales (`site/internal/demos/kit.go`) e chamada no
      capítulo do kit (`learn/ui-kit`, `aprender/interface-com-ui`).
- [x] T005 Referência nas duas locales (`reference/ui`, `referencia/ui`): as duas linhas da
      tabela e o atributo novo na seção do `ui.js`.
- [x] T006 `CHANGELOG.md` (0.30.0), `version` em `cmd/trilha/main.go`, item 21 do `ROADMAP.md`.
- [x] T007 `make test` verde e `make release VERSION=0.30.0 ISSUES="39"`.

## Aceitação

- **SC-001** `Pagination` não emite `<a>` para a página atual, e emite `rel="prev"`/`rel="next"`
  quando essas páginas existem — verificado no teste.
- **SC-002** Com 20 páginas e a atual no meio, saem no máximo sete números, com a primeira e a
  última sempre presentes.
- **SC-003** `Tooltip` renderiza `title` com o mesmo texto do `data-ui-tooltip`, de modo que a
  dica funcione com o `ui.js` desligado.
- **SC-004** Nenhum manipulador de evento embutido no HTML gerado (o teste
  `TestNoInlineEventHandlers` do site continua verde) e `make test` passa.
