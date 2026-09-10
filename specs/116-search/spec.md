# Spec 116 — `Search`: uma caixa, vários tipos

- **Issue**: [#149](https://github.com/emersonjoe/trilha/issues/149) — a issue é a fonte do escopo.
- **Branch**: `116-search`
- **Versão**: 0.95.0

## Por quê

A busca da barra de cima é a mesma em todo app interno: uma caixa, vários tipos, resultado
agrupado, clique leva ao registro. Medido no Acervo: 524 linhas de rota, 161 de serviço e 138 de
tela, com um bloco `ilike('%palavra%')` copiado por tipo.

O Trilha tem `c.Query`, `List` com filtro por coluna e `ui.SearchField` para **uma** tabela. Não
tem nada que atravesse tipos.

## O que muda

```go
var Busca = trilha.NewSearch(trilha.SearchOpts{
	Tenant: auth.Tenant,
	Allow:  func(c *trilha.Ctx, modulo string) bool { return acesso.Policy.Can(sessao.Atual(c), modulo, "read") },
}).
	Kind("pessoa", trilha.KindOpts{Label: "Pessoas", Module: "rh"}).
	Kind("processo", trilha.KindOpts{Label: "Processos"})

Busca.Put(c, trilha.Doc{Kind: "pessoa", ID: p.ID, Title: p.Nome, Body: p.CPF + " " + p.Email, URL: "/pessoas/" + p.ID})
res, _ := Busca.Query(c, c.Query("q"), trilha.SearchQuery{Limit: 10})
```

- **Acento e caixa não importam.** "joao" acha "João", que é o motivo de a busca ser um índice e
  não um `LIKE`.
- **Tenant vem do contexto.** `Doc.Tenant` vazio herda o do `SearchOpts.Tenant`, e a consulta
  filtra pelo mesmo. É a linha que alguém esquece uma vez e vira incidente.
- **Módulo negado some do resultado**, tipo inteiro: um contador que diz "3 pessoas" para quem não
  pode ver pessoas já contou o que não devia.
- **O trecho não é HTML.** O runtime devolve o pedaço partido em `Snippet{Text, Match}` e quem
  desenha o `<mark>` é o kit: uma string com marcação vinda do dado é uma injeção esperando o
  primeiro `Body` com `<script>`.
- **`ui.SearchBox` e `ui.SearchResults`**: a caixa do shell (com Ctrl+K) e o resultado agrupado
  por tipo com contador.
- **`trilha add search`**: setup, rota `/busca`, a caixa no layout e o teste.

## Fora de escopo

- **`trilha.SearchSQL(db)`.** O framework não tem driver e não vai ter: store é interface mais
  memória, e o SQL mora na receita — a mesma decisão do `Versioned`, do `approval` e de todas as
  outras. Um `SearchSQL` aqui pediria FTS5 e `tsvector` dentro de um pacote com zero dependências,
  e o critério de aceite "mesmo resultado em sqlite e postgres" não é testável neste repositório
  por construção.
- **Sugestão enquanto digita, busca vetorial, sinônimos.** O `SearchQuery.Rerank` é o gancho de
  quem quiser plugar o resto.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `strings`, `sort`, `unicode` |
| VI — teste primeiro | acento, agrupamento, tenant, módulo negado, trecho |
| Store é interface | `SearchStore` + `SearchMemory`, como o `VersionStore` |

## Tarefas

- [x] T001 Teste que falha: acento, tenant, módulo, agrupamento e trecho
- [x] T002 `Doc`, `Search`, `SearchStore`, `SearchMemory` e `SearchTerms`
- [x] T003 `ui.SearchBox`, `ui.SearchResults` e o Ctrl+K
- [x] T004 Receita `trilha add search`
- [x] T005 Documentação (en + pt), superfície de API e catálogo do kit
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.95.0`

## Aceitação

- **SC-001** "joao" acha "João", e "JOAO" também.
- **SC-002** Um documento de outro tenant não aparece.
- **SC-003** Um tipo cujo módulo é negado não aparece, nem no contador.
- **SC-004** O resultado sai agrupado na ordem em que os tipos foram declarados.
- **SC-005** O trecho vem partido, com as partes que casaram marcadas, e o `<` do corpo continua
  sendo um `<`.
- **SC-006** `trilha add search` deixa o projeto verde no `trilha check`.
