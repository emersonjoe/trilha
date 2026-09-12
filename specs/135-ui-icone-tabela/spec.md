# Spec 135 — ui: ícone próprio no NavItem e tabela em cartões

> Mudança pequena: um pacote (`ui`), sem convenção nova em `app/`, sem mudança incompatível
> na API pública.

- **Issues**: [#168](https://github.com/emersonjoe/trilha/issues/168) — `ui.NavItem.Icon` só
  aceita os 31 nomes embutidos, sem saída para um ícone próprio; [#178](https://github.com/emersonjoe/trilha/issues/178)
  — `ui.Table`/`ui.DataTable` só rolam na horizontal no celular, sem opção de a linha virar
  cartão. As issues são a fonte do escopo; este documento não as repete.
- **Branch**: `feat/ui-icone-tabela`
- **Versão**: 0.114.0

## Por quê

Um app de gestão precisa de ícones que o conjunto Lucide embutido não tem — documento,
prédio, régua, `</>` — e hoje `ui.Icon` entra em pânico com qualquer nome fora dos 31. A
única saída era pedir mais ícones ao kit, o que resolve hoje e não amanhã.

Separadamente, `ui.Table`/`ui.DataTable` desenham `<table>` dentro de uma faixa que só rola
na horizontal; numa tela de celular, uma listagem de sete colunas obriga a arrastar de um
lado a outro para ligar o valor à coluna, e a coluna de ações — normalmente a última — sai da
tela. O padrão que resolve isso (linha vira cartão abaixo de um limiar, com o rótulo da
coluna ao lado do valor) precisa do rótulo que só `Columns[T]` conhece; um CSS de app que não
tem acesso a esse rótulo teria de duplicá-lo à mão, coluna por coluna, longe da declaração e
sem ninguém para avisar quando as duas divergirem.

## O que muda

- `ui.NavItem` ganha `IconNode h.Node`: quando presente, é o que aparece no lugar de `Icon`
  — o app desenha o seu próprio nó (tipicamente um `h.Svg(...)` com a classe `ui-icon`, para
  herdar o tamanho e o alinhamento que o kit já define) e `ui.Icon` continua sendo o atalho
  para o conjunto embutido. `ui.EmptyOpts` ganha o mesmo campo, pela mesma razão: `ui.Empty`
  também só aceitava nome.
- `ui.Cards()` é um novo modificador de `ui.Table` (mesmo molde de `ui.Ghost()`/`ui.Sm()`):
  aplica a classe `ui-table-cards`, que abaixo de 640px vira cada linha um cartão, com o
  `<thead>` fora da tela mas ainda no *accessibility tree* (`clip-path`, não `display:none`).
  `ui.ListState` ganha `Cards bool`; `ui.DataTable` aplica `ui.Cards()` à tabela quando
  `true`. Ligado por opção, não por padrão: uma tabela numérica larga às vezes é melhor
  rolando mesmo.
- `ui.DataTable` passa a escrever `data-label` (o rótulo da coluna) em toda célula de dado,
  com ou sem `Cards` — é só um atributo, não muda desenho nenhum sem o CSS de `.ui-table-cards`,
  e elimina a duplicação do rótulo que um CSS de app teria de fazer à mão.

```go
ui.NavItem{Href: "/docs", Label: "Documentos", IconNode: h.Svg(h.Class("ui-icon"),
	h.Attr("viewBox", "0 0 24 24"), h.Attr("fill", "none"), h.Attr("stroke", "currentColor"),
	h.El("path", h.Attr("d", "…")))}

ui.DataTable(c, cols, rows, ui.ListState{Cards: true, /* … */})
// <table class="ui-table ui-table-cards">…<td data-label="Status">…</td>…</table>
```

## Fora de escopo

- Mais ícones embutidos no conjunto Lucide: é a alternativa que a própria issue #168
  descarta (resolve hoje, pesa para sempre em quem não usa).
- `ui.RegisterIcon` (estado global) para o mesmo problema: descartado pela issue #168 pelo
  mesmo motivo de sempre — estado global que qualquer struct literal já evita.
- Layout de cartão para a seleção em massa (`ListSelect`) — a célula da checkbox não ganha
  `data-label`; é o mesmo comportamento de hoje, só sem rótulo ao lado.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nenhuma dependência nova; `h.Svg`/`h.El` e `html.EscapeString` (via `h.Attr`/`h.Text`) já existem. |
| IV — API pública pequena e estável | Campos e função aditivos; `NavItem.Icon`, `EmptyOpts.Icon`, `ui.Table`, `ui.DataTable` mantêm assinatura e comportamento para quem não usa os campos novos. |
| VI — teste primeiro | `ui/shell_test.go` e `ui/ui_test.go` (ícone próprio vence o nome, escapado como os embutidos) e `ui/list_test.go` (`data-label` e `ui-table-cards`) antes da implementação. |
| VII — segurança por padrão | `IconNode` é um `h.Node` como qualquer outro — texto e atributos escapam pelas mesmas funções de sempre (`h.Text`, `h.Attr`); nenhum novo caminho para HTML não escapado. |

## Tarefas

- [x] T001 Teste que falha: `ui/shell_test.go` — `NavItem.IconNode` vence `Icon` e renderiza o
      nó dado, escapado como os embutidos.
- [x] T002 Teste que falha: `ui/ui_test.go` — `EmptyOpts.IconNode` vence `Icon`.
- [x] T003 Teste que falha: `ui/list_test.go` — `Cards: true` aplica `ui-table-cards` à
      tabela; `data-label` aparece em toda célula de dado com ou sem `Cards`.
- [x] T004 Implementação: `ui/shell.go` (`NavItem`, `Shell`), `ui/empty.go` (`EmptyOpts`,
      `Empty`), `ui/ui.go` (`Cards`), `ui/list.go` (`ListState`, `DataTable`, `bodyRows`),
      `ui/assets/ui.css` (`.ui-table-cards`).
- [x] T005 Uso em `examples/blog`: `Cards: true` na listagem de `app/documentos`;
      `app/admin/layout.go` novo, com `ui.Shell` e um `NavItem.IconNode` (ícone de documento,
      fora do conjunto embutido) para `/anexos`.
- [x] T006 Documentação nas duas locais (`site/internal/docs/content/{en,pt}/reference(ncia)/{shell,listings,ui}.md`)
      e `internal/uidoc/catalog.json` regerado (`make golden`).
- [x] T007 `CHANGELOG.md`, `version` em `cmd/trilha/main.go` (0.114.0), linha e item do
      `ROADMAP.md`.
- [x] T008 `make test` verde; `make api` (superfície pública) e `make golden` sem diferença
      pendente.

## Aceitação

- **SC-001**: `ui.NavItem{IconNode: n}` renderiza `n` no lugar de `Icon(it.Icon)`, mesmo com
  `Icon` também preenchido; sem `IconNode`, o comportamento de hoje (nome do conjunto
  embutido, pânico em nome desconhecido) não muda. O mesmo vale para `ui.EmptyOpts`.
- **SC-002**: `ui.DataTable` com `ListState.Cards: true` emite `class="ui-table ui-table-cards"`
  na tabela; toda célula de dado carrega `data-label` com o rótulo da coluna, com ou sem
  `Cards`. `ui.Table` sem `ui.Cards()` não muda de saída.
- **SC-003**: `make test` verde, incluindo `examples/blog`; `api/current.txt` e
  `internal/uidoc/catalog.json` sem diferença pendente depois de `make api golden`.
