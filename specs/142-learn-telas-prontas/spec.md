# Spec 142 — learn: as telas prontas

> Mudança pequena: conteúdo do site (`site/internal/docs/content`, navegação em
> `site/internal/docs/docs.go`) mais um arquivo de código em `examples/cookbook`, sem convenção
> nova em `app/`, sem mudança na API pública.

- **Issue**: [#192](https://github.com/emersonjoe/trilha/issues/192) — o capítulo do kit ensina
  peça por peça e nunca monta uma tela; a issue é a fonte do escopo, este documento não a repete.
- **Branch**: `feat/learn-telas-prontas`
- **Versão**: 0.121.0

## Por quê

`learn/ui-kit.md` cresceu até virar catálogo: trinta seções, cada uma com uma peça e a sua demo.
Quem lê sabe o que é `ui.Shell`, o que é `ui.DataTable` e o que é `ui.Defer`, e continua sem
saber em que ordem eles entram numa tela, quem fala com quem e o que sobra para o handler
escrever. A pergunta que chega ("como é uma tela de app interno inteira no Trilha?") não tem
página: tem trinta pedaços e a suposição de que o leitor os junta sozinho.

A referência não resolve isso — ela é por pacote, e a resposta atravessa `ui`, `trilha.ListParams`,
`Ctx.Draft` e as rotas de fragmento. A receita também não: `cookbook/listing` é a listagem, não a
tela em volta dela. Falta o capítulo que monta **uma** tela de ponta a ponta e diz, a cada passo,
por que aquela peça e não outra.

## O que muda

Um capítulo novo em `learn/`, depois de `ui-kit` na navegação: `ui-screens.md` (pt:
`aprender/telas-prontas.md`, slug `telas-prontas`). Ele monta a tela de pedidos de um app
interno, na ordem em que ela é escrita — moldura, números, tabela, vazio, parte lenta,
pré-visualização, cadastro em etapas, o crachá do enum e a célula que se atualiza — e termina no
"Challenge" com solução, como os outros capítulos.

O código do capítulo é declaração de `examples/cookbook/screens.go`, arquivo novo do mesmo pacote
das receitas: compila em `go vet ./...` e um teste do site confere que cada bloco `go` da página
ainda existe num `.go` do repositório, do jeito que já vale para a seção Cookbook. Cada seção
linka a demo viva do kit (`/learn/ui-kit#…`) e a referência do que usa (`/reference/shell`,
`/reference/charts`, `/reference/listings`, `/reference/live`, `/reference/ui`).

`learn/ui-kit.md` ganha, no fim, a linha "o que vem depois" apontando para o capítulo novo, nas
duas locales.

```go
// exemplo do uso final: a tela inteira é uma função que compõe as peças na ordem
func OrdersPage(c *trilha.Ctx) (h.Node, error) {
	var q OrdersQuery
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	rows, total := searchOrders(q)
	return h.Div(
		ui.PageHeader("Orders", ui.ButtonLink("/orders/new", h.Text("New order"))),
		ui.Grid(orderStats(c)...),
		ui.Defer(c, "insights", "/orders/insights", ui.DeferOpts{Height: "12rem"}),
		ordersTable(c, q, rows, total),
	), nil
}
```

## Fora de escopo

- **Demo viva nova.** As dez peças da tela já têm demo (specs 139–141); o capítulo linka as que
  existem em vez de repetir cada uma numa demo composta que teria de ser mantida duas vezes.
- **App de exemplo novo.** A tela vive em `examples/cookbook` como declarações que compilam, e não
  como um app rodando: `examples/blog` e `examples/cadastro` já cobrem as convenções de `app/`,
  que este capítulo não inventa.
- **Reescrever `ui-kit.md`.** O catálogo continua onde está; ganha só o link de saída.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `examples/cookbook/screens.go` importa `trilha`, `h` e `ui` e mais nada |
| VI — teste primeiro | o teste que exige a página nas duas locales, na navegação depois de `ui-kit`, e os blocos vindos de um `.go` do repositório entra antes do capítulo |
| VII — segurança por padrão | a tela do capítulo passa `Sort` por `Restrict` (coluna vem da URL), usa `trilha.CSRFInput` no formulário em etapas e diz que `Hide` no menu é cosmética, não permissão |

## Tarefas

- [ ] T001 Teste que falha em `site/site_test.go`: `ui-screens`/`telas-prontas` logo depois do
      capítulo do kit nas duas locales, e todo bloco `go` do capítulo presente num `.go` do
      repositório
- [ ] T002 `examples/cookbook/screens.go`: a tela em declarações que compilam
- [ ] T003 O capítulo em `content/en/learn/ui-screens.md` e `content/pt/aprender/telas-prontas.md`,
      as duas entradas em `docs.go`, o link "o que vem depois" no fim do capítulo do kit
- [ ] T004 `CHANGELOG.md` (Docs), `version` em `cmd/trilha/main.go`, linha do `ROADMAP.md`
- [ ] T005 `make test` verde e `scripts/release.sh 0.121.0 --issues "192"`

## Aceitação

- **SC-001** `/learn/ui-screens` e `/pt/aprender/telas-prontas` respondem 200, aparecem na
  navegação depois do capítulo do kit e nos quatro `llms.txt`/`llms-full.txt`
  (`TestEveryPageResponds`, `TestLLMsIndex`, `TestLLMsFull`).
- **SC-002** Todo bloco `go` do capítulo, nas duas locales, é texto de um `.go` do repositório
  (teste novo), e `go vet ./...` cobre `examples/cookbook/screens.go`.
- **SC-003** As duas locales têm as mesmas demos na mesma ordem e o capítulo tem desafio com
  solução (`TestLocalesInSync`, `TestChaptersHaveChallengeAndSolution`), e nenhum link atravessa
  locale (`TestNoCrossLocaleLinks`, `TestInternalLinksResolve`).
