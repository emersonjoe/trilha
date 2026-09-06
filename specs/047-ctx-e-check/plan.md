# Plano — Spec 047

Entrada: [spec.md](./spec.md). Duas issues, três pacotes tocados (`internal/scan`,
`internal/ctx` novo, `cmd/trilha`), então plano separado.

## Os fatos que decidem o desenho

1. **O `openapi` já é a inferência.** `openapi.Generate` lê do handler o que ele faz `Bind`,
   o status que escreve e o tipo que devolve, e monta os schemas dos tipos do projeto. O
   `ctx` **não** reimplementa nada disso: ele chama `Generate` e lê o documento de volta com
   `encoding/json`. Custa uma serialização e paga com a garantia de que `ctx` e `openapi`
   nunca discordam — e mantém a superfície do `openapi` do tamanho de hoje (princípio IX).
2. **Ordem de precedência é a do `trilha routes`.** O `routesTable` já ordena; o `ctx` usa a
   mesma função de ordenação, não uma segunda opinião.
3. **Determinismo é ausência de tempo.** Nada de data, duração, caminho absoluto ou ordem de
   mapa na saída: os caminhos são relativos à raiz e com `/`, as listas são ordenadas, e a
   versão é a constante da CLI. É o que torna o golden possível.
4. **O `check` não reimplementa os passos.** Ele chama o que já existe: `checkGen` em
   processo, `runAudit` em processo, `gofmt`/`go vet`/`go test` por `os/exec`. O que ele
   acrescenta é a ordem, a parada no primeiro erro e a tradução da saída de cada um para a
   mesma lista de `{tool, file, line, message, fix}`.
5. **O conserto mora no scanner, não no `check`.** `scan.Error` ganha `Line` e `Fix`; quem
   imprime (gen, dev, check) só formata. Assim a mesma frase aparece nos três, e um código de
   erro novo nasce com conserto ou não nasce.
6. **`go test` fala demais.** Da saída do teste sobram as linhas `--- FAIL:` e as
   `arquivo.go:linha:` do `t.Fatal`/`t.Error`; o resto (tempo, `ok`, `PASS`) é descartado —
   é exatamente o que a #48 pede e o que mantém a saída barata.

## Fases

1. **Erros com conserto** (`internal/scan`): `Line`, `Fix`, tabela de consertos por código,
   mensagens que dizem o que encontraram, `ErrDuplicateParam`. Teste de scanner primeiro.
2. **O modelo** (`internal/ctx`): `Build` monta o `Context` a partir do scanner + documento
   do `openapi`; `Markdown` e `JSON` imprimem. Golden dos dois.
3. **Os comandos** (`cmd/trilha`): `ctx` com as flags, `check` com a ordem, `--fix` e
   `--json`; renderização compartilhada dos erros do scanner com o conserto.
4. **Fechamento**: `AGENTS.md` do scaffold recomendando `trilha check`, documentação nos dois
   idiomas, `usage`, CHANGELOG, ROADMAP, régua.

## Riscos

- **Custo do `openapi` num projeto grande.** Ele parseia o projeto inteiro. SC-004 mede com
  40 rotas sintéticas; se estourar 1 s, o `ctx` sem `--all` deixa de pedir a inferência e a
  seção API vira opcional (`--api`). Medir antes de otimizar.
- **Golden do `check` com `go test` dentro.** A saída do `go test` tem tempo; por isso o
  golden é sobre um projeto que reprova no `gen` e nunca chega ao `test`. O caminho completo
  é coberto pelo e2e, sem golden.
- **`--fix` e o `dev` rodando.** `--fix` reescreve `trilha_gen.go`; com o `dev` no ar isso é
  só mais uma recarga. Nada a fazer, mas fica dito.
