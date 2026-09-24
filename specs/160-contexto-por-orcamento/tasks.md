# Tasks: Contexto sob orçamento

**Branch**: `160-contexto-por-orcamento` | **Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## Bloco 1 — o orçador e o pack

- [ ] **T01** — `internal/tokbudget` com `CharsPerToken = 4`, `Estimate(s) int` e testes de
  contrato (vazio, unicode, arredondamento para cima). *Aceite:*
  `go test ./internal/tokbudget/...`.

- [ ] **T02** — `Pack` + `--pack/--budget/--strict/--json` no `internal/ctx` e no comando;
  ordem de corte (contratos → receitas → enums, rotas nunca) em doc-comment e testada;
  receitas instaladas detectadas pelos marcadores `trilha:add` no `setup.go` + arquivos
  existentes; `provided()` mascara literais de string; goldens do blog regenerados
  (`make golden` + commit). *Aceite:* `make golden` e
  `go test ./internal/ctx/... ./cmd/trilha/...`.

- [ ] **T03** — `TestCtxNeverLeaksSecrets`: fixture com segredo escrito dentro de um
  `Provide` do `setup.go`; varre Markdown, `--json`, pack humano e pack `--json`; o segredo
  não aparece; nomes e tipos aparecem. *Aceite:*
  `go test ./internal/ctx/ -run TestCtxNeverLeaksSecrets`.

## Bloco 2 — o AGENTS.md e as fatias

- [ ] **T04** — `trilha agents` v2: seções fixas (mapa → `ctx --json`, receitas instaladas +
  link de docs, portões `make test`/`trilha check`, regras de leitura estreita), alvo
  ≤ 2.500 tokens est.; `TestAgentsMdBudget` sobre o projeto gerado pelo `trilha new`.
  *Aceite:* `go test ./internal/scaffold/... -run TestAgentsMdBudget`.

- [ ] **T05** — MCP do site: `get_context(pack)` (páginas de doc da receita com custo `est.`)
  e `search_code(query)` (`path:linha±N`, nunca o arquivo inteiro); testes de ferramenta;
  página de referência MCP atualizada nas duas línguas. *Aceite:*
  `go test ./site/... -run MCP`.

- [ ] **T06** — `llms.txt` por receita com custo `est.` por página (`TestLLMsTxtPerRecipe`) +
  badge mono de custo na página de cada receita, nas duas línguas. *Aceite:*
  `go test ./site/... -run 'TestLLMs|Recipe'`.

- [ ] **T07** — Capítulo "Context under budget"/"Contexto sob orçamento" no Learn (en+pt)
  com exemplos de chamada e a ordem de corte, registrado nas duas `Locales`. *Aceite:*
  `go test ./site/... -run TestLocalesInSync`.

## Fim da spec

- [ ] **T08** — Entrada no topo do `CHANGELOG.md`, ROADMAP se a área muda de estado, sem
  `make release`, sem tocar `const version`. *Aceite:* revisão do diff final + `make test`
  verde.
