# Spec 140 — site: demos do kit — dados e gráficos

> Mudança pequena: um pacote (`site/internal/demos`) mais o conteúdo do site, sem convenção
> nova em `app/`, sem mudança na API pública.

- **Issue**: [#190](https://github.com/emersonjoe/trilha/issues/190) — dados e gráficos são a
  segunda das três frentes de demos do kit; a issue é a fonte do escopo, este documento não a
  repete.
- **Branch**: `feat/demos-dados`
- **Versão**: 0.119.0

## Por quê

`site/internal/demos/kit.go` cobria layout e navegação (spec 139) mas nada da metade que
lida com dado: `ui.DataTable` só aparece na prosa de `reference/listings.md`, `ui.Tree` e
`ui.Combobox` só no catálogo, `ui.SearchBox`/`ui.SearchResults` idem, e a formatação por
`Config.Locale` — o motivo de `ui.Date`/`ui.Bytes`/`ui.Number` pedirem um `Ctx` — não tem
nenhum lugar que mostre a mesma tela em dois idiomas. Quem decide usar o Trilha vendo o site
não vê uma listagem filtrada até vazio, uma árvore de classificação ou um painel de números —
só o nome na tabela.

## O que muda

Oito demos novas em `site/internal/demos/kit.go` (função Go registrada no `init`, código e
resultado lado a lado — o mesmo molde das demos existentes), cobrindo os grupos da issue:

- **`ui-listagem`** — `ui.DataTable` com `ui.Columns`, ordenação e busca, sobre
  `trilha.ListParams` construído como o `Bind` deixaria.
- **`ui-listagem-vazia`** — as duas telas vazias lado a lado: sem filtro (`ui.Empty` padrão do
  kit, em inglês) e filtrado até vazio (`ListState.Empty` com a mensagem da aplicação, que é o
  caso que `reference/listings.md` já documenta como o motivo do campo existir).
- **`ui-arvore`** — `ui.Tree`/`TreeNode` e `ui.TreePicker` num plano de classificação de três
  níveis.
- **`ui-combobox`** — `ui.Combobox` com uma lista curta de opções.
- **`ui-busca`** — `ui.SearchBox` + `ui.SearchResults` sobre um `trilha.Search` de verdade,
  com `trilha.SearchMemory()` e um índice fixo.
- **`ui-locale`** — `ui.Date` (com `Relative`, `DateOnly`, `TimeOnly`), `ui.Bytes`,
  `ui.Duration` e `ui.Number` (com `Decimals`), a mesma tela renderizada duas vezes — uma vez
  com `Config.Locale` inglês, uma com `pt-BR` — postas lado a lado numa tabela só, para que a
  diferença apareça sem depender do idioma do site em que a demo está embutida.
- **`ui-indicadores`** — `ui.Stat`/`ui.StatHint`, `ui.Bars`, `ui.Sparkline`/
  `ui.SparklineTitle`, `ui.Donut` e `ui.ChartTitle` num painel de quatro cartões.
- **`ui-ao-vivo`** — `ui.Poll`/`ui.Live`/`ui.On`, com o código real de uma célula que se
  atualiza; o resultado renderizado é estático, pela mesma razão que `ui-espera` (spec 139) já
  é: a demo não tem uma rota de fragmento de verdade atrás dela, e um `ui.js`/`ui.live.js`
  tentando buscar algo que não existe na página publicada é pior do que uma célula parada.

`learn/ui-kit.md` e `aprender/interface-com-ui.md` ganham uma seção por demo, entre "O que
acontece durante uma troca" e "Atualizar e customizar". `reference/listings.md`,
`reference/charts.md`, `reference/live.md` e `reference/ui.md` (e os quatro espelhos em
`pt/referencia/`) ganham o link "veja \[demo\]" nas linhas do catálogo e a frase "Veja
funcionando: \[demo\](...)." nos pontos correspondentes da prosa — o mesmo mecanismo `@demo
<nome>` embutido no Markdown que as demos anteriores já usam; nenhuma rota `/demos/<id>` nova.

`ui.DataTable`, `ui.SearchBox` e `ui.SearchResults` leem `Config.Locale` do `Ctx` (a palavra
"Filter"/"Filtrar", o "results"/"resultados" do rodapé, a legenda da caixa de busca); como o
`Node` de uma demo não tem requisição por trás, as três demos que dependem disso sobem um
`*trilha.App` mínimo com o locale certo e capturam o `Ctx` de dentro do próprio handler — o
mesmo recurso que `ui/format_test.go` já usa para testar os formatadores nos dois idiomas,
aqui reaproveitado para renderizar (não testar) a demo.

```go
// exemplo do uso final: uma das oito funções em site/internal/demos/kit.go
add("en", Demo{
	Name:  "ui-locale",
	Title: "Config.Locale changes the word, not the code",
	Source: `ui.Date(c, doc.CreatedAt, ui.Relative())
ui.Bytes(c, doc.Size)
ui.Number(c, total, ui.Decimals(2))`,
	Node: func() h.Node { return wrap(localeComparison()) },
})
```

## Fora de escopo

- Demos de layout e navegação (spec 139) e das telas dos padrões: a terceira issue da série.
- Uma rota `/demos/<id>` própria: como na spec 139, o `@demo` embutido continua bastando.
- Um backend de verdade atrás de `ui-ao-vivo`: a demo mostra o código real da convenção, não
  uma prova de conceito de SSE dentro do site estático.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `site/internal/demos` continua dependendo só de `h`, `ui`, `trilha` e `net/http/httptest` (biblioteca padrão) para montar o `Ctx` das três demos que precisam de locale |
| VI — teste primeiro | `TestKitDemosDataRender` (novo) falha antes das oito demos existirem — cada demo (nas duas locales) precisa responder dentro da página que a embute e conter uma marca do componente; a de locale precisa mostrar o valor em inglês e o valor em `pt-BR` na mesma saída |
| VII — segurança por padrão | Nenhum HTML novo escapa por fora do `h`; nenhum link do resultado renderizado é absoluto (mesma regra da spec 139, por causa de `TestInternalLinksResolve`); o índice de busca da demo é fixo e não lê nada de fora |

## Tarefas

- [x] T001 Teste que falha: `TestKitDemosDataRender` em `site/site_test.go`, contra o estado
      sem as oito demos novas
- [x] T002 As oito demos em `site/internal/demos/kit.go` (pt e en), com o `demoCtx` auxiliar
      para as três que precisam de locale
- [x] T003 Seções novas em `learn/ui-kit.md` / `aprender/interface-com-ui.md` com os `@demo`;
      links "veja \[demo\]" e "Veja funcionando" nas quatro páginas de referência (en e pt)
- [x] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T005 `make test` verde

## Aceitação

- **SC-001**: `go test ./site/...` passa com as oito demos novas nas duas locales, mesma
  quantidade e mesma ordem de `@demo` (`TestLocalesInSync`).
- **SC-002**: nenhuma página do site linka para um caminho que não existe
  (`TestInternalLinksResolve` continua verde com o HTML das demos novas).
- **SC-003**: a demo `ui-locale` mostra, na mesma saída, um valor no formato inglês (separador
  de milhar `,`) e um no formato `pt-BR` (separador `.`, decimal `,`) — `TestKitDemosDataRender`
  verifica os dois.
- **SC-004**: cada componente da issue #190 aparece em pelo menos uma demo viva e tem um link
  "veja demo" a partir da linha correspondente em `reference/ui.md` (en e pt), mais os links de
  prosa em `listings.md`/`listagens.md`, `charts.md`/`graficos.md` e `live.md`/`vivo.md`.
