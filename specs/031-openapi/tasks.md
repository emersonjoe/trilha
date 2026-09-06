# Tasks: OpenAPI a partir das rotas

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — índice de tipos e schema (SC-002)

- [x] T001 Teste que falha em `internal/openapi/schema_test.go`: struct com tags `json` e
      `validate` vira `properties` + `required` + `maxLength`/`enum`/`format`; `time.Time` vira
      `string/date-time`; ponteiro sai do `required`; struct nomeada vira `$ref` e entra em
      `components`; `json:"-"` some.
- [x] T002 `internal/openapi/schema.go`: varredura do projeto, índice `pacote.Tipo` e
      `pacote.Func`, conversão para JSON Schema.

## Bloco 2 — inferência do handler (SC-001, SC-003, SC-005)

- [x] T003 Teste que falha em `internal/openapi/infer_test.go`: `c.Bind(&in)` dá corpo e 422;
      `v, ok := pkg.Get(); c.JSON(200, v)` dá 200 com o `$ref`; `WriteHeader(204)` dá 204 sem
      corpo; `trilha.ErrNotFound` dá 404 em `problem+json`; `c.Header("Content-Type", …)` troca
      o media type; as quatro diretivas `openapi:`; tipo inexistente na diretiva é erro.
- [x] T004 `internal/openapi/infer.go` e `internal/openapi/openapi.go`: `Generate`, `Options`,
      documento, `Problem`, tag padrão, `operationId`, parâmetros.

## Bloco 3 — comando (SC-004)

- [x] T005 Teste que falha em `cmd/trilha/e2e_test.go`: `trilha openapi -o -` no projeto
      recém-criado sai com JSON válido; `--check` passa com o arquivo fresco e falha depois de
      acrescentar uma rota.
- [x] T006 `cmd/trilha/openapi.go` + `main.go` (despacho) + `i18n.go` (mensagens nas duas
      línguas, linha no `usage`).

## Bloco 4 — exemplos (SC-001, SC-003)

- [x] T007 App sintético `testdata/apps/openapi` com golden `testdata/golden/openapi.json.golden`
      (`make golden` regrava), cobrindo cada regra de inferência.
- [x] T008 Diretivas nos exemplos: `examples/blog/app/api/posts/**` (`openapi:response`,
      `openapi:tag`) e `examples/orcamento/app/api/relatorio.csv/route.go` (`openapi:query`,
      `openapi:tag`), com asserções sobre os dois em `internal/openapi/openapi_test.go` —
      golden de exemplo mudaria a cada mexida no exemplo.

## Bloco 5 — documentação e fechamento

- [x] T009 `learn/api` + `pt/aprender/api`: seção "Documento OpenAPI" (o que é deduzido, quando
      escrever diretiva).
- [x] T010 `reference/cli` nas duas locales: o comando, as flags e a tabela de diretivas.
- [x] T011 `CHANGELOG.md` (0.22.0), `version` em `cmd/trilha/main.go`, ROADMAP (Fase 3, item 13).
- [x] T012 `make test` verde e `make release VERSION=0.22.0 ISSUES="31"`.
