# Implementation Plan: Contexto sob orçamento

**Branch**: `160-contexto-por-orcamento` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)
**Input**: Spec 160 do PLANO-TOKENS-70.md (M1 — menos leitura).

## Summary

`internal/tokbudget` (4 chars/token) vira o orçador compartilhado; `trilha ctx` ganha
`--pack/--budget/--strict` com ordem de corte documentada e rodapé de custo `est.`; o
`AGENTS.md` do scaffold ganha seções fixas e alvo medido de 2.500 tokens; o MCP do site ganha
`get_context`/`search_code`; cada receita ganha `llms.txt` com custo por página e badge
`est.`; capítulo bilíngue no Learn. Nenhuma superfície pública nova no módulo raiz
(`internal/` e saída de CLI ficam de fora de `api/current.txt`).

## Technical Context

**Language/Version**: Go 1.22+ | **Dependencies**: nenhuma nova (stdlib) | **Storage**: n/a
**Testing**: `go test` stdlib; goldens do ctx (`make golden` cobre `internal/ctx`)
**Target Platform**: CLI + site | **Constraints**: zero dependência; segredos nunca na saída

## Constitution Check

- [x] **I. Convenção sobre configuração** — nenhuma convenção nova em `app/`.
- [x] **II. Só biblioteca padrão em runtime** — `internal/tokbudget` é stdlib; nenhuma
  linha nova no `go.mod`.
- [x] **III. Geração explícita** — nada de `reflect` novo para descoberta; o pack sai do
  scanner e do registro de receitas que a CLI já usa.
- [x] **IV. Contrato de handler** — nenhum símbolo novo nos pacotes guardados;
  `api/current.txt` não muda.
- [x] **V. Dev < 2s / binário único** — nada toca o pipeline.
- [x] **VI. Teste primeiro** — contrato do estimador, corte na ordem, `--strict`, segredo,
  orçamento do AGENTS.md, ferramentas MCP, llms por receita, capítulo bilíngue.
- [x] **VII. Segurança por padrão** — máscara de literais em `Provide` +
  `TestCtxNeverLeaksSecrets`; evidência na spec.
- [x] **Estilo e idioma** — código em inglês; capítulo bilíngue no mesmo commit; spec/plan/
  tasks em pt-BR.

## Design (o que muda, onde)

```
internal/tokbudget/       # NOVO: CharsPerToken, Estimate, + testes de contrato
internal/ctx/pack.go      # NOVO: Pack, RecipeInfo, RouteLine; orçamento e ordem de corte
internal/ctx/markdown.go  # receitas + convenções + rodapé de custo est.
internal/ctx/ctx.go       # provided() mascara literais de string
cmd/trilha/ctx.go         # --pack/--budget/--strict, E_CTX_BUDGET
internal/scaffold/agents.go # AGENTS.md v2 (mapa, receitas, portões, leitura)
site/internal/docsmcp/    # get_context + search_code
site/internal/docs/llms.go# llms por receita com custo est.
site/app/referencia|reference # badge de custo est. na página da receita
site/internal/docs/content/{en,pt}/learn/ # contexto-sob-orcamento.md
```

Ordem dos blocos: T01 (tokbudget) → T02 (pack + comando) → T03 (segredo) → T04 (agents v2)
→ T05 (MCP) → T06 (llms + badge) → T07 (capítulo).

## Risk Management

- **Estimativa virar promessa**: toda saída que a usa carrega `est.`; o contrato da constante
  é testado e o capítulo explica que serve para orçar, não para faturar.
- **Vazamento por citação de código**: máscara de literais + teste negativo varrendo todas
  as formas de saída.
- **AGENTS.md crescer de novo**: o teste de orçamento falha o marcou; mudança de conteúdo
  exige re-medir.
