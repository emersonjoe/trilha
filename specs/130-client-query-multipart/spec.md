# Spec 130 — client: query opcional e multipart pelo schema

- **Issues**: [#170](https://github.com/emersonjoe/trilha/issues/170) e
  [#171](https://github.com/emersonjoe/trilha/issues/171) — as issues são a fonte do escopo.
- **Branch**: `feat/mig-s03-client-query-multipart`
- **Versão**: 0.109.0

## Por quê

Os dois achados vêm da mesma origem: um documento de FastAPI escreve coisas que o documento
sintético deste repositório não escrevia, e o gerador passou por cima delas sem falhar em
lugar nenhum — o arquivo compila, o `go vet` passa, e o erro só aparece quando a requisição
sai errada contra o servidor de verdade.

**#170.** `competencia: str | None = None` vira `anyOf: [string, null]`, e o gerador já
traduz isso para `*string` no struct de query (spec 062). Quem monta a URL, porém, não
conhece ponteiro: cai no ramo genérico e escreve `fmt.Sprint(p.Competencia)`, que para um
ponteiro a tipo básico imprime o endereço (`0xc0000a1b40`) ou `<nil>`. Os dois são diferentes
de `""`, então a condição que deveria deixar o parâmetro de fora é sempre verdadeira: o filtro
sai em toda requisição, sempre com lixo. Um filtro por competência nunca funciona, e o cliente
que o Verba contorna hoje em `internal/api/filtros.go`.

**#171.** A spec 092 (#141) fez o corpo multipart vir do schema — uma parte por propriedade,
array de binário virando `[]FilePart`, campo de texto virando campo de texto. O que ficou de
fora é que a FastAPI **nunca** escreve esse schema inline: ela declara um componente
`Body_<operação>` e aponta para ele com `$ref`. O gerador lê `mt.Schema.Properties` direto,
que num `$ref` está vazio, conclui "nenhuma parte" e cai no atalho histórico: um arquivo só,
no campo `file`. Uma operação `files: list[UploadFile]` fica **inalcançável** pelo cliente
gerado (a API responde 422 `Field required: files`) e um `senha` ao lado do arquivo some da
assinatura. Nada disso aparece na compilação.

## O que muda

**Parâmetro de query que é ponteiro.** O ponteiro é como o documento diz "isto pode não ser
mandado": `nil` é o parâmetro ausente, e qualquer outra coisa é um valor que quem chama
escolheu — inclusive a string vazia, que é como se filtra pelo valor vazio. O valor
desreferenciado é que vai para a URL, com a mesma conversão dos campos não-ponteiro
(`strconv` para número e booleano, `string()` para enum):

```go
// hoje: if fmt.Sprint(p.Since) != "" { q.Set("since", fmt.Sprint(p.Since)) }
if p.Since != nil {
	q.Set("since", *p.Since)
}
if p.State != nil {
	q.Set("state", string(*p.State))
}
```

Uma lista anulável — `tags: list[str] | None`, que na FastAPI é tão comum quanto a string —
continua sendo uma lista: um parâmetro por elemento quando o ponteiro tem valor, nenhum quando
é nil. Sem isso ela sairia como `label=[a b]`, a sintaxe do Go dentro da URL, que é a mesma
falha da issue com outra cara.

Isso difere do trecho escrito na #170 (`p.X != nil && *p.X != ""`) num ponto, de propósito:
com ponteiro quem chama já tem como dizer "ausente", então um `*string` apontando para `""`
viaja. Um campo não-ponteiro continua com a regra de sempre — o valor zero fica fora da URL —,
porque lá não existe outra forma de dizer ausente.

**Corpo multipart.** O `$ref` do `requestBody` é seguido (e um `allOf` achatado) antes de ler
as propriedades, então o corpo vem do schema em qualquer uma das duas formas que um documento
usa para escrever a mesma coisa:

```go
// Body_import_… { files: [binary], comment: string } — por $ref, como a FastAPI escreve
_, err := c.Documents().Import(ctx, api.DocumentsImportForm{
	Files:   []api.FilePart{a, b, c}, // três partes sob o nome "files"
	Comment: "lote de setembro",
})

// um binário e nada mais, também por $ref: continua dois argumentos
err = c.Documents().Attach(ctx, "d1", f, "anexo.pdf")
```

## Fora de escopo

- **O struct morto do `Body_…`.** O componente do corpo continua virando um tipo no arquivo
  gerado, sem ninguém referenciá-lo. É ruído antigo, não regressão, e tirá-lo pede saber
  quais componentes só servem de corpo — decisão maior que estas duas correções.
- **`[]*T` na query.** Um array *de nulos* (`list[str | None]`) não aparece em documento de
  FastAPI; o elemento ponteiro fica como está. O contrário — ponteiro para array, `*[]string` —
  está coberto acima.
- **`application/x-www-form-urlencoded`.** Continua sendo uma linha do relatório.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada entra: `strconv` e `mime/multipart` já são o que o gerado usa. |
| III — gerador determinístico | As partes continuam saindo em ordem de nome de propriedade; o golden `internal/client/api/client.go` é regravado e commitado. |
| VI — teste primeiro | Os dois testes contra `httptest` entram antes da correção, e falham do jeito que as issues descrevem: `since=0xc000…` na URL, `Field required: files` no servidor. |

## Tarefas

- [x] T001 Documento sintético (`testdata/openapi/acervo.json`): query `anyOf [T, null]`
      (string, integer, boolean e `$ref` de enum) e dois corpos multipart por `$ref` — um com
      array de binário mais campo de texto, um com um binário só.
- [x] T002 Testes que falham: URL sem o parâmetro quando o ponteiro é nil e com o valor
      quando não é; multipart lido com `r.MultipartForm` — dois arquivos em `files` e o
      `comment` junto.
- [x] T003 Implementação: `queryCode` conhece ponteiro; `multipart` resolve o `$ref`.
- [x] T004 Golden regravado (`go test ./internal/client -update`).
- [x] T005 Referência nas duas locales (`en/reference/cli.md`, `pt/referencia/cli.md`).
- [x] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, `ROADMAP.md`.
- [x] T007 `make test` verde e `make release VERSION=0.109.0 ISSUES="170 171"`.

## Aceitação

- **SC-001** Um parâmetro de query `*T` com valor sai na URL como o valor (`since=2026-09`,
  `year=2026`, `state=ready`, `draft=false`, `label=a&label=b`); nil não sai. Nenhum
  `fmt.Sprint` de ponteiro sobra no arquivo gerado.
- **SC-002** Um corpo multipart declarado por `$ref` gera uma parte por propriedade: duas
  partes `files` e uma `comment` chegam ao servidor na mesma requisição.
- **SC-003** Um corpo por `$ref` com um binário e nada mais continua sendo
  `(file io.Reader, filename string)`.
- **SC-004** `make test` verde, incluindo a compilação do cliente gerado em módulo próprio.
