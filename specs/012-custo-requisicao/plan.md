# Implementation Plan: 012-custo-requisicao

**Branch**: `012-custo-requisicao` | **Spec**: spec.md

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I Convenção | nenhuma convenção de arquivo muda |
| II Só stdlib | `math/rand/v2`, `sync`, `slog` — tudo stdlib |
| III Geração explícita | gerador intocado |
| IV Contrato de handler | `Ctx` mantém a mesma API pública |
| V Dev/prod | sem impacto no ciclo de recarga |
| VI Teste primeiro | cada mudança entra com o teste que a fixa; benchmark antes e depois |
| VII Segurança | nenhum cabeçalho sai; nonce continua imprevisível onde é usado; id de requisição deixa de usar `crypto/rand` e isso vai documentado (não é token) |

## Ordem (da mais segura para a mais arriscada, medindo entre cada passo)

1. **Profile antes**: `cd bench && go test -run XXX -bench JSON -cpuprofile cpu.out -memprofile mem.out` e `go tool pprof -top` para confirmar a tabela da spec com dados, não com leitura.
2. **CSP precompilada** (`security.go` + `applyConfig` em `trilha.go`): campos internos
   `cspNoNonce string` e `cspPre, cspPos string`; `csp(nonce)` vira concatenação.
3. **Nonce sob demanda** (`security.go`): `applySecurity` usa `cspNoNonce` quando
   `c.kind == kindAPI`; `Ctx.Nonce()` continua sorteando na primeira chamada.
4. **Id de requisição** (`ctx.go`): `math/rand/v2` (`rand.Uint64`), hex sem alocação extra.
5. **`values` preguiçoso** (`ctx.go`).
6. **Log guardado** (`serve.go`).
7. **`sync.Pool`** — só se 2–6 não bastarem para a meta; exige revisar que `Ctx` não escapa
   (o `Stream` e o `panicError` guardam referências; testar com `-race`).
8. Regravar `bench/RESULTS.md`, atualizar a tabela da página de desempenho, CHANGELOG, tag.

## Riscos

- `sync.Pool` com `Ctx` que vaza para uma goroutine do handler corromperia respostas: por
  isso é o último passo e opcional.
- Mudar o id de requisição pode surpreender quem o usa como token: o doc comment de
  `RequestID` passa a dizer explicitamente que é identificador de log, não segredo.
