---
title: Listagens
description: A ListParams lê o estado da listagem na URL; o ui.DataTable desenha.
---

A tela que todo app de gestão tem — filtro em cima, tabela no meio, paginação embaixo — é
uma convenção aqui, não um componente para configurar. O estado inteiro mora no endereço,
então a página pode ser compartilhada, recarregada, favoritada e usada com o botão de
voltar, e quem decide o que é um nome de coluna é o servidor.

## ListParams

A `trilha.ListParams` é embutida na struct da tela e lida pelo `c.Bind`, que também aplica
os limites e guarda o resto da query:

```go
type Listagem struct {
	trilha.ListParams
	Status string `form:"status"`
}

var q Listagem
if err := c.Bind(&q); err != nil { return nil, err }
docs, total := repo.Listar(q.Q, q.Status, q.Sort, q.Asc(), q.Offset(), q.Limit())
```

| Campo | Parâmetro | Padrão |
|---|---|---|
| `Page int` | `page` | 1; qualquer coisa abaixo vira 1 |
| `PerPage int` | `per_page` | `trilha.DefaultPerPage` (20), no teto `trilha.MaxPerPage` (200) |
| `Sort string` | `sort` | vazio; um nome de coluna, até o `Restrict` dizer que é |
| `Dir string` | `dir` | `asc`; só a palavra exata `desc` inverte |
| `Q string` | `q` | vazio; o texto livre da busca |

| Método | Responde |
|---|---|
| `Offset() int`, `Limit() int` | o que o repositório quer |
| `Asc() bool` | a direção como booleano |
| `TotalPages(total int) int` | quantas páginas `total` linhas fazem, no mínimo 1 |
| `Restrict(cols ...string) bool` | derruba um `Sort` que não está em `cols`; diz se derrubou |
| `Href(pares ...string) string` | a mesma listagem com alguns parâmetros trocados e o resto preservado |
| `PageHref(n int) string` | o endereço da página `n`, no formato que o `ui.Pages` quer |

O `Href` recebe pares nome/valor, e valor vazio remove o parâmetro:

```go
p.Href("sort", "tamanho", "dir", "desc", "page", "")   // ?dir=desc&q=nota&sort=tamanho&tipo=nota
```

O resultado é só a query, uma URL relativa que mantém o caminho atual — a mesma listagem
funciona onde quer que esteja montada, e nenhum link precisa saber onde ele está.

**O `Sort` é um nome de coluna digitado por quem escreveu o endereço.** O `Restrict` é o
que o transforma em coluna: derruba o nome que não está na lista e avisa, então o
repositório nunca recebe coluna que ninguém declarou. O `ui.DataTable` o chama com as
colunas marcadas `Sort: true`; uma listagem que desenha a própria tabela chama na mão.

## ui.DataTable

```go
return ui.DataTable(c, ui.Columns[Doc]{
	{Key: "nome", Label: "Arquivo", Sort: true, Cell: func(d Doc) h.Node { return h.Text(d.Nome) }},
	{Key: "tamanho", Label: "Tamanho", Sort: true, Num: true, Cell: func(d Doc) h.Node { return h.Text(d.Tamanho()) }},
}, docs, ui.ListState{Params: q.ListParams, Total: total, ID: "lista", Search: "Buscar"}), nil
```

Veja funcionando: [demo](/pt/aprender/interface-com-ui#tabelas-que-vivem-na-url).

O `ui.Column[T]` é genérico: a linha é o tipo do seu domínio, não um `map[string]any`.

| Campo de `Column[T]` | Papel |
|---|---|
| `Key` | o nome pelo qual a URL ordena; também o que o `Sort` aceita |
| `Label` | o texto do cabeçalho |
| `Sort` | o cabeçalho vira um link de verdade que ordena por esta coluna |
| `Num` | coluna numérica, alinhada à direita com algarismos tabulares |
| `Cell func(T) h.Node` | a célula de uma linha |

| Campo de `ListState` | Papel |
|---|---|
| `Params` | o que o `Bind` leu; os links saem dele |
| `Total` | quantas linhas passaram pelo filtro, para a paginação e a contagem |
| `ID` | id do fragmento; com ele, ordenar, filtrar e paginar não recarregam |
| `Search` | placeholder (e `aria-label`) do campo `q`; sem busca quando vazio |
| `Filters` | os outros campos do formulário de filtro — um `<select>`, uma data, o que a tela tiver |
| `Empty` | o que aparece no lugar das linhas quando não há nenhuma |
| `Caption` | `<caption>` da tabela, lido por leitor de tela |
| `RowHref func(int) string` | o endereço para onde a linha daquela posição leva |
| `Select *ListSelect` | uma caixa por linha e uma barra com as ações que levam a seleção |
| `Cards bool` | abaixo de 640px, cada linha vira um cartão em vez de rolar a tabela de lado |

### Não há script novo

O cabeçalho que ordena é link, o filtro é `<form method=get>` e a paginação são links: sem
JavaScript, os três navegam e a rota responde a página inteira. Com o `ID` preenchido eles
levam o `ui.Swap(ID)`, e o `ui.js` do kit — que já está na página — pede o fragmento e
troca a tabela. O handler é o mesmo dos dois jeitos:

```go
tabela := lista(c, q)
if c.Fragment() == "lista" {
	return tabela, nil
}
```

### Cartão no celular

Uma tabela com cinco colunas ou mais só rola de lado no celular — o que tira de tela a
coluna de ações, em geral a última. `Cards: true` vira cada linha um cartão abaixo de
640px: o `ui.DataTable` escreve `data-label` em toda célula com o próprio `Label` da
coluna, e a folha de estilo faz o resto — o `<thead>` sai de tela com `clip-path` (ainda
alcançável por leitor de tela, ao contrário de `display: none`), e cada `<td>` mostra o
rótulo ao lado do valor. Uma tabela numérica larga — um comparativo, um extrato — às
vezes ainda é melhor rolando, por isso é `Cards: true` e não o padrão.

Um `ui.Table` escrito à mão ganha o mesmo ponto de corte com `ui.Table(ui.Cards(), …)`,
mas tem de escrever o próprio `data-label` em cada `<td>` — o ganho do `ui.DataTable`
fazer isso sozinho é o rótulo morar num lugar só, `Columns[T].Label`.

### Seleção de linhas

O `ListSelect{Name, Value, Action, Label, Bar}` embrulha a tabela num formulário `POST`
com o token do CSRF, acrescenta a coluna de caixas e mostra a `Bar` acima dela. A rota lê
a seleção em `c.Request().Form[Name]` depois do `c.ParseForm`. O `Value` é por posição,
como o `RowHref`: a linha de índice `i`.

### O formulário de filtro

O que os `Params` guardam e o formulário não manda viaja em campos escondidos — a ordem e
o tamanho da página, para a busca não jogar a ordenação fora. A página não vai junto:
filtro novo começa na página 1, a única que com certeza existe.

## O estado vazio

Uma lista sem nada e uma lista filtrada até o nada são duas telas diferentes. Dizer "nada por
aqui" para quem acabou de buscar *xyz* informa que a aplicação está vazia, quando o que
aconteceu é que o termo não casou com nada — e a saída está a um link de distância. O
`ui.DataTable` faz essa distinção sozinho, então a tela acerta sem ninguém pensar nisso:

| Situação | O que mostra |
|---|---|
| sem linhas, sem busca | um ícone e "Nothing here yet" |
| sem linhas, com `?q=xyz` | "No results for *xyz*", e um link que limpa o termo e volta à primeira página |

O `ListState.Empty` substitui os dois quando a aplicação tem algo melhor a dizer — um app em
português, por exemplo, já que os textos do kit são em inglês.

Veja funcionando: [demo](/pt/aprender/interface-com-ui#vazia-e-filtrada-ate-vazia).

O componente por trás disso vale sozinho:

```go
ui.Empty(ui.EmptyOpts{
	Icon:   "info",                        // um nome do kit; um desconhecido não desenha nada
	Title:  "Nenhum documento ainda",      // o único campo obrigatório
	Hint:   "Envie o primeiro PDF e a classificação começa sozinha.",
	Action: ui.ButtonLink("/upload", h.Text("Enviar documento")),
})
```

O `Hint` é o campo que se paga: é a diferença entre dizer que a tela está vazia e dizer o que
fazer a respeito.

`IconNode h.Node`, quando presente, vence `Icon`: a mesma saída do `ui.NavItem`, para um
ícone que os 31 nomes do kit não cobrem.

Para uma tela que não carregou, o `ui.EmptyError(c, título, err, ação)` mostra o título, o
caminho de tentar de novo, e o erro real **só em desenvolvimento** — a frase de um driver numa
página de produção é vazamento de informação com fonte amigável.

