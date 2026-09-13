# Plano — spec 146

**Branch**: `feat/client-tag-resposta-anulavel` | **Spec**: [spec.md](spec.md) | **Versão**: 0.125.0

## Resumo

Três correções no `internal/client` — o gerador de cliente de API que já existe — e no arquivo
que ele escreve: o corte da tag no nome do método passa a valer só como prefixo exato, o cliente
gerado ganha `WithResponse` para entregar a resposta HTTP de cada chamada, e um schema anulável
vira ponteiro sem `validate:"required"`. Nenhuma convenção nova em `app/`, nenhum símbolo novo na
superfície pública do repositório; o que muda de forma visível é o **arquivo gerado**, que é
commitado, então o diff dos goldens e dos `examples/` é parte da revisão.

## Contexto técnico

**Pacote**: `internal/client` (`gen.go` nome e operações, `types.go` schemas e tags, `emit.go`
runtime escrito no arquivo gerado). **Teste**: `go test ./internal/client`, com golden em
`internal/client/api/client.go` (compilado e exercitado contra `httptest` pelos próprios testes
do pacote). **Documento sintético**: `testdata/openapi/acervo.json`. **Clientes gerados
commitados fora do pacote**: `examples/cookbook/api/client.go` e
`examples/local-login/internal/acervo/client.go`, com chamadas em `examples/cookbook/*.go` e
`examples/local-login/app/...`. **Sem dependência nova** (princípio II).

## Fatos que decidem o desenho

1. **O corte da tag é um laço de quatro combinações.** `methodName` (gen.go) percorre
   `{gName, singular(gName)}` × `{TrimSuffix, trimWordPrefix}`. A spec 131 (#169) consertou uma
   das quatro; as outras três são as da issue #205. Tirar o laço e deixar
   `trimWordPrefix(name, gName)` resolve as três de uma vez, e a guarda de fronteira de palavra
   que já existe continua sendo a única condição.
2. **`trimWordPrefix` já trata o nome que sobra vazio.** Tag `config` + `operationId` `config`
   dá resto `""`, cuja primeira runa não é maiúscula, então o nome fica inteiro. Nenhuma guarda
   nova.
3. **O corte muda o nome do arquivo gerado do próprio repositório.** `Documents().List` vira
   `Documents().ListDocuments`, `Get` vira `GetDocument`, `Certificates().Upload` vira
   `UploadCertificate`, `Users().Upsert` vira `UpsertUser`. Então o trabalho inclui: golden do
   pacote, os dois clientes de `examples/` regerados pela CLI, as chamadas nesses exemplos, e os
   trechos de documentação que citam os nomes.
4. **`do()` é o único lugar por onde toda resposta passa.** `call()` chama `do()`; a operação
   binária devolve o que `do()` devolveu. Um gancho dentro de `do()`, logo depois do
   `c.http.Do`, roda exatamente uma vez por resposta e alcança as três formas de operação e
   também o caminho do `*Error` — que é onde moram `WWW-Authenticate` e `Retry-After`.
5. **O gancho não pode oferecer o corpo.** Depois do `do()` o corpo é lido por quem chamou: pela
   decodificação do `call()`, pela leitura de `b` no caminho do erro ou pelo `c.Pipe` de quem
   recebeu o `*http.Response`. Então o contrato documentado é status e cabeçalhos, que é o que
   as três necessidades medidas na issue pedem (`Set-Cookie`, `ETag`/`Location`, `Link`).
6. **O `context` é o que faz o gancho servir num servidor.** A assinatura
   `func(context.Context, *http.Response)` é a mesma ideia do `WithHeader`: o cliente é um só e
   o estado é da requisição. É também o desenho do contorno que o app real escreveu com um
   `RoundTripper`, menos as 40 linhas.
7. **O validador do Trilha trata ponteiro não-nulo como "veio".** `bind.go` marca
   `sent: fv.Kind() == reflect.Pointer && !fv.IsNil()` e `ruleRequired` é
   `f.sent || !emptyValue(f.Value)`. Logo, num ponteiro, `required` quer dizer presente — e
   `"x": null` decodifica para ponteiro **nulo**, que falharia. Como o `go-playground` decide
   igual, um campo anulável não pode levar `required` em nenhuma das duas leituras: a tag sai.
8. **A regra do ponteiro é do tipo, não do schema.** `goType` já devolve `*T` para o `anyOf` do
   Pydantic, incluindo `*[]string`, e a spec 130 apoiou o parâmetro de query nisso. Para a lista
   do 3.1 e para `nullable: true`, o ponteiro entra no ponto de uso (`object` e o laço de query
   de `method`) e só onde o tipo não tem `nil` seu — fatia, mapa e `json.RawMessage` ficam. Isso
   também evita mexer no `multipart`, onde `scalar()` recusa ponteiro e não existe `null`.
9. **`acervo.json` não tem campo anulável obrigatório.** Os quatro anuláveis de `Document` são
   opcionais, então o caso da issue #209 não aparece no golden. Dois campos novos no documento
   sintético — um `["string","null"]` e um `nullable: true`, os dois em `required` — põem o caso
   no arquivo commitado, que é compilado pelos testes.

## Arquivos

| Arquivo | Mudança |
| --- | --- |
| `internal/client/gen.go` | `methodName` vira método do `builder`, corta só o prefixo exato e emite a linha do relatório; ponteiro do parâmetro de query anulável. |
| `internal/client/types.go` | `nullableType`, `pointerForNull`; ponteiro e `validate` sem `required` em `object`. |
| `internal/client/emit.go` | `Client.response`, `WithResponse`, o gancho dentro de `do`. |
| `internal/client/client_test.go` | Testes dos três contratos; `TestPrefixoDaTagSoCaiEmFronteiraDePalavra` ganha os casos de sufixo e singular. |
| `testdata/openapi/acervo.json` | Dois campos `required` anuláveis em `Document`. |
| `internal/client/api/client.go` | Golden regravado (`make golden`). |
| `examples/cookbook/api/client.go`, `examples/cookbook/apiclient.go`, … | Cliente regerado pela CLI e chamadas com o nome novo. |
| `examples/local-login/internal/acervo/client.go`, `examples/local-login/app/…` | Idem. |
| `site/internal/docs/content/{en,pt}/…` | § `trilha client` da referência da CLI (nome do método, `WithResponse`, anulável) e a receita da API que já existe, nas duas locales. |
| `CHANGELOG.md`, `cmd/trilha/main.go`, `ROADMAP.md` | 0.125.0. |

## Riscos

- **O nome mais longo é mudança visível para quem já gerou o cliente.** É o pedido da issue e o
  conserto é regenerar (`trilha client`) e renomear as chamadas, que o compilador aponta uma por
  uma. O `CHANGELOG` diz isso na seção `Changed`.
- **Ponteiro num campo que era valor** é a mesma história, e também é apontada pelo compilador —
  nunca silenciosa, que é o ponto da issue #209.

## Constitution Check

O quadro dos princípios tocados está na [spec](spec.md#constitution-check). Nenhuma violação, nada
para "Complexity Tracking": a mudança é de um pacote, sem convenção nova em `app/` e sem símbolo
novo na superfície versionada do repositório.
