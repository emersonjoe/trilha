# Spec 180 — Props de ilha pelo tipo da expressão

**Feature Branch**: `180-props-de-ilha-por-tipo` | **Created**: 2026-10-18 | **Status**: Pronta para release (0.160.0)

- **Issue**: #303 — a fonte do escopo.
- **Branch**: `180-props-de-ilha-por-tipo`
- **Versão**: 0.160.0

## Por quê

O `trilha gen` só derivava a entrada de `public/islands.d.ts` quando a chamada era
`c.Island("/x.js", Props{...}, fb)`, com o literal no próprio argumento. Uma página que monta as
props numa variável e escolhe a ilha num `switch` (o Prosa, três ilhas com o mesmo struct de
onze campos) recebia `unknown` e uma nota, e o contorno era repetir o literal em cada chamada.

## Jornada e risco

Jornada de quem escreve a ilha: o editor confere as props contra o Go. Pior impacto: um tipo
errado no `islands.d.ts` (pior que `unknown`, porque engana). Coberto por unidade e golden; nada
muda no navegador.

## O que muda

O tipo das props vem da expressão: literal, `&x`, parênteses, variável (`x := …`, `var x T`,
`var x = …`, a última declaração antes do uso), parâmetro da função, conversão `T(x)` e chamada
a uma função do app (do mesmo diretório) que devolve o struct. O que continua sem nome — um
mapa, um campo de outro struct, um método — segue `unknown`, com a nota dizendo que formas são
lidas.

## Fora de escopo

`go/types`: a resolução fica no AST, como decidido na spec 176 (scanner com `go/parser`, sem
carregar pacotes); método e campo de struct ficam para quando houver um caso real.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `go/ast` |
| III — gerador determinístico | golden do `islands.d.ts` regravado; `TestDeterministic` |
| VI — teste primeiro | `TestPropsTypedByTheExpression` reprovava (seis ilhas `unknown`) |

## Segurança e privacidade

Nenhuma fronteira nova: o scanner lê o código do próprio app em build. Exceções: nenhuma.

## Tarefas

- [x] T001 `testdata/islands/app/jogo` com as seis formas e `TestPropsTypedByTheExpression`.
- [x] T002 `internal/islands`: `propsType` pela expressão, `localType`, `namedStruct`, índice
  do primeiro resultado das funções.
- [x] T003 Documentação en/pt (referência do ctx), CHANGELOG, ROADMAP, versão.
- [ ] T004 Release 0.160.0 (#303).

## Aceitação

- **SC-001** As seis formas de `testdata/islands/app/jogo` saem tipadas, sem nota.
- **SC-002** O mapa de `/chart.js` continua `unknown` com a sua nota.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | | | |
