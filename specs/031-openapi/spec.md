# Spec 031 — OpenAPI a partir das rotas

- **Issue**: [#31](https://github.com/emersonjoe/trilha/issues/31) (ROADMAP, Fase 3, item 13)
- **Branch**: `031-openapi`
- **Versão**: 0.22.0

## Por quê

Quem consome a API de um app Trilha hoje descobre o contrato lendo o `route.go`. O framework
já sabe quase tudo o que um documento OpenAPI pede: o scanner conhece o caminho, o método e o
parâmetro de rota; o `Bind` conhece a struct de entrada e, desde a 0.18.0, as regras de
validação de cada campo; a 0.21.0 fixou o formato do erro (`problem+json`). O que falta é
escrever isso num arquivo que o resto do mundo lê.

A alternativa que todo framework escolhe é a anotação: um comentário mágico por operação,
mantido à mão, que envelhece separado do código. A aposta aqui é a mesma do gerador de rotas
— **ler o que já está escrito**. O caminho vem do diretório, o corpo vem da struct que o
`Bind` preenche, o schema do campo vem da tag `json` e da tag `validate`, o status vem do
`c.JSON(http.StatusCreated, …)` que o handler já chama. O comentário fica onde a dedução não
alcança, e só lá.

## O que muda

Um comando novo:

```
trilha openapi [-o openapi.json] [--title T] [--version V] [--server URL] [--check]
```

Ele varre `app/`, deduz o que dá e escreve um documento OpenAPI 3.1 em JSON. `-o -` escreve na
saída padrão; `--check` compara com o arquivo existente sem escrever (para o CI), do mesmo
jeito que `trilha gen --check`.

Só as rotas de API entram (`route.go`, `Kind == api`). Uma página serve HTML para um
navegador; descrevê-la em OpenAPI seria inventar um contrato que ninguém consome.

### O que é deduzido

| Do código | Vira |
|---|---|
| diretório da rota | caminho (`app/api/posts/id_` → `/api/posts/{id}`) |
| função exportada `GET`, `POST`, … | operação do método |
| segmento `id_` / `path__` | `parameter` em `path`, obrigatório |
| doc comment da função | `summary` (primeira frase) e `description` (o resto) |
| `c.Bind(&in)` / `c.BindJSON(&in)` | `requestBody` com o schema de `in` e mais uma resposta 422 |
| `c.JSON(http.StatusCreated, p)` | resposta 201 com o schema do tipo de `p` |
| `c.Writer().WriteHeader(http.StatusNoContent)` | resposta 204 sem corpo |
| `c.Header("Content-Type", "text/csv; charset=utf-8")` | media type das respostas de sucesso |
| `trilha.ErrNotFound`, `trilha.Errorf(400, …)`, `&trilha.Problem{Status: 409}` | respostas de erro em `application/problem+json` |
| tags `json` e `validate` da struct | `properties`, `required`, `maxLength`, `enum`, `format` |

O tipo de uma expressão sai de um índice de todos os `.go` do projeto (fora de `_test.go` e
`testdata/`): `p, ok := posts.Get(…)` dá a `p` o primeiro resultado de `posts.Get`, e
`posts.Post` vira `#/components/schemas/posts.Post`. É inferência sintática, não checagem de
tipos: o que não estiver ao alcance dela não é adivinhado — fica sem schema, e o comentário
resolve.

Toda operação ganha uma resposta `default` apontando para o schema `Problem` (RFC 9457), que
é o que o runtime responde desde a 0.21.0.

### O que é declarado

Diretivas no doc comment do handler, uma por linha:

```go
// GET returns one post by slug.
//
// openapi:response 200 posts.Post
// openapi:query mes string  competência no formato AAAA-MM
// openapi:tag posts
func GET(c *trilha.Ctx) error { … }
```

| Diretiva | Efeito |
|---|---|
| `openapi:response <status> [tipo]` | acrescenta ou substitui a resposta daquele status |
| `openapi:body <tipo>` | corpo da requisição, quando o `Bind` não é dedutível |
| `openapi:query <nome> <tipo> [descrição]` | `parameter` em `query` |
| `openapi:tag <nome>` | troca a tag padrão (último segmento fixo do caminho) |

`<tipo>` é `pacote.Tipo` (ou `Tipo`, no próprio pacote da rota), `[]pacote.Tipo` para lista.
Um nome que o índice não encontra é erro do comando, não um schema vazio silencioso.

### Superfície

| Símbolo | Papel |
|---|---|
| `trilha openapi` | o comando |
| `internal/openapi.Generate(root string, res *scan.Result, o Options) ([]byte, error)` | o documento |
| `internal/openapi.Options` | `Title`, `Version`, `Server` |

## Fora de escopo

- **YAML.** O JSON é válido em qualquer ferramenta; um emissor YAML próprio custaria mais do
  que vale sem dependência externa.
- **Autenticação (`securitySchemes`).** O framework não tem opinião sobre auth ainda (issue
  #41); declarar isso agora seria inventar um contrato que o runtime não sustenta.
- **Rotas de página.** Ver acima.
- **Checagem de tipos de verdade** (`go/types` com importador). A inferência sintática cobre
  o app de referência inteiro; o custo de carregar pacotes compilados não se paga.
- **Servir o documento numa rota** (`/openapi.json`, Swagger UI). Um arquivo gerado é o
  suficiente; quem quiser servi-lo faz um `route.go` de três linhas.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — roteamento por arquivos é a fonte | o documento sai do mesmo scanner que gera as rotas; nada é declarado duas vezes |
| II — só biblioteca padrão | `go/ast`, `go/parser`, `encoding/json` |
| III — gerador determinístico | mesma entrada, mesmos bytes: golden em `testdata/golden`, `--check` no CI |
| IV — convenção nova pede scanner + exemplo + integração | as diretivas aparecem no `examples/blog` e no `examples/orcamento`, com teste de ponta a ponta |
| VI — teste primeiro | golden dos dois exemplos antes do gerador |
| IX — API pública pequena | nada novo em `package trilha`; o comando é a superfície |

## Aceitação

- **SC-001** `cd examples/blog && trilha openapi -o -` sai com um documento OpenAPI 3.1 em
  que `/api/posts` tem `get` e `post`, `/api/posts/{id}` tem `get` e `delete` com o parâmetro
  de caminho, e `components.schemas` traz `posts.Post` e `Problem`.
- **SC-002** O `post` de `/api/posts` traz `requestBody` com `title` (`maxLength: 80`) e
  `body`, `required: ["title"]`, resposta 201 com `posts.Post`, 409 e 422.
- **SC-003** `examples/orcamento` produz documento válido: a rota de CSV responde 200 com
  media type `text/csv` e 400 em `problem+json`.
- **SC-004** Duas execuções seguidas geram bytes idênticos, e `--check` falha depois de
  acrescentar uma rota.
- **SC-005** Um `openapi:response` com tipo que não existe falha com mensagem que diz o nome
  e o arquivo.
