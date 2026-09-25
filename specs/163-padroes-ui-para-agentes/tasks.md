# Tasks: Padrões de UI para agentes

- [x] **T01** — `ui.Pattern` + `ui.Patterns()` com os seis padrões + snippets em
  `examples/patterns/` (arquivo gerado `ui/patterns_snippets.go`). *Aceite:*
  `TestPatternsCompile` + `TestPatternSnippetBudget` (+ `TestPatternSnippetsCurrent`,
  `TestPatternsNameRealComponents`, `TestPatternsKeepProtections`).
- [x] **T02** — `trilha ui patterns --json` + golden estável; `trilha ui patterns <nome>`.
  *Aceite:* `TestPatternsJSONStable`.
- [x] **T03** — MCP `get_pattern` (`mcp.ContextTools` e o MCP do site) + registro na doc
  bilíngue das ferramentas. *Aceite:* teste no `ai/mcp` + teste do `docsmcp`.
- [x] **T04** — Site: seção "usar este padrão" nas seis demos, nas duas línguas. *Aceite:*
  `TestPatternsPageBilingual`.
- [x] **T05** — Superfície: `make api` com `ui.Pattern`/`ui.Patterns()` registradas.
  *Aceite:* `make test` verde com a trava da superfície atualizada.
- [x] **T06** — Golden `uidoc` por padrão + `patterns` no catálogo e no `ui describe`.
  *Aceite:* `TestUIDocPatternGoldens`.
- [ ] **T07** — CHANGELOG + ROADMAP + suíte. *Aceite:* `make test` verde.
