# Spec 065 — O cliente Go de uma API que já existe

Issue: [#61](https://github.com/emersonjoe/trilha/issues/61) (`trilha client`). A issue é a
fonte do escopo; aqui fica só a decisão.

## Por que sozinha

A #60 (upstream) e a #59 (migrate) já entraram: o browser fala com a API pelo prefixo e a
árvore do `app/` nasce pronta. O que falta é o outro lado da mesma migração — a página, que
roda no servidor, precisa dos **tipos** da API para montar a tela. Hoje isso é `map[string]any`
e um `fetch` que só falha em produção.

## Decisões

1. **Um arquivo, determinístico, commitado.** Mesma regra do `trilha_gen.go`: `trilha client`
   grava, `--check` falha se o documento mudou e ninguém regerou. Um arquivo só porque o
   cliente é lido como referência — `grep` num arquivo acha o campo, `grep` em vinte não.
2. **A leitura do documento é própria, não a do `internal/openapi`.** Aquele pacote *escreve*
   OpenAPI a partir das rotas; aqui se *lê* um documento de terceiro, cheio de coisas que a
   Trilha nunca emite (`allOf`, `$ref` recursivo, `nullable`, tag sem nome). Reaproveitar as
   structs de escrita amarraria as duas direções e a leitura ficaria refém do que a Trilha
   escreve.
3. **Nome de tipo vem do `$ref`, não da forma.** `#/components/schemas/Document` vira
   `Document`. Esquema inline em resposta ou corpo vira um tipo nomeado pela operação
   (`DocumentsListResponse`), porque struct anônima em assinatura não se escreve à mão depois.
4. **O que não se sabe traduzir vira `json.RawMessage` com o comentário do esquema, e uma
   linha no relatório.** `oneOf`, `anyOf` e esquema sem `type` continuam chegando ao app; o
   que não pode acontecer é o gerador inventar um tipo errado ou desistir do arquivo inteiro.
5. **`allOf` é achatado.** É a forma como quase toda API descreve herança e o resultado é uma
   struct — traduzir para embedding daria o mesmo JSON com mais surpresa.
6. **Um método por operação, agrupado por tag.** `api.Documents.List(ctx, params)`. Operação
   sem tag cai num grupo `Default`. O nome sai do `operationId` quando ele existe, e de
   método+caminho quando não, porque `operationId` é opcional e metade das APIs não põe.
7. **Parâmetros de caminho na assinatura; query e corpo numa struct.** Caminho é obrigatório e
   posicional por natureza; query com dez campos opcionais em assinatura é ilegível, e a struct
   ainda serve de `Bind` de formulário. Corpo JSON entra como o tipo do esquema.
8. **Resposta binária devolve `*http.Response`.** Quem baixa um PDF quer o `Content-Type` e o
   corpo em stream para passar ao `c.Pipe`; decodificar seria jogar fora os dois.
9. **Erro é tipado e traz o corpo.** Status fora de 2xx vira `*Error{Status, Body, Detail}`,
   com `Detail` preenchido a partir de `problem+json` (`detail`/`title`) ou do `{"detail": …}`
   da FastAPI. Sem isso o app reescreve o mesmo `if resp.StatusCode >= 400` em cada chamada.
10. **`required` e `format` viram tags `validate`.** O mesmo tipo serve de resposta da API e de
    `Bind` de formulário: é o ponto da issue e sai de graça, porque a spec 027 já lê as tags.
11. **Genéricos só onde a API os pede.** `Paged[T]` não é adivinhado: se o documento descreve
    `PagedDocument` e `PagedUser`, saem dois tipos. Inferir genérico a partir de nome é
    heurística que erra em silêncio.
12. **Zero dependência, e o cliente gerado também.** `net/http`, `encoding/json`, `net/url`,
    `io`, `mime/multipart`, `strconv`, `context`. O arquivo gerado não importa a Trilha: serve
    num cron, num teste, num binário que não é web.
13. **URL é lida, arquivo também.** `trilha client https://api/openapi.json` faz um GET; é o
    caminho real de quem migra. Só `http`/`https`, timeout curto, e o documento é dado —
    nenhuma parte dele vira caminho de arquivo ou comando.

14. **`format: date-time` continua `string`.** Traduzir para `time.Time` faria a decodificação
    da resposta inteira falhar por causa de um campo com formato diferente do que o documento
    prometeu — e é justamente o campo que mais varia entre APIs. Quem quer `time.Time` faz o
    parse onde sabe o formato.

## Critérios de aceitação

- SC-001 `trilha client doc.json --out internal/api --package api` grava um arquivo que compila.
- SC-002 Mesma entrada, mesmos bytes; o golden não muda entre execuções.
- SC-003 Uma struct por esquema de `components/schemas`, com tags `json` e `validate`.
- SC-004 `$ref` recursivo vira ponteiro e não estoura a pilha.
- SC-005 `allOf` achata os campos das partes; `oneOf`/`anyOf` viram `json.RawMessage`.
- SC-006 Um método por operação, agrupado por tag; sem tag vai para `Default`.
- SC-007 Parâmetro de caminho na assinatura, query e corpo em struct, `enum` como constantes.
- SC-008 `multipart/form-data` gera assinatura com `io.Reader` e nome de arquivo.
- SC-009 Resposta binária devolve `*http.Response` sem decodificar.
- SC-010 Status ≠ 2xx vira `*Error` com `Status`, `Body` e `Detail` da FastAPI ou do RFC 9457.
- SC-011 `New(base, WithHeader(...), WithClient(...))` injeta credencial por requisição.
- SC-012 `--check` falha quando o arquivo está desatualizado e sai zero quando está em dia.
- SC-013 O relatório lista o que virou `json.RawMessage` e a operação sem `operationId`.
- SC-014 Teste com `httptest.Server` exercita GET com query, POST JSON, multipart, binário e erro.
- SC-015 `make test` verde, docs nas duas línguas, CHANGELOG.
