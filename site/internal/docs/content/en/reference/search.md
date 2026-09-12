---
title: search
description: Search, Doc, Kind, Query, SearchStore and SearchTerms — one search box over several kinds of thing, grouped by kind.
---

The search in the top bar is the same in every internal application: one field, a handful of
types, results grouped by type, and a click that goes to the record. What each application writes
instead is one `LIKE` per type, copied — and then finds out that "joao" does not match "João".

`trilha.Search` is that box. It is in the runtime and not in a package of its own because it needs
what the runtime already has: the request, the tenant on the session, and nothing else.

## Declaring the index

```go
var Busca = trilha.NewSearch(trilha.SearchOpts{
	Tenant: auth.Tenant,
	Allow:  func(c *trilha.Ctx, module string) bool { return acesso.Policy.Can(sessao.Atual(c), module, "read") },
}).
	Kind("pessoa", trilha.KindOpts{Label: "People", Module: "hr"}).
	Kind("processo", trilha.KindOpts{Label: "Cases"})
```

The order the kinds are declared in is the order the groups come back in, so the results page does
not rearrange itself between one search and the next.

`Tenant` and `Allow` are functions and not values because the runtime must not import `auth`: hand
it `auth.Tenant` and the two agree by construction.

## Writing and reading

```go
Busca.Put(c, trilha.Doc{Kind: "pessoa", ID: p.ID, Title: p.Name,
	Body: p.Email + " " + p.TaxID, URL: "/people/" + p.ID})
Busca.Delete(c, "pessoa", id)
Busca.Reindex(c, "pessoa", docs)     // replaces the whole kind

res, err := Busca.Query(c, c.Query("q"), trilha.SearchQuery{Limit: 10})
res.Total                            // how many matched, across every kind
res.Groups[0].Label, res.Groups[0].Hits
```

`res` is a `trilha.SearchResult`: a `Total` and a `Groups []trilha.SearchGroup`, one group per
kind, in the order the kinds were declared, each with the `Label` the screen shows and its
`Hits`. Grouping is above the store on purpose — a store only matches and scores — so the shape
is the same whatever is underneath.

A `Doc` is a projection — enough to find it, name it and go to it. It is not the record, because
an index that holds the whole row is a second copy of the database with its own staleness.

**A kind nobody declared is refused** with `trilha.ErrUnknownKind`, carrying a `Hint` whose code
is `trilha.ErrSearchKind` (`E_SEARCH_KIND`) and which lists the kinds that do exist. Storing it in
silence would be a record that never turns up in the search and nothing pointing at why.

**An empty query answers an empty result**, not everything: a box somebody tabbed past must not
become a full table scan.

## What the rules are

- **Accents and case do not count.** `SearchTerms` folds the query — lowercase, accents off, split
  on everything that is not a letter or a digit — and it is exported so a store written for SQL
  folds the same way. Two tokenizers that disagree are an index that cannot find what it wrote.
- **Two words mean both.** A search that answers *more* the more you type is a search people stop
  typing into.
- **The title outranks the body**, and that is the whole of the ranking. Weights per field are the
  next thing somebody asks for and the first thing nobody can explain.
- **The tenant is not optional.** A `Doc` with no `Tenant` inherits the one `SearchOpts.Tenant`
  answers, and the query filters by the same. It is the line somebody forgets once, and the report
  that shows another customer's rows is how they find out.
- **A denied module leaves no trace.** The kind disappears from the results — count included. A
  "3 people" shown to somebody who may not see people has already told them something.

## The snippet is not HTML

```go
for _, part := range hit.Snippet {
	// a trilha.SnippetPart: part.Text, part.Match
}
```

The piece of the body around the match comes back cut into parts, with the matched ones marked —
not as a string with `<mark>` in it. HTML built by the runtime out of a field somebody typed is an
injection waiting for the first body with a `<script>` in it. The tag is written by the screen, and
[`ui.SearchResults`](/reference/ui) already does it.

The whole word is marked and not the prefix: half a word in yellow reads like a rendering bug.

## The store

```go
type SearchStore interface {
	Put(ctx context.Context, docs []Doc) error
	Delete(ctx context.Context, kind, id string) error
	DeleteKind(ctx context.Context, kind string) error
	Search(ctx context.Context, q SearchStoreQuery) ([]Hit, error)
}
```

`trilha.SearchMemory()` is the default, and it is honest about what it is: every document is scanned, which is right
for the thousands an internal application has and wrong for the millions it does not. A table
behind the same four methods — FTS5 in SQLite, `tsvector` with a GIN index in Postgres — is the
next step, and no screen changes, because the store only matches and scores: the grouping, the
tenant, the module and the snippet are the same above every store.

**There is no `trilha.SearchSQL`, and there will not be one.** The framework has no driver, and a
store that needed one would be a dependency in a package that has none. The SQL belongs in your
project, which is also where the migration that creates the index belongs.

## Screens and the recipe

[`ui.SearchBox`](/reference/ui) is the field — a GET form, so the query ends up in the address and
a search can be sent to somebody, bookmarked and found again in the history. Ctrl+K and `/` move
the focus there. [`ui.SearchResults`](/reference/ui) is the grouped result.

```
trilha add search
```

writes the index, the `/busca` page and the tests, and leaves you the one thing only your
application knows: which records go in.

## SearchQuery.Rerank

```go
res, _ := Busca.Query(c, q, trilha.SearchQuery{Rerank: func(q string, hits []trilha.Hit) []trilha.Hit {
	return meuModelo.Ordenar(q, hits)
}})
```

The hook for whatever else ranks — a vector index, a model, a hand-written rule. It runs per kind,
on the hits that were found, which is the honest place for it: semantic search is a provider and a
bill, and neither belongs behind a function this package calls for you.
