# Spec 047 — `trilha ctx` e `trilha check`: o mapa do projeto e o portão único

- **Issues**: [#47](https://github.com/emersonjoe/trilha/issues/47) e
  [#48](https://github.com/emersonjoe/trilha/issues/48) (ROADMAP, Fase 5, itens 25 e 26)
- **Branch**: `047-ctx-e-check`
- **Versão**: 0.37.0

## Por quê

As duas issues são o mesmo problema medido de dois lados: **um agente gasta rodada
descobrindo o que a máquina já sabe**. Uma rodada é a conversa inteira reenviada, então cada
pergunta que o framework não responde de primeira é dinheiro.

- Para saber *o que o projeto tem*, o agente abre trinta arquivos. O scanner já conhece rota,
  método, layout, middleware e parâmetro; o `internal/openapi` já lê do handler o que ele
  recebe e o que devolve. Falta imprimir isso (#47).
- Para saber *se o que ele escreveu está certo*, o agente roda cinco comandos e interpreta
  cinco saídas diferentes. E quando um deles reclama, a mensagem diz o que está errado mas
  não o conserto — mais duas rodadas (#48).

Elas entram na mesma spec porque são o mesmo comando visto pelas duas pontas: os dois leem
`internal/scan` e a inferência do `internal/openapi`, os dois têm `--json` determinístico com
golden, os dois viram ferramenta no MCP (#50), e o `AGENTS.md` da 0.36.0 só fica honesto
quando puder dizer *um* portão em vez de cinco. Uma medição da régua (#45) cobre as duas.

O nome importa: `ctx` é o contexto que o agente carregaria sozinho, e `check` é o único
comando que ele precisa rodar antes de dizer que terminou.

## O que muda

### `trilha ctx [--json] [--routes|--types|--all]`

Sem flag, um Markdown compacto (alvo: 1–2k tokens num projeto médio) com cinco seções:

1. **Projeto** — módulo, versão da CLI, se `trilha_gen.go` está atualizado, quantas rotas.
2. **Rotas** — em ordem de precedência (a mesma do `trilha routes`), com padrão, tipo,
   métodos, arquivo de origem e, quando houver, layouts e middlewares aplicados.
3. **API** — por rota de API e método: o que o handler recebe (`Bind`) e o que devolve
   (status + tipo), vindo da inferência que o `openapi` já faz.
4. **Tipos** — os tipos do projeto que aparecem nesse contrato, com campos, tipo de cada um,
   obrigatoriedade e as regras da tag `validate`.
5. **Setup** — `Setup`, `Config`, `Shutdown` e os valores registrados com `trilha.Provide`,
   quando o tipo é dedutível sem compilar.

`--routes` imprime só a seção 2, `--types` só a 4, `--all` imprime tudo sem elidir: cadeia de
middlewares por método, layouts de cada rota e todo tipo indexado, não só os alcançáveis.

`--json` devolve a mesma coisa estruturada e determinística: mesma entrada, mesmos bytes,
sem data, sem tempo, sem caminho absoluto.

### `trilha check [--json] [--fix]`

Roda, nesta ordem, parando no primeiro que falhar:

| Passo | O que roda | Com `--fix` |
|---|---|---|
| `gen` | `trilha gen --check` | regera `trilha_gen.go` e segue |
| `gofmt` | `gofmt -l` fora de `testdata` | `gofmt -w` e segue |
| `vet` | `go vet ./...` | — |
| `test` | `go test ./...` | — |
| `audit` | as checagens do `trilha audit` (só *critical* reprova) | — |
| `openapi` | `trilha openapi --check`, só quando `openapi.json` existe | — |

A saída humana é uma linha por passo no estilo do `audit` (`✓`/`✗`/`–` para não executado),
e os problemas do passo que falhou vêm indentados abaixo, cada um com `arquivo:linha`, a
mensagem e o conserto. `--json` devolve
`{"ok":…,"steps":[{"tool","status"}],"problems":[{"tool","file","line","message","fix"}]}`.

### O conserto na mensagem

Cada erro do scanner ganha dois campos: `Line` (quando há linha) e `Fix` — a frase que
resolve, no estilo do `hint` do `audit`. O `gen`, o `dev` e o `check` passam a imprimi-la.
Onde a mensagem hoje só diz o que falta, ela passa a dizer o que encontrou:

- `page.go` sem `func Page` → diz qual função exportada está lá no lugar e em que linha.
- `route.go` só com `func get` → "handlers are named by HTTP method in upper case", com a
  linha do `func get`.
- parâmetro repetido no mesmo caminho (`app/a/slug_/b/slug_`) → erro novo
  (`ErrDuplicateParam`), apontando as duas pastas; hoje isso passa pelo scanner e explode em
  tempo de execução dentro do `net/http`.

## Superfície

| Símbolo | Papel |
|---|---|
| `trilha ctx` | o mapa do projeto |
| `trilha check` | o portão único |
| `scan.Error.Line`, `scan.Error.Fix` | linha e conserto de cada violação |
| `scan.ErrDuplicateParam` | parâmetro repetido no mesmo caminho |
| `internal/ctx.Build(root, module) (Context, error)`, `Context.Markdown`, `Context.JSON` | o modelo |

Nada novo em `package trilha`.

## Fora de escopo

- **Inferência de tipos de verdade** (`go/types` com importador) para os valores de
  `trilha.Provide`. O tipo sai do argumento de tipo explícito ou do literal composto; o resto
  é registrado como expressão, com o tipo vazio. O `openapi` faz a mesma aposta desde a 031.
- **`ctx` sobre projeto que não compila.** O que o scanner recusa, o `ctx` recusa: quem quer
  saber o que está quebrado roda `check`.
- **`check --fix` consertando código.** `--fix` regera e formata; ele nunca edita `app/`.
- **Servir o `ctx` numa rota** ou no inspetor. O MCP (#50) é quem vai entregá-lo a máquinas.
- **Cache.** `ctx` relê o projeto a cada chamada; abaixo de 1 s não vale a invalidação.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — roteamento por arquivos é a fonte | as duas saídas vêm do scanner; nada é declarado duas vezes |
| II — só biblioteca padrão | `go/ast`, `go/parser`, `encoding/json`, `os/exec` |
| III — gerador determinístico | `ctx --json` e `check --json` têm golden; sem data nem caminho absoluto |
| IV — convenção nova pede scanner + exemplo + integração | `ErrDuplicateParam` tem teste de scanner e e2e da CLI |
| VI — teste primeiro | golden e teste de erro antes do comando |
| IX — API pública pequena | dois comandos; `package trilha` não muda |

## Aceitação

- **SC-001** `cd examples/blog && trilha ctx` imprime as cinco seções; `/api/posts` aparece
  com `GET` e `POST`, a rota `/blog/{slug}` com o parâmetro e o layout, e a saída inteira
  cabe em menos de 2k tokens (≈ 8 KB).
- **SC-002** `trilha ctx --json` roda duas vezes com bytes idênticos e bate com o golden
  `testdata/golden/ctx.json.golden`; `trilha ctx` bate com `ctx.md.golden`
  (`make golden` regrava os dois).
- **SC-003** `--routes` e `--types` imprimem só a sua seção; `--all` traz a cadeia de
  middlewares por método que a saída padrão elide.
- **SC-004** `trilha ctx` num projeto de 40 rotas responde em menos de 1 s (teste com árvore
  sintética gerada).
- **SC-005** Num projeto recém-criado (`trilha new`), `trilha check` termina verde e com
  código de saída 0.
- **SC-006** Cada erro da lista da #48 (`page.go` sem `Page`, método em minúsculas,
  parâmetro repetido) sai do `check` com `file:line` preenchido e `fix` não vazio, tanto no
  texto quanto no `--json`.
- **SC-007** `check` para no primeiro passo que falha: os passos seguintes saem como não
  executados, e o código de saída é 1.
- **SC-008** `check --fix` num projeto com `trilha_gen.go` velho e arquivo desformatado
  regera, formata e termina verde; sem `--fix`, o mesmo projeto reprova.
- **SC-009** `check --json` tem golden estável (`testdata/golden/check.json.golden`) num
  projeto que reprova no `gen`, sem tempo nem caminho absoluto na saída.
- **SC-010** O `AGENTS.md` do scaffold recomenda `trilha check` como o único portão, nos dois
  idiomas, e `TestAgentsMatchesUsage` continua verde.
