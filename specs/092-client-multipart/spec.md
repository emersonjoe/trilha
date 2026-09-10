# Spec 092 — trilha client: multipart é um contrato, não um arquivo

- **Issue**: [#141](https://github.com/emersonjoe/trilha/issues/141) — a issue é a fonte do escopo.
- **Branch**: `092-client-multipart`
- **Versão**: 0.73.0

## Por quê

O gerador reconhece `multipart/form-data` e reduz a operação a **um** campo binário: a assinatura
recebe `file io.Reader, filename string` e o resto do formulário desaparece.

O documento sintético deste repositório já mostrava o buraco antes do Verba: a operação de upload
declara `upload` **e** `folder`, e o `folder` nunca chegou na assinatura. Um cliente gerado que
some com um campo é pior do que um cliente que não existe — quem chama não vê o que faltou até o
servidor recusar.

No Verba são três contratos reais: `files: list[UploadFile]` com até cinquenta XMLs no mesmo
campo, `file: UploadFile` mais `senha: Form(...)`, e formulários com mais de um campo de arquivo.
Portar qualquer um deles obriga a manter um cliente na mão ao lado do gerado, exatamente no
caminho em que o `trilha client` existe para não repetir.

## O que muda

O multipart passa a ser lido como o que é: uma lista de partes.

**Uma operação com um único campo binário e mais nada continua como está** — `file io.Reader,
filename string`. É a chamada mais comum e não há motivo para pedir um struct por ela.

**Qualquer outra forma vira um struct de entrada**, `<Grupo><Método>Form`:

```go
type CertificatesUploadForm struct {
	File  FilePart // the certificate
	Senha string   // required
}

type DocumentsBatchForm struct {
	Files  []FilePart // repeated under the same name, in order
	Folder string     // optional: goes only when it is not empty
}

// FilePart is one file of a multipart body.
type FilePart struct {
	Filename string
	Content  io.Reader
}
```

As partes são escritas na ordem em que os campos aparecem no struct — que é a ordem dos nomes,
porque um documento JSON não tem ordem própria a preservar e um gerador que responde diferente em
duas execuções não pode ser conferido. Uma lista vira várias partes com o mesmo nome, **na ordem
do slice** — que é o que um `list[UploadFile]` do outro lado
espera. Campo escalar obrigatório vai sempre; opcional vai só quando não está vazio, que é a
mesma regra que a query já usa.

**Continua em streaming.** O corpo é um `io.Pipe` e nenhum arquivo é lido inteiro em memória; o
erro do writer chega pelo `CloseWithError`, e cancelar o contexto fecha o cano — a goroutine que
escreve termina em vez de vazar.

## Fora de escopo

- **`multipart/mixed` e partes com `Content-Type` próprio.** O que o FastAPI escreve é
  `form-data`, e um `Content-Type` por parte pede uma API maior do que os contratos que existem.
- **Campo de formulário que não é escalar.** Um objeto aninhado dentro de um multipart vira nota
  no relatório, como já acontece com um corpo que o gerador não traduz.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `mime/multipart` e `io.Pipe` |
| VI — teste primeiro | o `httptest` que lê as partes falha antes da mudança |
| Determinismo | as partes saem na ordem dos nomes dos campos, e o golden prova |

## Tarefas

- [x] T001 Teste que falha: `httptest` conferindo nomes, filenames, escalares e repetição
- [x] T002 O multipart lido como contrato no `gen.go`, com nota para o que não é escalar
- [x] T003 `FilePart`, o struct de entrada e o writer de partes no runtime gerado
- [x] T004 Documento sintético com os três contratos, e o golden regravado
- [x] T005 Documentação (en + pt) e superfície de API
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.73.0`

## Aceitação

- **SC-001** Arquivo + string geram assinatura tipada para os dois, e o servidor recebe os dois.
- **SC-002** `array` de `string/binary` vira várias partes com o mesmo nome, na ordem do slice.
- **SC-003** Dois campos binários distintos são representáveis na mesma operação.
- **SC-004** O upload continua em streaming: um arquivo maior que a memória atravessa.
- **SC-005** Uma operação com um único campo binário mantém a assinatura de antes.
