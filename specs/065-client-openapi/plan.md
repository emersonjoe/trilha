# Plano — spec 065

## Onde cada coisa entra

| Arquivo | O que faz |
|---|---|
| `internal/client/doc.go` (novo) | leitura do documento OpenAPI: `Doc`, `Schema`, `Operation`, resolução de `$ref` |
| `internal/client/types.go` (novo) | esquema → tipo Go: nome, campos, tags `json`/`validate`, `allOf`, recursão |
| `internal/client/gen.go` (novo) | emissão do arquivo: `New`, `Option`, `Error`, os grupos por tag e os métodos |
| `internal/client/client_test.go` (novo) | golden com `-update`, determinismo, e o cliente gerado exercitado contra `httptest` |
| `testdata/openapi/` (novo) | documento sintético com paginação, path, enum, multipart, binário, `oneOf`, `$ref` recursivo |
| `testdata/openapi.golden/` (novo) | o arquivo gerado |
| `cmd/trilha/client.go` (novo) | `trilha client <doc> [--out] [--package] [--check]` |
| `cmd/trilha/main.go`, `i18n.go` | o `case` e as mensagens nas duas línguas |
| `Makefile` | `./internal/client/` no alvo `golden` |
| docs | `reference/cli` nas duas línguas, receita ligando upstream + cliente |

## Ordem

O leitor primeiro, porque tudo depende da resolução de `$ref`; a tradução de esquema depois,
que é onde estão as decisões 3 a 5 e 10; a emissão em seguida, com o golden; a CLI e o
`--check` no fim, quando já há o que checar.

## O que não entra

- OpenAPI 2.0 (Swagger) e YAML: o documento é JSON 3.x. Quem tem YAML converte uma vez.
- `securitySchemes`: a credencial entra pelo `WithHeader`, que é a mesma porta do upstream.
- Servidor de mock, retry, circuit breaker: é `*http.Client` do app.
- `trilha generate route --from-openapi`: a segunda metade da issue, explicitamente depois.
