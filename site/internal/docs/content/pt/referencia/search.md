---
title: search
description: Search, Doc, Kind, Query, SearchStore e SearchTerms — uma caixa de busca sobre vários tipos, agrupada por tipo.
---

A busca da barra de cima é igual em toda aplicação interna: uma caixa, alguns tipos, resultado
agrupado por tipo, e um clique que leva ao registro. O que cada aplicação escreve no lugar é um
`LIKE` por tipo, copiado — e aí descobre que "joao" não acha "João".

O `trilha.Search` é essa caixa. Está no runtime e não em um pacote próprio porque precisa do que o
runtime já tem: a requisição, o tenant da sessão, e mais nada.

## Declarar o índice

```go
var Busca = trilha.NewSearch(trilha.SearchOpts{
	Tenant: auth.Tenant,
	Allow:  func(c *trilha.Ctx, modulo string) bool { return acesso.Policy.Can(sessao.Atual(c), modulo, "read") },
}).
	Kind("pessoa", trilha.KindOpts{Label: "Pessoas", Module: "rh"}).
	Kind("processo", trilha.KindOpts{Label: "Processos"})
```

A ordem em que os tipos são declarados é a ordem em que os grupos voltam, para a tela de resultado
não se reorganizar entre uma busca e outra.

`Tenant` e `Allow` são funções e não valores porque o runtime não pode importar o `auth`: passe o
`auth.Tenant` e os dois concordam por construção.

## Gravar e consultar

```go
Busca.Put(c, trilha.Doc{Kind: "pessoa", ID: p.ID, Title: p.Nome,
	Body: p.Email + " " + p.CPF, URL: "/pessoas/" + p.ID})
Busca.Delete(c, "pessoa", id)
Busca.Reindex(c, "pessoa", docs)     // troca o tipo inteiro

res, err := Busca.Query(c, c.Query("q"), trilha.SearchQuery{Limit: 10})
res.Total                            // quantos casaram, somando os tipos
res.Groups[0].Label, res.Groups[0].Hits
```

O `res` é um `trilha.SearchResult`: um `Total` e um `Groups []trilha.SearchGroup`, um grupo por
tipo, na ordem em que os tipos foram declarados, cada um com o `Label` que a tela mostra e os
seus `Hits`. O agrupamento está acima do store de propósito — um store só casa e pontua —, então
a forma é a mesma com qualquer coisa embaixo.

Um `Doc` é uma projeção — o bastante para achar, nomear e ir até o registro. Não é o registro,
porque um índice que guarda a linha inteira é uma segunda cópia do banco com a sua própria
desatualização.

**Um tipo que ninguém declarou é recusado** com o `trilha.ErrUnknownKind`, levando um `Hint` cujo
código é o `trilha.ErrSearchKind` (`E_SEARCH_KIND`) e que lista os tipos que existem. Gravar em
silêncio seria um registro que nunca aparece na busca, sem nada apontando o porquê.

**Uma busca vazia responde vazio**, e não tudo: uma caixa por onde alguém passou de tab não pode
virar uma varredura da tabela inteira.

## As regras

- **Acento e caixa não contam.** O `SearchTerms` dobra a consulta — caixa baixa, acento fora,
  quebrando em tudo que não é letra nem dígito — e é exportado para quem escrever um store em SQL
  dobrar igual. Dois tokenizadores diferentes são um índice que não acha o que gravou.
- **Duas palavras querem dizer as duas.** Uma busca que responde *mais* quanto mais você digita é
  uma busca em que as pessoas param de digitar.
- **O título pesa mais que o corpo**, e é toda a classificação. Peso por campo é a próxima coisa
  que alguém pede e a primeira que ninguém consegue explicar.
- **O tenant não é opcional.** Um `Doc` sem `Tenant` herda o que o `SearchOpts.Tenant` responde, e
  a consulta filtra pelo mesmo. É a linha que alguém esquece uma vez, e o relatório que mostra as
  linhas de outro cliente é como se descobre.
- **Módulo negado não deixa rastro.** O tipo some do resultado — contador incluído. Um "3 pessoas"
  mostrado para quem não pode ver pessoas já contou o que não devia.

## O trecho não é HTML

```go
for _, parte := range hit.Snippet {
	// um trilha.SnippetPart: parte.Text, parte.Match
}
```

O pedaço do corpo em volta do que casou volta partido, com as partes que casaram marcadas — e não
como uma string com `<mark>` dentro. HTML montado pelo runtime a partir de um campo que alguém
digitou é uma injeção esperando o primeiro corpo com `<script>`. Quem escreve a tag é a tela, e o
[`ui.SearchResults`](/pt/referencia/ui) já escreve.

A palavra inteira é marcada, não o prefixo: meia palavra em amarelo parece defeito de renderização.

## O store

```go
type SearchStore interface {
	Put(ctx context.Context, docs []Doc) error
	Delete(ctx context.Context, kind, id string) error
	DeleteKind(ctx context.Context, kind string) error
	Search(ctx context.Context, q SearchStoreQuery) ([]Hit, error)
}
```

O `trilha.SearchMemory()` é o padrão, e é honesto sobre o que é: cada documento é varrido, o que está certo para os
milhares que uma aplicação interna tem e errado para os milhões que ela não tem. Uma tabela atrás
dos mesmos quatro métodos — FTS5 no SQLite, `tsvector` com índice GIN no Postgres — é o passo
seguinte, e nenhuma tela muda: o store só casa e pontua; o agrupamento, o tenant, o módulo e o
trecho são iguais acima de qualquer store.

**Não existe `trilha.SearchSQL`, e não vai existir.** O framework não tem driver, e um store que
precisasse de um seria uma dependência num pacote que não tem nenhuma. O SQL fica no seu projeto,
que é também onde fica a migração que cria o índice.

## As telas e a receita

O [`ui.SearchBox`](/pt/referencia/ui) é o campo — um formulário GET, então a busca vai para o
endereço e pode ser mandada para alguém, guardada e achada de novo no histórico. Ctrl+K e `/` levam
o foco para lá. O [`ui.SearchResults`](/pt/referencia/ui) é o resultado agrupado.

```
trilha add search
```

escreve o índice, a página `/busca` e os testes, e deixa para você a única coisa que só a sua
aplicação sabe: quais registros entram.

## SearchQuery.Rerank

```go
res, _ := Busca.Query(c, q, trilha.SearchQuery{Rerank: func(q string, hits []trilha.Hit) []trilha.Hit {
	return meuModelo.Ordenar(q, hits)
}})
```

O gancho para o que mais classificar — um índice vetorial, um modelo, uma regra escrita à mão. Roda
por tipo, sobre o que foi achado, que é o lugar honesto para ele: busca semântica é um provedor e
uma conta, e nenhum dos dois cabe atrás de uma função que este pacote chama por você.
