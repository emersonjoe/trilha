# Spec 122 — O retorno do Verba: o cliente de um FastAPI e o `<svg>` que não desenha

- **Issues**: [#156](https://github.com/emersonjoe/trilha/issues/156),
  [#157](https://github.com/emersonjoe/trilha/issues/157),
  [#158](https://github.com/emersonjoe/trilha/issues/158),
  [#159](https://github.com/emersonjoe/trilha/issues/159),
  [#160](https://github.com/emersonjoe/trilha/issues/160) — as issues são a fonte do escopo.
- **Branch**: `claude/eloquent-gates-qm3ojs`
- **Versão**: 0.101.0

## Por quê

As cinco issues são de uma migração real (o Verba, `emersonjoe/verba`) e não de uma leitura do
código: alguém apontou o `trilha client` para um `openapi.json` de FastAPI com 110 operações e
apontou o `trilha migrate next` para 20 rotas de Next.js. O que voltou:

1. O comando **falha** — `illegal UTF-8 encoding` — quando uma descrição em português tem um
   acento no byte errado (#160). O corte do comentário é por byte, e `d[:107]` parte um `ç` ao
   meio. Nenhum arquivo é escrito. Hoje o contorno é reescrever o docstring da API.
2. O cliente gerado **compila e não tipa nada**: das 110 operações, nenhuma declara
   `response_model`, então 230 `json.RawMessage` saem sem uma palavra de aviso (#157). Quem
   migra descobre ao abrir o arquivo, não ao rodar o comando.
3. Um formulário SSR que quer marcar "e-mail inválido" ao lado do campo tem de reparsear
   `Error.Body` à mão em toda página, porque o 422 da FastAPI (`detail: [{loc, msg}]`) vira
   **uma** frase em `Error.Detail` (#158). O Trilha já tem o tipo certo: `FieldErrors`.
4. Quatro das 20 rotas vieram **C — drawing** por causa do mesmo `<svg>` de 22 px do logo
   (#156). Nenhuma delas é interativa. A classe C é a mais conservadora, então o relatório
   empurra quem migra a escrever ilha onde um formulário basta.
5. Quem chega pelo `ROADMAP.md` lê "Onde o Trilha está … v0.41.0" com o framework na 0.100.0
   (#159) e planeja a migração contra uma fronteira que ficou sessenta versões para trás.

## O que muda

**`internal/client` — cortar por rune, não por byte (#160).** `clip(s, max)` recua até o começo
de um caractere antes de concatenar `"..."`; `doc()` e `firstSentence()` passam por ela. Uma
descrição com acento na posição do corte gera um comentário válido em vez de um arquivo que não
compila.

**`internal/client` — dizer o que não foi tipado (#157).** `Result` ganha `Ops int` e
`Untyped []string` (as operações cujo retorno é `json.RawMessage`, como `"GET /api/folhas"`). O
comando imprime a conta:

```
$ trilha client openapi.json
  ...
  97/110 operations have no response schema — returned as json.RawMessage.
  In FastAPI, declare response_model=... so the client can type the answer.
  internal/api/client.go written, package api.
```

`--verbose` imprime uma linha por operação; `--fail-on-untyped` devolve erro, para a CI de quem
está migrando e quer que a conta chegue a zero. A parte binária desta issue (`*http.Response`
para `application/pdf`, `text/csv`, `application/octet-stream`, `image/*`) já entrou na 0.99.0 e
a própria issue registra isso; aqui só fica o aviso.

**`internal/client` — o 422 da FastAPI por campo (#158).** O `Error` do cliente gerado ganha
`Fields map[string]string`, preenchido quando `detail` for uma lista de objetos com `loc` e
`msg`: o `loc` sem o prefixo `body`/`query`/`path`/`header`/`cookie` vira a chave
(`email`, `itens.0.valor`) e o `msg` o texto. `Detail` continua sendo a primeira mensagem — e
agora é a frase (`msg`), não o objeto inteiro serializado. `AsError(err)` é o `errors.As` sem
a variável intermediária, e `map[string]string` converte para `trilha.FieldErrors` sem cópia:

```go
if _, err := c.Usuarios().Criar(ctx, body); err != nil {
    if e, ok := api.AsError(err); ok && e.Status == 422 {
        return c.Render(page(form, trilha.FieldErrors(e.Fields)))
    }
    return err
}
```

O cliente gerado continua importando só a biblioteca padrão: `FieldErrors` é um
`map[string]string` do lado de lá, e a conversão é de quem chama.

**`internal/migrate` — um `<svg>` é um logo até que algo desenhe nele (#156).** O sinal
`drawing` passa a ser `<canvas>`/`getContext(` apenas. Um `<svg>` só puxa para C junto com o
que o usa: `ref=` no próprio elemento, `onWheel`, `requestAnimationFrame(`, ou um import de
biblioteca de desenho (`konva`, `pixi`, `fabric`, `three`) — e aí o sinal se chama `live svg`.
Handler de ponteiro e `d3`/`recharts` continuam classificando sozinhos, pelos sinais `pointer`
e `chart`, então nenhuma tela de verdade interativa muda de classe. O motivo passa a carregar a
**linha**: `C — live svg (fluxos/FlowCanvas.tsx:12)`, `C — pointer (linha 12)` quando o sinal é
da própria página, para o falso positivo ser descartável à mão sem abrir o arquivo.

**`ROADMAP.md` e `scripts/release.sh` (#159).** A seção "Onde o Trilha está" volta para a versão
corrente, e o `release.sh` passa a **recusar** uma release cujo `ROADMAP.md` não nomeie a versão
que está saindo, do mesmo jeito que já recusa um `CHANGELOG.md` sem a seção. Envelhecer de novo
deixa de ser possível sem alguém ignorar um erro na cara.

## Fora de escopo

- Gerar a linha do `ROADMAP.md` a partir do `CHANGELOG.md` por escrita automática: o script não
  commita (exige árvore limpa), então escrever o arquivo no meio do ritual criaria uma mudança
  não commitada dentro dele. Conferir é o mesmo ganho sem o efeito colateral.
- Um `trilha.ValidationError` no runtime para `errors.As` atravessar o cliente: acoplaria o
  arquivo gerado ao framework, que é exatamente o que a doc promete que ele não faz.
- Uma issue nova para o defeito achado no caminho: o cliente de um documento **sem** operação
  multipart não compilava — a parte do runtime que monta o formulário era emitida sempre e o
  `mime/multipart` só era importado quando alguma operação usava, então `multipart` saía
  indefinido no cliente de toda API que não recebe arquivo, que é a maioria. O teste do
  relatório de #157 foi quem gerou o primeiro documento assim. Está corrigido aqui, no mesmo
  arquivo, e registrado no `CHANGELOG.md`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `unicode/utf8`, `encoding/json`, `regexp`; o cliente gerado continua sem importar o Trilha (`TestNoExternalDeps` e a doc do `emit`). |
| III — coerência com Go | `AsError` é `errors.As` embrulhado; `Fields` é um `map[string]string` que converte para `trilha.FieldErrors`. |
| VI — teste primeiro | Um teste por issue, cada um falhando antes: o documento com acento no byte 107, a conta das operações sem schema, o 422 da FastAPI, o `<svg>` decorativo. |
| VII — segurança por padrão | Nada muda em rede, credencial ou resposta: `--fail-on-untyped` é código de saída, o aviso é texto no terminal. |

## Tarefas

- [x] T001 Testes que falham: `TestDocComAcentoNoCorte`, `TestRelatorioSemSchema`,
      `TestErro422PorCampo` (`internal/client`), `TestSvgDecorativoNaoEhDesenho`
      (`internal/migrate`).
- [x] T002 `clip()` e os dois pontos de corte; `Result.Ops`/`Result.Untyped`; `Error.Fields`,
      `fieldsOf`, `AsError` e o `Detail` que volta a ser frase.
- [x] T003 `--verbose` e `--fail-on-untyped` no `cmd/trilha/client.go`, com as mensagens nas
      duas locales.
- [x] T004 `internal/migrate`: `drawing` sem `<svg>`, sinal `live svg`, linha no motivo, texto
      da regra no relatório (en + pt).
- [x] T005 Golden do cliente regravado (`go test ./internal/client -update`), o
      `examples/cookbook/api/client.go` junto — a receita passou a usar o `AsError`, e o teste
      do site exige que todo bloco Go do livro de receitas exista num `.go` que compila — e
      `api/current.txt` (inalterado: nada da superfície pública mudou).
- [x] T006 Documentação nas duas locales (`reference/cli.md`, `cookbook/existing-api.md` e as
      traduções), `CHANGELOG.md`, `version`, `ROADMAP.md`, `scripts/release.sh`.
- [x] T007 `make test` verde e a release.

## Aceitação

- **SC-001** Um `openapi.json` cuja `description` tenha um caractere multibyte cruzando o byte
  107 gera o cliente; o arquivo é UTF-8 válido e compila (#160).
- **SC-002** `trilha client` de um documento sem `response_model` imprime `N/M operations have
  no response schema`; `--verbose` lista as operações; `--fail-on-untyped` sai com erro (#157).
- **SC-003** Um 422 com `detail: [{loc: ["body","email"], msg: "..."}]` chega como
  `Error.Fields["email"]`, e `Error.Detail` é a frase (#158).
- **SC-004** Uma página cujo único sinal é um `<svg aria-hidden>` volta `A`; `<canvas>`,
  `<svg ref={…}>` e handler de ponteiro continuam `C`, com a linha no motivo (#156).
- **SC-005** `ROADMAP.md` nomeia a 0.101.0, e `scripts/release.sh X.Y.Z` recusa uma versão que o
  `ROADMAP.md` não nomeie (#159).
